// Package ognl extracts values from arbitrary Go object graphs using a string
// path expression, in the spirit of OGNL. It traverses structs, maps, slices,
// arrays, pointers and interfaces via reflection, without the caller having to
// know the concrete types in advance, and can read unexported fields.
//
// Path syntax:
//
//   - An unescaped "." separates path segments. Empty, leading, trailing, and
//     repeated separators do not address empty map keys.
//   - At segment start, unescaped ASCII space, tab, line feed, and carriage
//     return are ignored. Once a segment starts, those bytes are literal.
//   - String-keyed maps use the decoded segment text exactly, even when it is
//     numeric. Int-keyed maps, slices, arrays, and structs accept decimal
//     non-negative indices, including leading zeroes, a leading "+", and the
//     compatibility spellings "-0", "-00", and so on for index zero.
//   - "#" expands the current value into a list: subsequent segments are then
//     applied to every element (similar to flat-map). "##" expands twice.
//     Expansion work and result count are bounded per Get/GetE call.
//   - "\\" is a general escape introducer. For example, "\\.", "\\#", and
//     "\\\\" address a literal dot, hash, and backslash. A final unmatched
//     backslash is invalid and is reported as ErrInvalidSelector.
//   - Unicode text is matched exactly without normalization or case folding.
//
// Result compatibility contract:
//
//   - C1: Type is traversal metadata, not necessarily the dynamic kind of
//     Value. Parse/empty paths start at Interface; resolved non-expanded
//     segments use the resolved kind; expansion uses the kind reported by the
//     expansion, with Interface for an empty expansion. Mapping an already
//     expanded Result does not recompute Type.
//   - C2: Effective is false for Invalid and empty expanded results. Otherwise
//     it tests whether the stored interface is nil, so an interface containing
//     a typed nil pointer, map or slice is effective.
//   - C3: the first expansion of an empty collection succeeds with an
//     ineffective Result. A further segment or expansion has no matches, so
//     Get stays ineffective and GetE returns ErrInvalidValue.
//   - C4: Values ignores expansion errors and returns a scalar as one element;
//     ValuesE returns nil and ErrUnableExpand for that scalar.
//
// Concurrency: Get/GetE and the methods on a Result do not mutate their inputs
// and are safe to call concurrently on the same object or Result, as long as
// the underlying object is not being mutated elsewhere.
//
// Note: traversal relies on reflection (and unsafe, to read unexported fields),
// so it is slower than hand-written field access.
package ognl

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

var ErrInvalidStructure = errors.New("the structure cannot continue")

var ErrSliceSubscript = errors.New("invalid slice subscript")

var ErrMapKeyMustString = errors.New("map key must be string")

var ErrMapKeyMustInt = errors.New("map key must be int")

var ErrIndexOutOfBounds = errors.New("index out of bounds")

var ErrStructIndexOutOfBounds = errors.New("struct index out of bounds")

var ErrParseInt = errors.New("parse int error")

var ErrUnableExpand = errors.New("unable to expand")

var ErrInvalidValue = errors.New("invalid value")

// ErrExpansionLimit reports that '#' expansion exceeded the fixed operation
// or result limit for one Get/GetE call.
var ErrExpansionLimit = errors.New("expansion limit exceeded")

// ErrInvalidSelector reports malformed selector syntax.
var ErrInvalidSelector = errors.New("invalid selector")

// Type is traversal metadata returned by Result.Type, not necessarily the
// dynamic Go kind of Result.Value. Its constants are declared in the same order
// as the reflect.Kind constants, so Type(v.Kind()) is a safe conversion that
// preserves the integer value (a test locks this invariant). Use the Result.Type
// accessor rather than relying on the underlying integer.
type Type int

const (
	Invalid Type = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Pointer
	Slice
	String
	Struct
	UnsafePointer
)

func (t Type) String() string {
	switch t {
	case Func:
		return "func"
	case Chan:
		return "chan"
	case Pointer:
		return "pointer"
	case Interface:
		return "interface"
	case Map:
		return "map"
	case Slice:
		return "slice"
	case Array:
		return "array"
	case Struct:
		return "struct"
	case String:
		return "string"
	case Int:
		return "int"
	case Int8:
		return "int8"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case Uint:
		return "uint"
	case Uint8:
		return "uint8"
	case Uint16:
		return "uint16"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
	case Uintptr:
		return "uintptr"
	case Bool:
		return "bool"
	case Float32:
		return "float32"
	case Float64:
		return "float64"
	case Complex64:
		return "complex64"
	case Complex128:
		return "complex128"
	case UnsafePointer:
		return "unsafePointer"
	default:
		return "invalid"
	}
}

// Result holds the value a path resolved to. Obtain one from Get/GetE/Parse,
// inspect it with Value/Values/Type/Effective, collect non-fatal errors with
// Diagnosis, and descend further with Get/GetE (chaining). The zero value is an
// invalid Result.
type Result struct {
	// typ is traversal metadata. When the path used '#', the Result is expanded:
	// raw is a []interface{} and typ reflects the element kind reported by
	// deployment (often Interface, e.g. for map[string]interface{}). See C1.
	typ Type

	// raw is the resolved value. When expanded (see deployment) it is a
	// []interface{} of the collected elements.
	raw interface{}

	// deployment reports whether this Result is an expanded list (via '#').
	deployment bool

	// diagnosis collects non-fatal errors encountered while traversing; it does
	// not affect the returned value.
	diagnosis []error

	// retainedResults is internal accounting for expanded list slots reachable
	// from raw during one resolve call. Public entry points always start a fresh
	// budget; the field lets a parent account for nested child expansions.
	retainedResults int
}

// Effective reports whether the Result carries a usable value (contract C2):
// its Type is not Invalid, and an expanded Result has at least one element, or
// an unexpanded Result's stored interface is non-nil. An interface containing a
// typed nil pointer, map or slice is therefore effective.
func (r Result) Effective() bool {
	if r.Type() == Invalid {
		return false
	}
	if r.deployment {
		if r.raw == nil {
			return false
		}
		return len(r.raw.([]interface{})) != 0
	}
	return r.raw != nil
}

// Type returns traversal metadata (contract C1), not necessarily the dynamic
// kind of Value. Parse and empty paths start at Interface; resolved
// non-expanded segments and expansions update the metadata as described in the
// package contract. Invalid means nothing was resolved.
func (r Result) Type() Type {
	return r.typ
}

// Value returns the resolved value. For an expanded Result (via '#') it returns
// a fresh shallow copy of the []interface{} on every call. Otherwise it returns
// the single resolved value as-is (possibly nil); a slice or map in that value
// therefore retains its original ownership and is not copied.
func (r Result) Value() interface{} {
	if r.deployment {
		if r.raw == nil {
			return nil
		}
		return cloneTail(r.raw.([]interface{}), 0)
	}
	return r.raw
}

// Values returns the value as a slice: an expanded Result's elements directly,
// an expandable single value (slice/array/map/struct) expanded, or otherwise a
// one-element slice holding the value. Each call returns a fresh shallow slice;
// element objects are not deep-copied. Per contract C4, expansion errors are
// ignored, so a scalar is returned as one element; use ValuesE to observe the
// error.
func (r Result) Values() []interface{} {

	if r.deployment {
		if r.raw == nil {
			return nil
		}
		return cloneTail(r.raw.([]interface{}), 0)
	}

	v, t, _ := deployment(reflect.TypeOf(r.raw), reflect.ValueOf(r.raw), 0)
	if t == Invalid {
		return []interface{}{r.raw}
	}
	return v
}

// ValuesE is the error-returning form of Values (contract C4): it returns a
// fresh shallow slice on every successful call. When a scalar cannot be
// expanded, it returns nil values and an error wrapping ErrUnableExpand.
func (r Result) ValuesE() ([]interface{}, error) {

	if r.deployment {
		if r.raw == nil {
			return nil, nil
		}
		return cloneTail(r.raw.([]interface{}), 0), nil
	}

	v, t, err := deployment(reflect.TypeOf(r.raw), reflect.ValueOf(r.raw), 0)
	if err != nil {
		return nil, wrapResolutionError(err, reflect.TypeOf(r.raw), resolutionLocation{})
	}
	if t == Invalid {
		return []interface{}{r.raw}, nil
	}
	return v, nil
}

// Diagnosis returns a fresh shallow slice of the non-fatal errors collected
// while traversing. Each error is wrapped with %w, so errors.Is works against
// the package's sentinel errors.
func (r Result) Diagnosis() []error {
	if r.diagnosis == nil {
		return nil
	}
	diagnosis := make([]error, len(r.diagnosis))
	copy(diagnosis, r.diagnosis)
	return diagnosis
}

// Get applies path to a Result. When the Result is an expanded list (created
// via '#'), path is applied to every element and the matching results are
// collected. It is safe to call Get repeatedly, and concurrently, on the same
// Result: each call allocates its own backing slice and never mutates r.
func (r Result) Get(path string) Result {
	state := newResolutionState(path)
	tokens, err := parseSelector(path)
	if err != nil {
		diagnosis := append([]error(nil), r.diagnosis...)
		diagnosis = append(diagnosis, wrapResolutionError(err, reflect.TypeOf(r.raw), state.selectorError(path)))
		return Result{typ: Invalid, diagnosis: diagnosis}
	}
	return r.get(tokens, path, &expansionBudget{}, state)
}

func (r Result) get(tokens []selectorToken, path string, budget *expansionBudget, state resolutionState) Result {
	if r.deployment {
		nr := r
		nr.retainedResults = 0
		src, _ := r.raw.([]interface{})
		out := make([]interface{}, 0, len(src))
		diag := append([]error(nil), r.diagnosis...)
		for _, item := range src {
			next := get(item, tokens, path, 0, budget, state)
			diag = append(diag, next.diagnosis...)
			if budget.err != nil {
				nr.diagnosis = diag
				return expansionLimitResult(nr, wrapResolutionError(budget.err, reflect.TypeOf(item), state.firstOperation(tokens)))
			}
			if next.typ != Invalid {
				if next.deployment {
					out = append(out, next.raw.([]interface{})...)
					nr.retainedResults += next.retainedResults
				} else {
					if err := budget.retainResults(1); err != nil {
						nr.diagnosis = diag
						return expansionLimitResult(nr, wrapResolutionError(err, reflect.TypeOf(item), state.firstOperation(tokens)))
					}
					out = append(out, next.raw)
					nr.retainedResults += next.retainedResults + 1
				}
			} else if err := budget.releaseResults(next.retainedResults); err != nil {
				nr.diagnosis = diag
				return expansionLimitResult(nr, wrapResolutionError(err, reflect.TypeOf(item), state.firstOperation(tokens)))
			}
		}
		// Preserve the nil vs empty-slice distinction of the original raw so
		// Value()/Values() stay observationally identical.
		if len(out) == 0 && src == nil {
			out = nil
		}
		nr.raw = out
		nr.diagnosis = diag
		return nr
	}
	return get(r.raw, tokens, path, 0, budget, state)
}

// GetE is the error-returning form of Get. On an expanded Result it returns an
// error when no element matched path or when expansion exceeds its per-call
// limit. Per contract C3, creating an empty expansion succeeds, while applying
// another path to it returns ErrInvalidValue. Like Get, it never mutates r and
// is safe to call concurrently.
func (r Result) GetE(path string) (Result, error) {
	state := newResolutionState(path)
	tokens, err := parseSelector(path)
	if err != nil {
		return Result{typ: Invalid, diagnosis: append([]error(nil), r.diagnosis...)}, wrapResolutionError(err, reflect.TypeOf(r.raw), state.selectorError(path))
	}
	return r.getE(tokens, path, &expansionBudget{}, state)
}

func (r Result) getE(tokens []selectorToken, path string, budget *expansionBudget, state resolutionState) (Result, error) {
	if r.deployment {
		nr := r
		nr.retainedResults = 0
		src, _ := r.raw.([]interface{})
		out := make([]interface{}, 0, len(src))
		diag := append([]error(nil), r.diagnosis...)
		var firstFailure error
		for _, item := range src {
			next, err := getE(item, tokens, path, 0, budget, state)
			if budget.err != nil || errors.Is(err, ErrExpansionLimit) {
				if err == nil {
					err = budget.err
				}
				return invalidExpansionResult(nr), wrapResolutionError(err, reflect.TypeOf(item), state.firstOperation(tokens))
			}
			if err != nil {
				if firstFailure == nil {
					firstFailure = err
				}
				diag = appendResultFailure(diag, err, next.diagnosis)
			} else {
				diag = append(diag, next.diagnosis...)
			}
			if next.typ != Invalid {
				if next.deployment {
					out = append(out, next.raw.([]interface{})...)
					nr.retainedResults += next.retainedResults
				} else {
					if err := budget.retainResults(1); err != nil {
						return invalidExpansionResult(nr), wrapResolutionError(err, reflect.TypeOf(item), state.firstOperation(tokens))
					}
					out = append(out, next.raw)
					nr.retainedResults += next.retainedResults + 1
				}
			} else if err := budget.releaseResults(next.retainedResults); err != nil {
				return invalidExpansionResult(nr), wrapResolutionError(err, reflect.TypeOf(item), state.firstOperation(tokens))
			}
		}
		if len(out) == 0 && src == nil {
			out = nil
		}
		nr.raw = out
		nr.diagnosis = diag
		if len(out) == 0 {
			if firstFailure != nil {
				return nr, firstFailure
			}
			return nr, wrapResolutionError(ErrInvalidValue, reflect.TypeOf(src), state.firstOperation(tokens))
		}
		return nr, nil
	}
	return getE(r.raw, tokens, path, 0, budget, state)
}

// Parse wraps a value in a Result without navigating into it, so paths can be
// applied later via Result.Get/GetE. Parse(v) is equivalent to Get(v, "").
func Parse(result interface{}) Result {
	return Get(result, "")
}

// GetE resolves path against value and returns the Result together with an
// error describing the first fatal failure (it is the error-returning form of
// Get). See the package doc for the path syntax.
func GetE(value interface{}, path string) (Result, error) {
	state := newResolutionState(path)
	tokens, err := parseSelector(path)
	if err != nil {
		return Result{typ: Invalid}, wrapResolutionError(err, reflect.TypeOf(value), state.selectorError(path))
	}
	return getE(value, tokens, path, 0, &expansionBudget{}, state)
}

func getE(value interface{}, tokens []selectorToken, path string, depth int, budget *expansionBudget, state resolutionState) (Result, error) {
	if depth > maxResolveDepth {
		return Result{typ: Invalid, raw: value}, wrapResolutionError(ErrInvalidStructure, reflect.TypeOf(value), state.firstOperation(tokens))
	}
	if value == nil && len(tokens) == 0 {
		return Result{typ: Interface, raw: value}, nil
	}
	if value == nil {
		return Result{typ: Invalid, raw: value}, wrapResolutionError(ErrInvalidValue, nil, state.firstOperation(tokens))
	}

	result := Result{typ: Interface, raw: value}
	tp := reflect.TypeOf(value)
	tv := reflect.ValueOf(value)

	if !tv.IsValid() {
		result.typ = Invalid
		return result, wrapResolutionError(ErrInvalidValue, tp, state.firstOperation(tokens))
	}

	for index, token := range tokens {
		if token.kind == selectorSeparatorToken {
			remaining := tokens[index+1:]
			location := state.firstOperation(remaining)
			if result.deployment {
				list := result.raw.([]interface{})
				out := makeResultList(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(list), location)
				}
				result.retainedResults = 0
				var firstFailure error
				for _, item := range list {
					next, err := getE(item, remaining, token.remainingPath, depth+1, budget, state)
					if budget.err != nil || errors.Is(err, ErrExpansionLimit) {
						if err == nil {
							err = budget.err
						}
						return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(item), location)
					}
					if err != nil {
						if firstFailure == nil {
							firstFailure = err
						}
						result.diagnosis = appendResultFailure(result.diagnosis, err, next.diagnosis)
					} else {
						result.diagnosis = append(result.diagnosis, next.diagnosis...)
					}
					if next.typ != Invalid {
						if err := budget.retainResults(1); err != nil {
							return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(item), location)
						}
						out = append(out, next.raw)
						result.retainedResults += next.retainedResults + 1
					} else if err := budget.releaseResults(next.retainedResults); err != nil {
						return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(item), location)
					}
				}
				result.raw = out
				if len(out) == 0 {
					if firstFailure != nil {
						return result, firstFailure
					}
					return result, wrapResolutionError(ErrInvalidValue, reflect.TypeOf(list), location)
				}
				return result, nil
			}

			if result.raw == nil {
				if onlySelectorSeparators(remaining) {
					result.typ = Interface
					continue
				}
				result.typ = Invalid
				return result, wrapResolutionError(ErrInvalidValue, nil, location)
			}
			tp, tv = reflect.TypeOf(result.raw), reflect.ValueOf(result.raw)
			if !tv.IsValid() {
				result.typ = Invalid
				return result, wrapResolutionError(ErrInvalidValue, tp, location)
			}
			result.typ = Interface
			continue
		}

		location := state.location(token)
		if token.kind == selectorExpansionToken {
			if err := budget.consumeOperations(1); err != nil {
				result.deployment = true
				return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(result.raw), location)
			}
			if result.deployment {
				list := result.raw.([]interface{})
				out := makeResultList(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(list), location)
				}
				result.retainedResults = 0
				var firstFailure error
				for _, item := range list {
					raw, typ, err := deploymentWithBudget(reflect.TypeOf(item), reflect.ValueOf(item), 0, budget)
					failure := err
					if failure == nil && typ == Invalid {
						failure = ErrInvalidValue
					}
					if failure != nil {
						contextual := wrapResolutionError(failure, reflect.TypeOf(item), location)
						if errors.Is(failure, ErrExpansionLimit) {
							return invalidExpansionResult(result), contextual
						}
						result.diagnosis = append(result.diagnosis, contextual)
						if firstFailure == nil {
							firstFailure = contextual
						}
					}
					if typ != Invalid {
						out = append(out, raw...)
						result.retainedResults += len(raw)
					} else if err := budget.releaseResults(len(raw)); err != nil {
						return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(item), location)
					}
				}
				result.raw = out
				if len(out) == 0 {
					if firstFailure != nil {
						return result, firstFailure
					}
					return result, wrapResolutionError(ErrInvalidValue, reflect.TypeOf(list), location)
				}
				continue
			}

			result.deployment = true
			src := result.raw
			raw, typ, err := deploymentWithBudget(reflect.TypeOf(src), reflect.ValueOf(src), 0, budget)
			result.raw, result.typ = raw, typ
			if errors.Is(err, ErrExpansionLimit) {
				return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(src), location)
			}
			result.retainedResults = len(raw)
			failure := err
			if failure == nil && typ == Invalid && src != nil {
				failure = ErrInvalidValue
			}
			if failure != nil {
				return result, wrapResolutionError(failure, reflect.TypeOf(src), location)
			}
			continue
		}

		segment := token.value
		if result.deployment {
			list := result.raw.([]interface{})
			out := makeResultList(list)
			if err := budget.releaseResults(result.retainedResults); err != nil {
				return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(list), location)
			}
			result.retainedResults = 0
			var firstFailure error
			for _, item := range list {
				resolved, typ, failureType, err := resolveSegment(reflect.TypeOf(item), reflect.ValueOf(item), segment)
				failure := err
				if failure == nil && typ == Invalid {
					failure = ErrInvalidValue
				}
				if failure != nil {
					contextual := wrapResolutionError(failure, failureType, location)
					result.diagnosis = append(result.diagnosis, contextual)
					if firstFailure == nil {
						firstFailure = contextual
					}
				}
				if typ != Invalid {
					if err := budget.retainResults(1); err != nil {
						return invalidExpansionResult(result), wrapResolutionError(err, reflect.TypeOf(item), location)
					}
					out = append(out, resolved)
					result.retainedResults++
				}
			}
			result.raw = out
			if len(out) == 0 {
				if firstFailure != nil {
					return result, firstFailure
				}
				return result, wrapResolutionError(ErrInvalidValue, reflect.TypeOf(list), location)
			}
			continue
		}

		if result.raw == nil {
			result.typ = Invalid
			return result, wrapResolutionError(ErrInvalidValue, nil, location)
		}
		nv, typ, failureType, err := resolveSegment(tp, tv, segment)
		result.raw, result.typ = nv, typ
		if err != nil {
			return result, wrapResolutionError(err, failureType, location)
		}
		if typ == Invalid {
			return result, wrapResolutionError(ErrInvalidValue, failureType, location)
		}
		tp, tv = reflect.TypeOf(nv), reflect.ValueOf(nv)
	}

	return result, nil
}

// Get resolves path against value and returns the Result. It does not return an
// error: an unresolved path yields a Result whose Effective reports false, and
// non-fatal problems are recorded in Result.Diagnosis. Expansion work and
// result count are bounded, so adversarial '#' paths cannot exhaust memory or
// CPU, while recursive reflection and expanded-list traversal are depth-bounded.
// See the package doc for the path syntax.
func Get(value interface{}, path string) Result {
	state := newResolutionState(path)
	tokens, err := parseSelector(path)
	if err != nil {
		return Result{typ: Invalid, diagnosis: []error{wrapResolutionError(err, reflect.TypeOf(value), state.selectorError(path))}}
	}
	return get(value, tokens, path, 0, &expansionBudget{}, state)
}

func get(value interface{}, tokens []selectorToken, path string, depth int, budget *expansionBudget, state resolutionState) Result {
	if depth > maxResolveDepth {
		return Result{typ: Invalid, raw: value, diagnosis: []error{wrapResolutionError(ErrInvalidStructure, reflect.TypeOf(value), state.firstOperation(tokens))}}
	}
	if value == nil && len(tokens) == 0 {
		return Result{typ: Interface, raw: value}
	}
	if value == nil {
		return Result{typ: Invalid, raw: value, diagnosis: []error{wrapResolutionError(ErrInvalidValue, nil, state.firstOperation(tokens))}}
	}

	result := Result{
		typ: Interface,
		raw: value,
	}
	tp := reflect.TypeOf(value)
	tv := reflect.ValueOf(value)

	if !tv.IsValid() {
		result.diagnosis = append(result.diagnosis, wrapResolutionError(ErrInvalidValue, tp, state.firstOperation(tokens)))
		return result
	}

	for index, token := range tokens {
		if token.kind == selectorSeparatorToken {
			remaining := tokens[index+1:]
			location := state.firstOperation(remaining)
			if result.deployment {
				list := result.raw.([]interface{})
				out := makeResultList(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(list), location))
				}
				result.retainedResults = 0
				for _, item := range list {
					next := get(item, remaining, token.remainingPath, depth+1, budget, state)
					result.diagnosis = append(result.diagnosis, next.diagnosis...)
					if budget.err != nil {
						return expansionLimitResult(result, wrapResolutionError(budget.err, reflect.TypeOf(item), location))
					}
					if next.typ != Invalid {
						if err := budget.retainResults(1); err != nil {
							return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(item), location))
						}
						out = append(out, next.raw)
						result.retainedResults += next.retainedResults + 1
					} else if err := budget.releaseResults(next.retainedResults); err != nil {
						return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(item), location))
					}
				}
				result.raw = out
				return result
			}

			if result.raw == nil {
				if onlySelectorSeparators(remaining) {
					result.typ = Interface
					continue
				}
				result.typ = Invalid
				result.diagnosis = append(result.diagnosis, wrapResolutionError(ErrInvalidValue, nil, location))
				return result
			}
			tp, tv = reflect.TypeOf(result.raw), reflect.ValueOf(result.raw)
			if !tv.IsValid() {
				result.typ = Invalid
				result.diagnosis = append(result.diagnosis, wrapResolutionError(ErrInvalidValue, tp, location))
				return result
			}
			result.typ = Interface
			continue
		}

		location := state.location(token)
		if token.kind == selectorExpansionToken {
			if err := budget.consumeOperations(1); err != nil {
				result.deployment = true
				return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(result.raw), location))
			}
			if result.deployment {
				list := result.raw.([]interface{})
				out := makeResultList(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(list), location))
				}
				result.retainedResults = 0
				for _, item := range list {
					raw, typ, err := deploymentWithBudget(reflect.TypeOf(item), reflect.ValueOf(item), 0, budget)
					if errors.Is(err, ErrExpansionLimit) {
						return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(item), location))
					}
					failure := err
					if failure == nil && typ == Invalid {
						failure = ErrInvalidValue
					}
					if failure != nil {
						result.diagnosis = append(result.diagnosis, wrapResolutionError(failure, reflect.TypeOf(item), location))
					}
					if typ != Invalid {
						out = append(out, raw...)
						result.retainedResults += len(raw)
					} else if err := budget.releaseResults(len(raw)); err != nil {
						return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(item), location))
					}
				}
				result.raw = out
				continue
			}

			result.deployment = true
			src := result.raw
			raw, typ, err := deploymentWithBudget(reflect.TypeOf(src), reflect.ValueOf(src), 0, budget)
			if errors.Is(err, ErrExpansionLimit) {
				return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(src), location))
			}
			failure := err
			if failure == nil && typ == Invalid && src != nil {
				failure = ErrInvalidValue
			}
			if failure != nil {
				result.diagnosis = append(result.diagnosis, wrapResolutionError(failure, reflect.TypeOf(src), location))
			}
			result.raw, result.typ = raw, typ
			result.retainedResults = len(raw)
			continue
		}

		segment := token.value
		if result.deployment {
			list := result.raw.([]interface{})
			out := makeResultList(list)
			if err := budget.releaseResults(result.retainedResults); err != nil {
				return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(list), location))
			}
			result.retainedResults = 0
			for _, item := range list {
				resolved, typ, failureType, err := resolveSegment(reflect.TypeOf(item), reflect.ValueOf(item), segment)
				failure := err
				if failure == nil && typ == Invalid {
					failure = ErrInvalidValue
				}
				if failure != nil {
					result.diagnosis = append(result.diagnosis, wrapResolutionError(failure, failureType, location))
				}
				if typ != Invalid {
					if err := budget.retainResults(1); err != nil {
						return expansionLimitResult(result, wrapResolutionError(err, reflect.TypeOf(item), location))
					}
					out = append(out, resolved)
					result.retainedResults++
				}
			}
			result.raw = out
			continue
		}

		if result.raw == nil {
			result.typ = Invalid
			result.diagnosis = append(result.diagnosis, wrapResolutionError(ErrInvalidValue, nil, location))
			return result
		}
		nv, typ, failureType, err := resolveSegment(tp, tv, segment)
		if err != nil {
			result.diagnosis = append(result.diagnosis, wrapResolutionError(err, failureType, location))
		}
		result.raw, result.typ = nv, typ
		if typ == Invalid {
			return result
		}
		tp, tv = reflect.TypeOf(nv), reflect.ValueOf(nv)
	}

	return result
}

type selectorTokenKind uint8

const (
	selectorSegmentToken selectorTokenKind = iota
	selectorSeparatorToken
	selectorExpansionToken
)

type selectorToken struct {
	kind          selectorTokenKind
	value         string
	remainingPath string
	offset        uint64
	operation     uint64
}

// parseSelector validates selector syntax and decodes it once into the private
// token form shared by every traversal entry point.
func parseSelector(selector string) ([]selectorToken, error) {
	tokens := make([]selectorToken, 0, 8)
	segment := make([]byte, 0)
	started := false
	segmentOffset := 0
	var operation uint64
	flushSegment := func() {
		if !started {
			return
		}
		tokens = append(tokens, selectorToken{
			kind:      selectorSegmentToken,
			value:     string(segment),
			offset:    uint64(segmentOffset),
			operation: operation,
		})
		operation++
		segment = segment[:0]
		started = false
	}

	for index := 0; index < len(selector); {
		switch selector[index] {
		case '\\':
			if index+1 >= len(selector) {
				return nil, ErrInvalidSelector
			}
			if !started {
				segmentOffset = index
			}
			_, size := utf8.DecodeRuneInString(selector[index+1:])
			segment = append(segment, selector[index+1:index+1+size]...)
			started = true
			index += size + 1
		case '.':
			flushSegment()
			if len(tokens) == 0 || tokens[len(tokens)-1].kind != selectorSeparatorToken {
				tokens = append(tokens, selectorToken{
					kind:          selectorSeparatorToken,
					remainingPath: selector[index+1:],
				})
			}
			index++
		case '#':
			flushSegment()
			tokens = append(tokens, selectorToken{
				kind:      selectorExpansionToken,
				offset:    uint64(index),
				operation: operation,
			})
			operation++
			index++
		default:
			if !started && isIgnorableLeadingWhitespace(selector[index]) {
				index++
				continue
			}
			if !started {
				segmentOffset = index
			}
			segment = append(segment, selector[index])
			started = true
			index++
		}
	}
	flushSegment()
	return tokens, nil
}

func isIgnorableLeadingWhitespace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// resolveSegment chooses string or integer lookup only after observing the
// current container. This preserves numeric text as-is for string-keyed maps.
func resolveSegment(t reflect.Type, v reflect.Value, segment string) (interface{}, Type, reflect.Type, error) {
	kind, keyKind := selectorContainerKinds(t, v)
	if kind == reflect.Map && keyKind == reflect.String {
		return parseString(t, v, segment, 0)
	}
	if index, ok := parseSelectorIndex(segment); ok {
		return parseInt(t, v, index, 0)
	}
	return parseString(t, v, segment, 0)
}

func selectorContainerKinds(t reflect.Type, v reflect.Value) (reflect.Kind, reflect.Kind) {
	for depth := 0; t != nil && depth <= maxResolveDepth; depth++ {
		switch t.Kind() {
		case reflect.Ptr:
			if !v.IsValid() || v.Kind() != reflect.Ptr || v.IsNil() {
				return reflect.Ptr, reflect.Invalid
			}
			t = t.Elem()
			v = v.Elem()
		case reflect.Interface:
			if !v.IsValid() || v.IsNil() {
				return reflect.Interface, reflect.Invalid
			}
			v = v.Elem()
			t = v.Type()
		default:
			if t.Kind() == reflect.Map {
				return reflect.Map, t.Key().Kind()
			}
			return t.Kind(), reflect.Invalid
		}
	}
	return reflect.Invalid, reflect.Invalid
}

func parseSelectorIndex(segment string) (int, bool) {
	if segment == "" {
		return 0, false
	}

	start := 0
	switch segment[0] {
	case '+':
		start = 1
	case '-':
		if len(segment) == 1 {
			return 0, false
		}
		for index := 1; index < len(segment); index++ {
			if segment[index] != '0' {
				return 0, false
			}
		}
		return 0, true
	}
	if start == len(segment) {
		return 0, false
	}
	for index := start; index < len(segment); index++ {
		if segment[index] < '0' || segment[index] > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(segment)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func makeResultList(source []interface{}) []interface{} {
	if source == nil {
		return nil
	}
	return make([]interface{}, 0, len(source))
}

func onlySelectorSeparators(tokens []selectorToken) bool {
	for _, token := range tokens {
		if token.kind != selectorSeparatorToken {
			return false
		}
	}
	return true
}

// GetMany resolves several paths against the same value and returns one Result
// per path, in order.
func GetMany(value interface{}, path ...string) []Result {
	results := make([]Result, 0, len(path))
	for _, s := range path {
		results = append(results, Get(value, s))
	}
	return results
}

// The expansion limits are intentionally fixed and internal: they bound the
// CPU work and retained result size of one public Get/GetE or Result.Get/GetE
// call without adding configuration to the existing API. Ten thousand results
// keep a returned []interface{} in the low hundreds of KiB, while the larger
// operation allowance leaves room for normal fan-out and repeated expansion.
// Each '#' and each value it scans consume one operation.
const (
	maxExpansionOperations = 100_000
	maxExpansionResults    = 10_000
)

type expansionBudget struct {
	operations int
	results    int // expanded list slots currently retained across the result tree
	err        error
}

func (b *expansionBudget) consumeOperations(count int) error {
	if b == nil || count == 0 {
		return nil
	}
	if b.err != nil {
		return b.err
	}
	if count < 0 || count > maxExpansionOperations-b.operations {
		b.err = fmt.Errorf("expansion operations exceed %d: %w", maxExpansionOperations, ErrExpansionLimit)
		return b.err
	}
	b.operations += count
	return nil
}

func (b *expansionBudget) retainResults(count int) error {
	if b == nil || count == 0 {
		return nil
	}
	if b.err != nil {
		return b.err
	}
	if count < 0 || count > maxExpansionResults-b.results {
		b.err = fmt.Errorf("expansion results exceed %d: %w", maxExpansionResults, ErrExpansionLimit)
		return b.err
	}
	b.results += count
	return nil
}

func (b *expansionBudget) releaseResults(count int) error {
	if b == nil || count == 0 {
		return nil
	}
	if b.err != nil {
		return b.err
	}
	if count < 0 || count > b.results {
		b.err = fmt.Errorf("invalid expansion result accounting: %w", ErrExpansionLimit)
		return b.err
	}
	b.results -= count
	return nil
}

func invalidExpansionResult(result Result) Result {
	result.typ = Invalid
	result.raw = nil
	result.deployment = true
	result.retainedResults = 0
	return result
}

func expansionLimitResult(result Result, err error) Result {
	result = invalidExpansionResult(result)
	for _, diagnosis := range result.diagnosis {
		if errors.Is(diagnosis, ErrExpansionLimit) {
			return result
		}
	}
	result.diagnosis = append(result.diagnosis, err)
	return result
}

// maxResolveDepth bounds every recursive descent in this package: parseString
// (pointer deref + anonymous-field promotion), parseInt (pointer deref),
// deployment (pointer/interface deref), and the get/getE expansion of "#."
// segments. Cyclic data
// (a self-pointing embedded field, a self-referential interface) or an
// adversarial "#.#.#..." path would otherwise recurse forever and hit an
// unrecoverable fatal "stack overflow". Past the cap the walk stops with an
// Invalid result / error instead of crashing.
const maxResolveDepth = 1000

func parseString(t reflect.Type, v reflect.Value, value string, depth int) (interface{}, Type, reflect.Type, error) {
	if depth > maxResolveDepth {
		return nil, Invalid, t, ErrInvalidStructure
	}
	if !v.IsValid() {
		return nil, Invalid, t, nil
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil, Invalid, t, nil
		}
		if !v.Elem().IsValid() {
			return nil, Invalid, t, nil
		}
		// Derive the type from the dereferenced value, not t.Elem(): for an
		// interface kind t.Elem() panics, whereas v.Elem().Type() yields the
		// concrete dynamic type.
		ev := v.Elem()
		return parseString(ev.Type(), ev, value, depth+1)

	case reflect.Map:
		// MUST map key is string
		if t.Key().Kind() != reflect.String {
			return nil, Invalid, t, ErrMapKeyMustString
		}

		// Convert the lookup key to the map's actual key type. A defined key
		// type (type K string) has Kind String but is not assignable from a
		// plain string, which would make MapIndex panic.
		key := reflect.ValueOf(value)
		if key.Type() != t.Key() {
			key = key.Convert(t.Key())
		}
		mv := v.MapIndex(key)
		if !mv.IsValid() {
			return nil, Invalid, t, nil
		}

		return mv.Interface(), Type(mv.Kind()), t, nil

	case reflect.Slice, reflect.Array:
		return nil, Invalid, t, ErrSliceSubscript

	case reflect.Struct:
		rt, ok := t.FieldByName(value)
		if !ok {
			return nil, Invalid, t, nil
		}

		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		rv, err := cp.FieldByIndexErr(rt.Index)
		if err != nil {
			return nil, Invalid, t, ErrInvalidValue
		}

		res := reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Interface()

		if rt.Anonymous {
			nv, nt, failureType, ne := parseString(reflect.TypeOf(res), reflect.ValueOf(res), value, depth+1)
			if ne == nil && nt != Invalid {
				return nv, nt, failureType, ne
			}
		}

		return res, Type(rv.Kind()), t, nil

	default:
		return nil, Invalid, t, ErrInvalidStructure
	}
}

func parseInt(t reflect.Type, v reflect.Value, tokenValue int, depth int) (interface{}, Type, reflect.Type, error) {
	if depth > maxResolveDepth {
		return nil, Invalid, t, ErrParseInt
	}
	if !v.IsValid() {
		return nil, Invalid, t, nil
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil, Invalid, t, nil
		}
		if !v.Elem().IsValid() {
			return nil, Invalid, t, nil
		}
		ev := v.Elem()
		return parseInt(ev.Type(), ev, tokenValue, depth+1)
	case reflect.Map:
		// MUST map key is int
		if t.Key().Kind() != reflect.Int {
			return nil, Invalid, t, ErrMapKeyMustInt
		}

		// Convert to the map's actual key type (handles type K int).
		key := reflect.ValueOf(tokenValue)
		if key.Type() != t.Key() {
			key = key.Convert(t.Key())
		}
		mv := v.MapIndex(key)
		if !mv.IsValid() {
			return nil, Invalid, t, nil
		}

		return mv.Interface(), Type(mv.Kind()), t, nil

	case reflect.Slice, reflect.Array:
		if tokenValue < 0 || tokenValue >= v.Len() {
			return nil, Invalid, t, ErrIndexOutOfBounds
		}

		value := v.Index(tokenValue)
		if !value.IsValid() {
			return nil, Invalid, t, nil
		}
		return value.Interface(), Type(value.Kind()), t, nil

	case reflect.Struct:
		if tokenValue < 0 || tokenValue >= v.NumField() {
			return nil, Invalid, t, ErrStructIndexOutOfBounds
		}

		value := v.Field(tokenValue)
		if !value.IsValid() {
			return nil, Invalid, t, nil
		}

		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		value = cp.Field(tokenValue)

		res := reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface()

		return res, Type(value.Kind()), t, nil

	default:
		return nil, Invalid, t, ErrParseInt
	}
}

func deployment(t reflect.Type, v reflect.Value, depth int) ([]interface{}, Type, error) {
	return deploymentWithBudget(t, v, depth, nil)
}

func deploymentWithBudget(t reflect.Type, v reflect.Value, depth int, budget *expansionBudget) ([]interface{}, Type, error) {
	if depth > maxResolveDepth {
		return nil, Invalid, ErrUnableExpand
	}
	if !v.IsValid() {
		return nil, Invalid, nil
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil, Invalid, nil
		}

		if !v.Elem().IsValid() {
			return nil, Invalid, nil
		}
		return deploymentWithBudget(t, v.Elem(), depth+1, budget)

	case reflect.Map:
		if err := budget.consumeOperations(v.Len()); err != nil {
			return nil, Invalid, err
		}
		if err := budget.retainResults(v.Len()); err != nil {
			return nil, Invalid, err
		}
		var ret []interface{}
		var tp Type = Interface
		if v.Len() > 0 {
			ret = make([]interface{}, 0, v.Len())
		}

		iter := v.MapRange()
		for iter.Next() {
			ret = append(ret, iter.Value().Interface())
			if tp != Type(iter.Value().Kind()) {
				tp = Type(iter.Value().Kind())
			}
		}
		return ret, tp, nil

	case reflect.Slice, reflect.Array:
		if err := budget.consumeOperations(v.Len()); err != nil {
			return nil, Invalid, err
		}
		if err := budget.retainResults(v.Len()); err != nil {
			return nil, Invalid, err
		}
		var ret []interface{}
		var tp Type = Interface
		if v.Len() > 0 {
			ret = make([]interface{}, 0, v.Len())
		}

		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).IsValid() {
				return nil, Invalid, nil
			}
			ret = append(ret, v.Index(i).Interface())
			if tp != Type(v.Index(i).Kind()) {
				tp = Type(v.Index(i).Kind())
			}
		}
		return ret, tp, nil

	case reflect.Struct:
		if err := budget.consumeOperations(v.NumField()); err != nil {
			return nil, Invalid, err
		}
		if err := budget.retainResults(v.NumField()); err != nil {
			return nil, Invalid, err
		}
		var ret []interface{}
		var cp reflect.Value
		if v.NumField() > 0 {
			ret = make([]interface{}, 0, v.NumField())
		}

		for i := 0; i < v.NumField(); i++ {
			value := v.Field(i)
			if !value.IsValid() {
				return nil, Invalid, nil
			}

			if value.CanInterface() {
				ret = append(ret, value.Interface())
			} else {
				if !cp.IsValid() {
					cp = reflect.New(v.Type()).Elem()
					cp.Set(v)
				}
				value = cp.Field(i)

				ret = append(ret, reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface())
			}
		}

		return ret, Interface, nil

	default:
		if err := budget.consumeOperations(1); err != nil {
			return nil, Invalid, err
		}
		if err := budget.retainResults(1); err != nil {
			return nil, Invalid, err
		}
		return []interface{}{v.Interface()}, Type(v.Kind()), ErrUnableExpand
	}
}

const (
	maxContextTypeTokenLen = 256
	maxContextErrorLen     = 352
	contextTruncatedMarker = "<truncated>"
)

// resolutionState is independent from expansionBudget: selector tokenization
// owns syntax and immutable source spans, while this per-call state owns only
// the original selector length used to assemble bounded error context.
type resolutionState struct {
	totalLen uint64
}

type resolutionLocation struct {
	offset    uint64
	operation uint64
	totalLen  uint64
}

func newResolutionState(path string) resolutionState {
	return resolutionState{totalLen: uint64(len(path))}
}

func (s resolutionState) location(token selectorToken) resolutionLocation {
	return resolutionLocation{offset: token.offset, operation: token.operation, totalLen: s.totalLen}
}

func (s resolutionState) firstOperation(tokens []selectorToken) resolutionLocation {
	for _, token := range tokens {
		if token.kind != selectorSeparatorToken {
			return s.location(token)
		}
	}
	return resolutionLocation{offset: s.totalLen, totalLen: s.totalLen}
}

func (s resolutionState) selectorError(path string) resolutionLocation {
	offset := uint64(0)
	if len(path) > 0 {
		offset = uint64(len(path) - 1)
	}
	return resolutionLocation{offset: offset, totalLen: s.totalLen}
}

type resolutionError struct {
	err      error
	typeName string
	location resolutionLocation
}

func (e *resolutionError) Error() string {
	suffix := fmt.Sprintf(",offset=%d,op=%d,total_len=%d: %s", e.location.offset, e.location.operation, e.location.totalLen, e.err.Error())
	tokenLimit := maxContextTypeTokenLen
	if available := maxContextErrorLen - len("type=") - len(suffix); available < tokenLimit {
		tokenLimit = available
	}
	return "type=" + encodeTypeToken(e.typeName, tokenLimit) + suffix
}

func (e *resolutionError) Unwrap() error {
	return e.err
}

func wrapResolutionError(err error, currentType reflect.Type, location resolutionLocation) error {
	if err == nil {
		return nil
	}
	var contextual *resolutionError
	if errors.As(err, &contextual) {
		return err
	}
	typeName := "<nil>"
	if currentType != nil {
		typeName = currentType.String()
	}
	return &resolutionError{err: err, typeName: typeName, location: location}
}

func appendResultFailure(diagnosis []error, fatal error, childDiagnosis []error) []error {
	var fatalContext *resolutionError
	if errors.As(fatal, &fatalContext) {
		for _, child := range childDiagnosis {
			var childContext *resolutionError
			if errors.As(child, &childContext) && childContext == fatalContext {
				return append(diagnosis, childDiagnosis...)
			}
		}
	}
	diagnosis = append(diagnosis, fatal)
	return append(diagnosis, childDiagnosis...)
}

func encodeTypeToken(typeName string, limit int) string {
	quoted := strconv.QuoteToASCII(typeName)
	if len(quoted) <= limit {
		return quoted
	}

	markerOnly := strconv.QuoteToASCII(contextTruncatedMarker)
	if limit < len(markerOnly) {
		return markerOnly
	}

	end := len(typeName)
	if end > limit {
		end = limit
	}
	for end > 0 && !utf8.ValidString(typeName[:end]) {
		end--
	}
	for {
		candidate := strconv.QuoteToASCII(typeName[:end] + contextTruncatedMarker)
		if len(candidate) <= limit {
			return candidate
		}
		if end == 0 {
			return markerOnly
		}
		_, size := utf8.DecodeLastRuneInString(typeName[:end])
		end -= size
	}
}

// cloneTail returns a fresh slice holding list[ln:]. Re-slicing (list[ln:])
// would keep the whole backing array alive — including the ln consumed
// elements — for the lifetime of the Result; copying severs that reference so
// the consumed inputs can be garbage-collected. The nil vs empty distinction of
// the original is preserved so Value()/Values() stay observationally identical.
func cloneTail(list []interface{}, ln int) []interface{} {
	n := len(list) - ln
	if n <= 0 {
		if list == nil {
			return nil
		}
		return []interface{}{}
	}
	out := make([]interface{}, n)
	copy(out, list[ln:])
	return out
}

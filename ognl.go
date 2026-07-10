// Package ognl extracts values from arbitrary Go object graphs using a string
// path expression, in the spirit of OGNL. It traverses structs, maps, slices,
// arrays, pointers and interfaces via reflection, without the caller having to
// know the concrete types in advance, and can read unexported fields.
//
// Path syntax:
//
//   - "." separates path segments: "A.B.C".
//   - A segment is a map key, a struct field name, or a numeric index. A
//     non-negative integer indexes a slice/array element, a map keyed by int,
//     or a struct field by position; anything else is a string map key or a
//     struct field name.
//   - "#" expands the current value into a list: subsequent segments are then
//     applied to every element (similar to flat-map). "##" expands twice.
//     Expansion work and result count are bounded per Get/GetE call.
//   - Use "\\" to escape a literal "." inside a key, e.g. "Foo\\.Bar".
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
		return nil, wrapError(err, r.raw, "")
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
	return r.get(path, &expansionBudget{})
}

func (r Result) get(path string, budget *expansionBudget) Result {
	if r.deployment {
		nr := r
		nr.retainedResults = 0
		src, _ := r.raw.([]interface{})
		out := make([]interface{}, 0, len(src))
		diag := append([]error(nil), r.diagnosis...)
		for _, item := range src {
			next := get(item, path, 0, budget)
			diag = append(diag, next.diagnosis...)
			if budget.err != nil {
				nr.diagnosis = diag
				return expansionLimitResult(nr, budget.err, item, path)
			}
			if next.typ != Invalid {
				if next.deployment {
					out = append(out, next.raw.([]interface{})...)
					nr.retainedResults += next.retainedResults
				} else {
					if err := budget.retainResults(1); err != nil {
						nr.diagnosis = diag
						return expansionLimitResult(nr, err, item, path)
					}
					out = append(out, next.raw)
					nr.retainedResults += next.retainedResults + 1
				}
			} else if err := budget.releaseResults(next.retainedResults); err != nil {
				nr.diagnosis = diag
				return expansionLimitResult(nr, err, item, path)
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
	return get(r.raw, path, 0, budget)
}

// GetE is the error-returning form of Get. On an expanded Result it returns an
// error when no element matched path or when expansion exceeds its per-call
// limit. Per contract C3, creating an empty expansion succeeds, while applying
// another path to it returns ErrInvalidValue. Like Get, it never mutates r and
// is safe to call concurrently.
func (r Result) GetE(path string) (Result, error) {
	return r.getE(path, &expansionBudget{})
}

func (r Result) getE(path string, budget *expansionBudget) (Result, error) {
	if r.deployment {
		nr := r
		nr.retainedResults = 0
		src, _ := r.raw.([]interface{})
		out := make([]interface{}, 0, len(src))
		diag := append([]error(nil), r.diagnosis...)
		for _, item := range src {
			next, err := getE(item, path, 0, budget)
			if budget.err != nil || errors.Is(err, ErrExpansionLimit) {
				if err == nil {
					err = budget.err
				}
				return invalidExpansionResult(nr), wrapError(err, item, path)
			}
			if err != nil {
				diag = append(diag, wrapError(err, item, path))
			}
			if next.typ != Invalid {
				if next.deployment {
					out = append(out, next.raw.([]interface{})...)
					nr.retainedResults += next.retainedResults
				} else {
					if err := budget.retainResults(1); err != nil {
						return invalidExpansionResult(nr), wrapError(err, item, path)
					}
					out = append(out, next.raw)
					nr.retainedResults += next.retainedResults + 1
				}
			} else if err := budget.releaseResults(next.retainedResults); err != nil {
				return invalidExpansionResult(nr), wrapError(err, item, path)
			}
			diag = append(diag, next.diagnosis...)
		}
		if len(out) == 0 && src == nil {
			out = nil
		}
		nr.raw = out
		nr.diagnosis = diag
		if len(out) == 0 {
			return nr, wrapError(ErrInvalidValue, src, path)
		}
		return nr, nil
	}
	return getE(r.raw, path, 0, budget)
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
	return getE(value, path, 0, &expansionBudget{})
}

func getE(value interface{}, path string, depth int, budget *expansionBudget) (Result, error) {
	if depth > maxResolveDepth {
		return Result{typ: Invalid, raw: value}, wrapError(ErrInvalidStructure, value, path)
	}
	if value == nil && (len(path) == 0 || validIdentifier(path, 0)) {
		return Result{typ: Interface, raw: value}, nil
	}
	if value == nil {
		return Result{typ: Invalid, raw: value}, wrapError(ErrInvalidValue, value, path)
	}

	var (
		index  int
		result = Result{
			typ: Interface,
			raw: value,
		}
	)
	tp := reflect.TypeOf(value)
	tv := reflect.ValueOf(value)

	if !tv.IsValid() {
		result.typ = Invalid
		return result, wrapError(ErrInvalidValue, value, path)
	}

	for ; index < len(path); index++ {
		switch path[index] {
		case ' ', '\t', '\n', '\r', '.':
			switch result.deployment {
			case true:
				list := result.raw.([]interface{})
				ln := len(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return invalidExpansionResult(result), wrapError(err, list, path[index+1:])
				}
				result.retainedResults = 0
				for i := 0; i < ln; i++ {
					next, err := getE(list[i], path[index+1:], depth+1, budget)
					if budget.err != nil || errors.Is(err, ErrExpansionLimit) {
						if err == nil {
							err = budget.err
						}
						return invalidExpansionResult(result), wrapError(err, list[i], path[index+1:])
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, wrapError(err, list[i], path[index+1:]))
					}
					if next.typ != Invalid {
						if err := budget.retainResults(1); err != nil {
							return invalidExpansionResult(result), wrapError(err, list[i], path[index+1:])
						}
						list = append(list, next.raw)
						result.retainedResults += next.retainedResults + 1
					} else if err := budget.releaseResults(next.retainedResults); err != nil {
						return invalidExpansionResult(result), wrapError(err, list[i], path[index+1:])
					}

					result.diagnosis = append(result.diagnosis, next.diagnosis...)
				}
				result.raw = cloneTail(list, ln)
				if len(list[ln:]) == 0 {
					return result, wrapError(ErrInvalidValue, list, string(path[index]))
				}
				return result, nil
			default:
				// Iterative descent (no recursion): a separator just re-bases
				// tp/tv on the current value and lets the loop continue. This
				// keeps stack depth O(1) in the number of separators, so a path
				// with millions of '.' (or a self-referential "Mid.Mid.Mid..."
				// path) can no longer exhaust the goroutine stack.
				if result.raw == nil {
					if len(path[index+1:]) == 0 || validIdentifier(path[index+1:], 0) {
						result.typ = Interface
						continue
					}
					result.typ = Invalid
					return result, wrapError(ErrInvalidValue, result.raw, path[index+1:])
				}
				tp = reflect.TypeOf(result.raw)
				tv = reflect.ValueOf(result.raw)
				if !tv.IsValid() {
					result.typ = Invalid
					return result, wrapError(ErrInvalidValue, result.raw, path[index+1:])
				}
				result.typ = Interface
			}
		case '#':
			if err := budget.consumeOperations(1); err != nil {
				result.deployment = true
				return invalidExpansionResult(result), wrapError(err, result.raw, "#")
			}
			// 转化成slice 类型 平铺
			switch result.deployment {
			case true:
				// queue 展开
				list := result.raw.([]interface{})
				ln := len(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return invalidExpansionResult(result), wrapError(err, list, "#")
				}
				result.retainedResults = 0
				for i := 0; i < ln; i++ {
					raw, typ, err := deploymentWithBudget(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), 0, budget)
					if errors.Is(err, ErrExpansionLimit) {
						return invalidExpansionResult(result), wrapError(err, list[i], "#")
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, wrapError(err, list[i], "#"))
					}
					if typ != Invalid {
						list = append(list, raw...)
						result.retainedResults += len(raw)
					} else if err := budget.releaseResults(len(raw)); err != nil {
						return invalidExpansionResult(result), wrapError(err, list[i], "#")
					}
				}
				result.raw = cloneTail(list, ln)
				if len(list[ln:]) == 0 {
					return result, wrapError(ErrInvalidValue, list, "#")
				}
			default:
				result.deployment = true
				// Expand the value we have descended to (result.raw), not the
				// stale entry value captured in tp/tv. Write raw/typ back BEFORE
				// the error check so the deployed Result is always consistent
				// (raw is a []interface{}) — otherwise Effective()/Values() would
				// panic on the type assertion when deployment fails (e.g. on a
				// scalar "#").
				src := result.raw
				raw, typ, err := deploymentWithBudget(reflect.TypeOf(src), reflect.ValueOf(src), 0, budget)
				result.raw, result.typ = raw, typ
				if errors.Is(err, ErrExpansionLimit) {
					return invalidExpansionResult(result), wrapError(err, src, "#")
				}
				result.retainedResults = len(raw)
				if err != nil {
					return result, wrapError(err, src, "#")
				}
			}

		default:
			key, newIndex := parseNextKey(path, index)
			index = newIndex
			sv := key
			v, err := strconv.Atoi(sv)
			digit := err == nil && v >= 0
			switch result.deployment {
			case true:
				var (
					value interface{}
					tp    Type
				)
				list := result.raw.([]interface{})
				ln := len(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return invalidExpansionResult(result), wrapError(err, list, sv)
				}
				result.retainedResults = 0
				for i := 0; i < ln; i++ {
					if digit {
						value, tp, err = parseInt(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), v, 0)
					} else {
						value, tp, err = parseString(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), sv, 0)
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, wrapError(err, list[i], sv))
					}
					if tp != Invalid {
						if err := budget.retainResults(1); err != nil {
							return invalidExpansionResult(result), wrapError(err, list[i], sv)
						}
						list = append(list, value)
						result.retainedResults++
					}
				}
				result.raw = cloneTail(list, ln)
				if len(list[ln:]) == 0 {
					return result, wrapError(ErrInvalidValue, list, sv)
				}

			default:
				var (
					nv  interface{}
					tpe Type
				)
				if digit {
					nv, tpe, err = parseInt(tp, tv, v, 0)
				} else {
					nv, tpe, err = parseString(tp, tv, sv, 0)
				}
				result.raw = nv
				result.typ = tpe
				if err != nil {
					return result, wrapError(err, value, sv)
				}
				if tpe == Invalid {
					return result, wrapError(ErrInvalidValue, value, sv)
				}
			}
		}
	}

	return result, nil
}

// Get resolves path against value and returns the Result. It does not return an
// error: an unresolved path yields a Result whose Effective reports false, and
// non-fatal problems are recorded in Result.Diagnosis. Expansion work and
// result count are bounded, so adversarial '#' paths cannot exhaust memory or
// CPU. See the package doc for the path syntax.
func Get(value interface{}, path string) Result {
	return get(value, path, 0, &expansionBudget{})
}

func get(value interface{}, path string, depth int, budget *expansionBudget) Result {
	if depth > maxResolveDepth {
		return Result{typ: Invalid, raw: value, diagnosis: []error{wrapError(ErrInvalidStructure, value, path)}}
	}
	if value == nil && (len(path) == 0 || validIdentifier(path, 0)) {
		return Result{typ: Interface, raw: value}
	}
	if value == nil {
		return Result{typ: Invalid, raw: value, diagnosis: []error{wrapError(ErrInvalidValue, value, path)}}
	}

	var index int

	var result = Result{
		typ: Interface,
		raw: value,
	}
	tp := reflect.TypeOf(value)
	tv := reflect.ValueOf(value)

	if !tv.IsValid() {
		result.diagnosis = append(result.diagnosis, wrapError(ErrInvalidValue, value, path))
		return result
	}

	for ; index < len(path); index++ {
		switch path[index] {
		case ' ', '\t', '\n', '\r', '.':
			switch result.deployment {
			case true:
				list := result.raw.([]interface{})
				ln := len(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return expansionLimitResult(result, err, list, path[index+1:])
				}
				result.retainedResults = 0
				for i := 0; i < ln; i++ {
					next := get(list[i], path[index+1:], depth+1, budget)
					result.diagnosis = append(result.diagnosis, next.diagnosis...)
					if budget.err != nil {
						return expansionLimitResult(result, budget.err, list[i], path[index+1:])
					}
					if next.typ != Invalid {
						if err := budget.retainResults(1); err != nil {
							return expansionLimitResult(result, err, list[i], path[index+1:])
						}
						list = append(list, next.raw)
						result.retainedResults += next.retainedResults + 1
					} else if err := budget.releaseResults(next.retainedResults); err != nil {
						return expansionLimitResult(result, err, list[i], path[index+1:])
					}
				}
				result.raw = cloneTail(list, ln)
				return result
			default:
				// Iterative descent; see the GetE counterpart for rationale.
				if result.raw == nil {
					if len(path[index+1:]) == 0 || validIdentifier(path[index+1:], 0) {
						result.typ = Interface
						continue
					}
					result.diagnosis = append(result.diagnosis, wrapError(ErrInvalidValue, result.raw, path[index+1:]))
					result.typ = Invalid
					return result
				}
				tp = reflect.TypeOf(result.raw)
				tv = reflect.ValueOf(result.raw)
				if !tv.IsValid() {
					result.diagnosis = append(result.diagnosis, wrapError(ErrInvalidValue, result.raw, path[index+1:]))
					result.typ = Invalid
					return result
				}
				result.typ = Interface
			}
		case '#':
			if err := budget.consumeOperations(1); err != nil {
				result.deployment = true
				return expansionLimitResult(result, err, result.raw, "#")
			}
			// 转化成slice 类型 平铺
			switch result.deployment {
			case true:
				// queue 展开
				list := result.raw.([]interface{})
				ln := len(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return expansionLimitResult(result, err, list, "#")
				}
				result.retainedResults = 0
				for i := 0; i < ln; i++ {
					raw, typ, err := deploymentWithBudget(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), 0, budget)
					if errors.Is(err, ErrExpansionLimit) {
						return expansionLimitResult(result, err, list[i], "#")
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, wrapError(err, list[i], "#"))
					}
					if typ != Invalid {
						list = append(list, raw...)
						result.retainedResults += len(raw)
					} else if err := budget.releaseResults(len(raw)); err != nil {
						return expansionLimitResult(result, err, list[i], "#")
					}
				}
				result.raw = cloneTail(list, ln)
			default:
				result.deployment = true
				// Expand result.raw, not the stale entry value in tp/tv.
				src := result.raw
				raw, typ, err := deploymentWithBudget(reflect.TypeOf(src), reflect.ValueOf(src), 0, budget)
				if errors.Is(err, ErrExpansionLimit) {
					return expansionLimitResult(result, err, src, "#")
				}
				if err != nil {
					result.diagnosis = append(result.diagnosis, wrapError(err, src, "#"))
				}
				result.raw, result.typ = raw, typ
				result.retainedResults = len(raw)
			}

		default:
			key, newIndex := parseNextKey(path, index)
			index = newIndex
			sv := key
			v, err := strconv.Atoi(sv)
			digit := err == nil && v >= 0
			switch result.deployment {
			case true:
				var (
					value interface{}
					tp    Type
				)
				list := result.raw.([]interface{})
				ln := len(list)
				if err := budget.releaseResults(result.retainedResults); err != nil {
					return expansionLimitResult(result, err, list, sv)
				}
				result.retainedResults = 0
				for i := 0; i < ln; i++ {
					if digit {
						value, tp, err = parseInt(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), v, 0)
					} else {
						value, tp, err = parseString(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), sv, 0)
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, wrapError(err, list[i], sv))
					}
					if tp != Invalid {
						if err := budget.retainResults(1); err != nil {
							return expansionLimitResult(result, err, list[i], sv)
						}
						list = append(list, value)
						result.retainedResults++
					}
				}
				result.raw = cloneTail(list, ln)
			default:
				var (
					nv  interface{}
					tpe Type
				)
				if digit {
					nv, tpe, err = parseInt(tp, tv, v, 0)
				} else {
					nv, tpe, err = parseString(tp, tv, sv, 0)
				}
				if err != nil {
					result.diagnosis = append(result.diagnosis, wrapError(err, value, sv))
				}
				result.raw = nv
				result.typ = tpe
				if tpe == Invalid {
					return result
				}
			}
		}
	}

	return result
}

// 从选择器中解析出下一个要处理的key
// params:
// selector: 路径选择器，比如"Foo.Bar.Name"，比如"Foo\\.Bar\\.Name"
// index: 选择器上次消费到的位置
//
// returns:
// string 下一个要解析的key
// index selector被消费到的位置
func parseNextKey(selector string, index int) (string, int) {
	key := make([]byte, 0)
loop:
	for ; index < len(selector); index++ {
		switch selector[index] {
		case '\\':
			// 先跳过转义字符
			index++
			// 转义字符，无论下一个字符是什么都跳过，如果有的话
			if index < len(selector) {
				key = append(key, selector[index])
			}
		case '.', '#':
			index--
			break loop
		default:
			// 普通字符，当做key的一部分消费掉
			key = append(key, selector[index])
		}
	}
	return string(key), index
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

func expansionLimitResult(result Result, err error, object interface{}, path string) Result {
	result = invalidExpansionResult(result)
	for _, diagnosis := range result.diagnosis {
		if errors.Is(diagnosis, ErrExpansionLimit) {
			return result
		}
	}
	result.diagnosis = append(result.diagnosis, wrapError(err, object, path))
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

func parseString(t reflect.Type, v reflect.Value, value string, depth int) (interface{}, Type, error) {
	if depth > maxResolveDepth {
		return nil, Invalid, ErrInvalidStructure
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
		// Derive the type from the dereferenced value, not t.Elem(): for an
		// interface kind t.Elem() panics, whereas v.Elem().Type() yields the
		// concrete dynamic type.
		ev := v.Elem()
		return parseString(ev.Type(), ev, value, depth+1)

	case reflect.Map:
		// MUST map key is string
		if t.Key().Kind() != reflect.String {
			return nil, Invalid, ErrMapKeyMustString
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
			return nil, Invalid, nil
		}

		return mv.Interface(), Type(mv.Kind()), nil

	case reflect.Slice, reflect.Array:
		return nil, Invalid, ErrSliceSubscript

	case reflect.Struct:
		rt, ok := t.FieldByName(value)
		if !ok {
			return nil, Invalid, nil
		}

		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		rv, err := cp.FieldByIndexErr(rt.Index)
		if err != nil {
			return nil, Invalid, ErrInvalidValue
		}

		res := reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Interface()

		if rt.Anonymous {
			nv, nt, ne := parseString(reflect.TypeOf(res), reflect.ValueOf(res), value, depth+1)
			if ne == nil && nt != Invalid {
				return nv, nt, ne
			}
		}

		return res, Type(rv.Kind()), nil

	default:
		return nil, Invalid, ErrInvalidStructure
	}
}

func parseInt(t reflect.Type, v reflect.Value, tokenValue int, depth int) (interface{}, Type, error) {
	if depth > maxResolveDepth {
		return nil, Invalid, ErrParseInt
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
		ev := v.Elem()
		return parseInt(ev.Type(), ev, tokenValue, depth+1)
	case reflect.Map:
		// MUST map key is int
		if t.Key().Kind() != reflect.Int {
			return nil, Invalid, ErrMapKeyMustInt
		}

		// Convert to the map's actual key type (handles type K int).
		key := reflect.ValueOf(tokenValue)
		if key.Type() != t.Key() {
			key = key.Convert(t.Key())
		}
		mv := v.MapIndex(key)
		if !mv.IsValid() {
			return nil, Invalid, nil
		}

		return mv.Interface(), Type(mv.Kind()), nil

	case reflect.Slice, reflect.Array:
		if tokenValue < 0 || tokenValue >= v.Len() {
			return nil, Invalid, ErrIndexOutOfBounds
		}

		value := v.Index(tokenValue)
		if !value.IsValid() {
			return nil, Invalid, nil
		}
		return value.Interface(), Type(value.Kind()), nil

	case reflect.Struct:
		if tokenValue < 0 || tokenValue >= v.NumField() {
			return nil, Invalid, ErrStructIndexOutOfBounds
		}

		value := v.Field(tokenValue)
		if !value.IsValid() {
			return nil, Invalid, nil
		}

		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		value = cp.Field(tokenValue)

		res := reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface()

		return res, Type(value.Kind()), nil

	default:
		return nil, Invalid, ErrParseInt
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

		for i := 0; i < v.NumField(); i++ {
			value := v.Field(i)
			if !value.IsValid() {
				return nil, Invalid, nil
			}

			if value.CanInterface() {
				ret = append(ret, value.Interface())
			} else {
				cp := reflect.New(v.Type()).Elem()
				cp.Set(v)
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

func validIdentifier(path string, limit int) bool {
	count := 0
	for _, v := range path {
		switch v {
		case ' ', '\t', '\n', '\r', '.':
		default:
			count++
			if count > limit {
				return false
			}
		}
	}
	return true
}

// wrapError annotates err with the path and the *type* of the object being
// traversed. The object's value is deliberately NOT included: this library
// walks arbitrary caller objects that routinely hold credentials or PII, and
// embedding the value (e.g. via %v) would leak secrets into error strings and
// the Diagnosis list. The sentinel is wrapped with %w so errors.Is still works.
func wrapError(err error, object interface{}, path string) error {
	return fmt.Errorf("type:%T,path:%s: %w", object, path, err)
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

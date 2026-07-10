package ognl

// Regression tests for the production-hardening pass. Each test pins a
// previously-confirmed defect (P0-*/P1-*) to its fixed behavior, so a
// regression re-surfaces as a normal test failure rather than a crash. The
// TestCoverage_* tests exercise public API that previously had no coverage.

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// P0-1: a path with a huge number of separators (or a self-referential
// "Next.Next..." path) used to recurse one stack frame per separator and crash
// with an unrecoverable fatal stack overflow. The walker is now iterative.
func TestP0_1_NoStackOverflow_Separators(t *testing.T) {
	value := map[string]interface{}{"a": 1}
	path := strings.Repeat(".", 1_000_000) + "a"

	r := Get(value, path)
	assert.Equal(t, 1, r.Value())

	r2, err := GetE(value, path)
	assert.NoError(t, err)
	assert.Equal(t, 1, r2.Value())
}

func TestP0_1_NoStackOverflow_DeepCyclicPath(t *testing.T) {
	type Node struct {
		Next *Node
		V    int
	}
	n := &Node{V: 7}
	n.Next = n // cycle

	path := strings.Repeat("Next.", 1_000_000) + "V"
	assert.Equal(t, 7, Get(n, path).Value())
}

// P0-2: an anonymous self-referential pointer field used to recurse forever in
// parseString/parseInt. The depth cap turns that into a graceful result.
type selfRef struct {
	*selfRef
	V int
}

func TestP0_2_AnonSelfRef_NoCrash(t *testing.T) {
	a := &selfRef{V: 1}
	a.selfRef = a // self cycle through the anonymous field

	// Must not crash; the embedded pointer body is returned, and other fields
	// remain reachable.
	r := Get(a, "selfRef")
	assert.True(t, r.Effective())
	assert.Equal(t, 1, Get(a, "V").Value())
}

// P0-3: "key#" used to expand the ROOT object instead of the value the path had
// descended to.
func TestP0_3_HashExpandsResolvedValue(t *testing.T) {
	value := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "alice"},
			map[string]interface{}{"name": "bob"},
		},
		"other": 1,
	}

	vs := Get(value, "users#").Values()
	assert.Len(t, vs, 2, "users# must expand the users slice, not the outer map")

	names := Get(value, "users#.name").Values()
	assert.ElementsMatch(t, []interface{}{"alice", "bob"}, names)
}

func TestP0_3_GetEHashExpandsResolvedValue(t *testing.T) {
	value := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "alice"},
			map[string]interface{}{"name": "bob"},
		},
		"other": 1,
	}

	result, err := GetE(value, "users#.name")
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"alice", "bob"}, result.Values())
}

// P0-4: a map keyed by a defined type (type K string / type K int) used to
// panic inside reflect.Value.MapIndex.
type namedKey string
type namedIntKey int

func TestP0_4_NamedMapKeys(t *testing.T) {
	ms := map[namedKey]int{namedKey("a"): 1, namedKey("b"): 2}
	assert.Equal(t, 1, Get(ms, "a").Value())

	mi := map[namedIntKey]string{namedIntKey(1): "x", namedIntKey(2): "y"}
	assert.Equal(t, "x", Get(mi, "1").Value())
}

// P0-5: Result.Get shallow-copied a deployed Result and appended into a shared
// backing array, racing under concurrency and corrupting sequential results.
func TestP0_5_ResultGet_Concurrent(t *testing.T) {
	type item struct {
		Name string
		Tags []string
	}
	value := []item{
		{Name: "a", Tags: []string{"x"}},
		{Name: "b", Tags: []string{"u"}},
		{Name: "c", Tags: []string{"p"}},
	}
	r := Get(value, "#")
	require.True(t, r.Effective())

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "Name"
			if i%2 == 0 {
				path = "Tags"
			}
			_ = r.Get(path)
		}(i)
	}
	wg.Wait()
}

func TestP0_5_ResultGet_NoSequentialCorruption(t *testing.T) {
	// White-box: force spare capacity in the deployed Result so the OLD
	// shared-backing append would write into the same array and the second
	// call would clobber the first call's returned slice. Querying *different*
	// keys makes the corruption observable (without spare cap, append reallocs
	// and the bug hides — which is why the naive version of this test passed
	// even against the buggy code).
	big := make([]interface{}, 3, 64)
	big[0] = map[string]interface{}{"k": 1, "j": 10}
	big[1] = map[string]interface{}{"k": 2, "j": 20}
	big[2] = map[string]interface{}{"k": 3, "j": 30}
	r := Result{deployment: true, raw: big, typ: Interface}

	a := r.Get("k")
	require.Equal(t, []interface{}{1, 2, 3}, a.Values())

	b := r.Get("j")
	require.Equal(t, []interface{}{10, 20, 30}, b.Values())

	// With the old list[ln:] aliasing, b's appends overwrite a's slice in place.
	assert.Equal(t, []interface{}{1, 2, 3}, a.Values(), "a must be unaffected by b")
}

func TestP0_5_ResultGetE_Concurrent(t *testing.T) {
	type item struct {
		Name string
		Tags []string
	}
	value := []item{
		{Name: "a", Tags: []string{"x"}},
		{Name: "b", Tags: []string{"u"}},
		{Name: "c", Tags: []string{"p"}},
	}
	r := Get(value, "#")
	require.True(t, r.Effective())

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "Name"
			if i%2 == 0 {
				path = "Tags"
			}
			_, _ = r.GetE(path)
		}(i)
	}
	wg.Wait()
}

func TestP0_5_ResultGetE_NoSequentialCorruption(t *testing.T) {
	big := make([]interface{}, 3, 64)
	big[0] = map[string]interface{}{"k": 1, "j": 10}
	big[1] = map[string]interface{}{"k": 2, "j": 20}
	big[2] = map[string]interface{}{"k": 3, "j": 30}
	r := Result{deployment: true, raw: big, typ: Interface}

	a, err := r.GetE("k")
	require.NoError(t, err)
	require.Equal(t, []interface{}{1, 2, 3}, a.Values())

	b, err := r.GetE("j")
	require.NoError(t, err)
	require.Equal(t, []interface{}{10, 20, 30}, b.Values())

	assert.Equal(t, []interface{}{1, 2, 3}, a.Values(), "a must be unaffected by b")
}

// P1-1: error messages used to embed the whole object via %v, leaking secrets.
type secretUser struct {
	Username string
	Password string
	APIToken string
}

func TestP1_1_ErrorDoesNotLeakSecrets(t *testing.T) {
	u := secretUser{Username: "alice", Password: "super-secret-pwd-123", APIToken: "sk-DEADBEEF"}
	_, err := GetE(u, "NoSuchField")
	require.Error(t, err)

	msg := err.Error()
	assert.NotContains(t, msg, "super-secret-pwd-123")
	assert.NotContains(t, msg, "sk-DEADBEEF")
	assert.True(t, errors.Is(err, ErrInvalidValue), "sentinel must remain unwrappable")
}

// P1-2: GetE's '#' branch used to append a nil error into Diagnosis.
func TestP1_2_GetEHash_NoNilDiagnosis(t *testing.T) {
	r, err := GetE([]int{1, 2, 3}, "#")
	require.NoError(t, err)
	assert.Empty(t, r.Diagnosis())
	for _, d := range r.Diagnosis() {
		assert.NotNil(t, d)
	}
}

// P1-3: list[ln:] re-slicing kept the consumed prefix alive via the shared
// backing array; cloneTail copies the tail into a fresh array so the prefix can
// be collected. Tested deterministically against cloneTail directly (a
// finalizer/GC test would be flaky and, worse, would not even exercise this
// function).
func TestP1_3_CloneTailCopiesAndPreservesNilEmpty(t *testing.T) {
	list := make([]interface{}, 0, 16)
	list = append(list, "a", "b", "c", "d", "e")

	out := cloneTail(list, 2)
	require.Equal(t, []interface{}{"c", "d", "e"}, out)

	// Must be a copy, not an alias of list's backing array.
	list[2] = "MUT"
	assert.Equal(t, "c", out[0], "cloneTail must copy, not re-slice")
	// And carry no leftover capacity from the (larger) source backing array.
	assert.Equal(t, len(out), cap(out))

	// nil vs empty-slice distinction preserved.
	assert.Nil(t, cloneTail(nil, 0))
	assert.NotNil(t, cloneTail([]interface{}{}, 0))
	assert.Len(t, cloneTail([]interface{}{"x"}, 1), 0)
}

// P0-1 (round 2): the deployment-mode separator branch recursed once per "#."
// transition, driven by path length — a path-only stack-overflow DoS reachable
// with a 1-element cyclic input. Now depth-bounded.
func TestP0_1_DeploymentSeparatorBounded(t *testing.T) {
	outer := make([]interface{}, 1)
	outer[0] = outer // cycle
	// Enough "#." pairs to overflow the 1 GB stack if the recursion were
	// unbounded; with the depth cap it returns after maxResolveDepth levels.
	path := strings.Repeat("#.", 1_000_000) + "x"

	// Before the fix this recursed once per "#." and crashed the process with a
	// fatal (uncatchable) stack overflow. Reaching the end of this test is the
	// regression check: the depth bound caps recursion regardless of path length.
	_ = Get(outer, path)
	_, _ = GetE(outer, path)
}

// deployment() pointer/interface deref had no depth bound: a self-referential
// interface looped forever. Now bounded.
func TestP0_1_DeploymentInterfaceCycleBounded(t *testing.T) {
	var x interface{}
	x = &x // self-referential interface
	assert.False(t, Get(x, "#").Effective())
}

// parseString/parseInt dereferenced via t.Elem(), which panics on interface
// types. A pointer-to-interface value used to crash; now it resolves.
func TestP0_4_InterfacePtrNoPanic(t *testing.T) {
	var x interface{} = map[string]int{"a": 1}
	assert.Equal(t, 1, Get(&x, "a").Value())

	var y interface{} = []int{10, 20, 30}
	assert.Equal(t, 20, Get(&y, "1").Value())
}

// GetE's '#' branch set deployment=true but, on a deployment error (e.g. a
// scalar that cannot be expanded), returned before writing raw back to a
// []interface{} — so Effective()/Values()/Get() panicked on the type assertion.
func TestGetE_ScalarHash_NoPanic(t *testing.T) {
	r, err := GetE(42, "#")
	assert.Error(t, err) // a scalar cannot be expanded

	assert.NotPanics(t, func() {
		_ = r.Effective()
		_ = r.Values()
		_ = r.Get("x")
	})
}

// ---- Coverage for previously-untested public API ----

func TestCoverage_GetMany(t *testing.T) {
	m := map[string]interface{}{"a": 1, "b": "two"}
	rs := GetMany(m, "a", "b", "missing")
	require.Len(t, rs, 3)
	assert.Equal(t, 1, rs[0].Value())
	assert.Equal(t, "two", rs[1].Value())
	assert.False(t, rs[2].Effective())
}

func TestCoverage_DiagnosisCollectsErrors(t *testing.T) {
	// Expand a mixed list, then index each element; the int element cannot be
	// indexed and produces a (non-fatal) diagnosis entry.
	value := []interface{}{[]int{10, 20}, 99}
	r := Get(value, "#1")
	assert.Equal(t, []interface{}{20}, r.Values())
	require.NotEmpty(t, r.Diagnosis())
	assert.True(t, errors.Is(r.Diagnosis()[0], ErrParseInt))
}

func TestCoverage_ValuesE(t *testing.T) {
	// Expandable single value: ValuesE returns its elements.
	vs, err := Get([]int{1, 2, 3}, "").ValuesE()
	require.NoError(t, err)
	assert.Equal(t, []interface{}{1, 2, 3}, vs)

	// Non-expandable scalar: ValuesE surfaces the expansion error.
	_, err = Get(42, "").ValuesE()
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnableExpand))
}

func TestCoverage_ChainedGetE(t *testing.T) {
	value := map[string]interface{}{
		"a": map[string]interface{}{"b": map[string]interface{}{"c": 42}},
	}
	r := Parse(value)
	r, err := r.GetE("a")
	require.NoError(t, err)
	r, err = r.GetE("b")
	require.NoError(t, err)
	r, err = r.GetE("c")
	require.NoError(t, err)
	assert.Equal(t, 42, r.Value())

	mid, err := Parse(value).GetE("a")
	require.NoError(t, err)
	_, err = mid.GetE("nope")
	assert.Error(t, err)
}

func TestCoverage_EmptyExpand(t *testing.T) {
	assert.False(t, Get([]int{}, "#").Effective())
	assert.False(t, Get(map[string]int{}, "#").Effective())
	assert.False(t, Get([0]int{}, "#").Effective())
}

func TestCoverage_NestedNil(t *testing.T) {
	type Sub struct{ Name string }
	type Holder struct{ S *Sub }

	// nil pointer field, descend through it -> graceful, no panic.
	assert.False(t, Get(Holder{S: nil}, "S.Name").Effective())

	// nil map value, descend through it -> graceful, no panic.
	m := map[string]interface{}{"k": nil}
	assert.False(t, Get(m, "k.Name").Effective())
}

// Locks the invariant the library relies on: the Type constants are declared in
// the same order as reflect.Kind, so Type(v.Kind()) is a value-preserving cast.
func TestCoverage_TypeMatchesReflectKind(t *testing.T) {
	pairs := []struct {
		kind reflect.Kind
		ty   Type
	}{
		{reflect.Invalid, Invalid},
		{reflect.Bool, Bool},
		{reflect.Int, Int},
		{reflect.Int8, Int8},
		{reflect.Int16, Int16},
		{reflect.Int32, Int32},
		{reflect.Int64, Int64},
		{reflect.Uint, Uint},
		{reflect.Uint8, Uint8},
		{reflect.Uint16, Uint16},
		{reflect.Uint32, Uint32},
		{reflect.Uint64, Uint64},
		{reflect.Uintptr, Uintptr},
		{reflect.Float32, Float32},
		{reflect.Float64, Float64},
		{reflect.Complex64, Complex64},
		{reflect.Complex128, Complex128},
		{reflect.Array, Array},
		{reflect.Chan, Chan},
		{reflect.Func, Func},
		{reflect.Interface, Interface},
		{reflect.Map, Map},
		{reflect.Pointer, Pointer},
		{reflect.Slice, Slice},
		{reflect.String, String},
		{reflect.Struct, Struct},
		{reflect.UnsafePointer, UnsafePointer},
	}
	for _, p := range pairs {
		assert.Equal(t, int(p.kind), int(p.ty), "kind %s drifted from Type", p.kind)
	}
}

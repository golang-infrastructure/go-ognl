package ognl

// Regression tests for the production-hardening pass. Each test pins a
// previously-confirmed defect (P0-*/P1-*) to its fixed behavior, so a
// regression re-surfaces as a normal test failure rather than a crash. The
// TestCoverage_* tests exercise public API that previously had no coverage.

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	value := []map[string]interface{}{{"k": 1}, {"k": 2}, {"k": 3}}
	r := Get(value, "#")

	a := r.Get("k")
	assert.Equal(t, []interface{}{1, 2, 3}, a.Values())

	b := r.Get("k")
	assert.Equal(t, []interface{}{1, 2, 3}, b.Values())

	// a must be unchanged after b ran on the same Result.
	assert.Equal(t, []interface{}{1, 2, 3}, a.Values())
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

// P1-3: list[ln:] re-slicing kept the consumed inputs alive via the shared
// backing array; cloneTail severs that so they can be collected.
func TestP1_3_NoResliceRetention(t *testing.T) {
	var collected int64

	tail := func() Result {
		const n = 50
		items := make([]*[4096]byte, n)
		for i := range items {
			items[i] = &[4096]byte{}
			runtime.SetFinalizer(items[i], func(*[4096]byte) { atomic.AddInt64(&collected, 1) })
		}
		r := Get(items, "#")        // deploy -> raw holds all n pointers
		return r.Get("NonExistent") // empty result; must not retain items
	}()
	_ = tail

	for i := 0; i < 50 && atomic.LoadInt64(&collected) < 50; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, int64(50), atomic.LoadInt64(&collected), "consumed inputs must be collectable")
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

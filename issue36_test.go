package ognl

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue36Private1 struct {
	f00 int
}

type issue36Private8 struct {
	f00, f01, f02, f03, f04, f05, f06, f07 int
}

type issue36Private32 struct {
	f00, f01, f02, f03, f04, f05, f06, f07 int
	f08, f09, f10, f11, f12, f13, f14, f15 int
	f16, f17, f18, f19, f20, f21, f22, f23 int
	f24, f25, f26, f27, f28, f29, f30, f31 int
}

type issue36Exported1 struct {
	F00 int
}

type issue36Exported32 struct {
	F00, F01, F02, F03, F04, F05, F06, F07 int
	F08, F09, F10, F11, F12, F13, F14, F15 int
	F16, F17, F18, F19, F20, F21, F22, F23 int
	F24, F25, F26, F27, F28, F29, F30, F31 int
}

type issue36MixedFields struct {
	Exported string
	private  int
	Nil      *int
	Tail     uint
}

type issue36SelectorRoot struct {
	Level issue36SelectorLevel
}

type issue36SelectorLevel struct {
	Branch issue36SelectorBranch
}

type issue36SelectorBranch struct {
	Leaf    issue36SelectorLeaf
	Payload issue36Private32
}

type issue36SelectorLeaf struct {
	Name string
}

var (
	issue36ResultSink []interface{}
	issue36TypeSink   Type
	issue36ErrorSink  error
	issue36GetSink    Result
)

var issue36Private8Value = issue36Private8{
	f00: 0, f01: 1, f02: 2, f03: 3, f04: 4, f05: 5, f06: 6, f07: 7,
}

var issue36Private32Value = issue36Private32{
	f00: 0, f01: 1, f02: 2, f03: 3, f04: 4, f05: 5, f06: 6, f07: 7,
	f08: 8, f09: 9, f10: 10, f11: 11, f12: 12, f13: 13, f14: 14, f15: 15,
	f16: 16, f17: 17, f18: 18, f19: 19, f20: 20, f21: 21, f22: 22, f23: 23,
	f24: 24, f25: 25, f26: 26, f27: 27, f28: 28, f29: 29, f30: 30, f31: 31,
}

var issue36SelectorValue = issue36SelectorRoot{
	Level: issue36SelectorLevel{
		Branch: issue36SelectorBranch{
			Leaf:    issue36SelectorLeaf{Name: "leaf"},
			Payload: issue36Private32Value,
		},
	},
}

func TestIssue36DeploymentPreservesStructValuesAndOrder(t *testing.T) {
	value := issue36MixedFields{
		Exported: "public",
		private:  7,
		Nil:      nil,
		Tail:     9,
	}
	want := []interface{}{"public", 7, (*int)(nil), uint(9)}

	for _, tt := range []struct {
		name  string
		value interface{}
	}{
		{name: "value", value: value},
		{name: "pointer", value: &value},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, typ, err := deployment(reflect.TypeOf(tt.value), reflect.ValueOf(tt.value), 0)
			require.NoError(t, err)
			assert.Equal(t, Interface, typ)
			assert.Equal(t, want, got)
		})
	}
}

func TestIssue36DeploymentPreservesPrivateAndExportedFields(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  []interface{}
	}{
		{name: "private value", value: issue36Private1{f00: 11}, want: []interface{}{11}},
		{name: "private pointer", value: &issue36Private1{f00: 12}, want: []interface{}{12}},
		{name: "exported value", value: issue36Exported1{F00: 13}, want: []interface{}{13}},
		{name: "exported pointer", value: &issue36Exported1{F00: 14}, want: []interface{}{14}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, typ, err := deployment(reflect.TypeOf(tt.value), reflect.ValueOf(tt.value), 0)
			require.NoError(t, err)
			assert.Equal(t, Interface, typ)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssue36DeploymentPreservesContainerOrderAndIdentity(t *testing.T) {
	value := 42
	pointer := &value

	tests := []struct {
		name  string
		value interface{}
		want  []interface{}
	}{
		{name: "slice", value: []*int{pointer, nil}, want: []interface{}{pointer, (*int)(nil)}},
		{name: "array", value: [3]int{3, 1, 2}, want: []interface{}{3, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := deployment(reflect.TypeOf(tt.value), reflect.ValueOf(tt.value), 0)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssue36DeploymentPreservesNilAndEmptyResults(t *testing.T) {
	var nilPointer *issue36Private1
	got, typ, err := deployment(reflect.TypeOf(nilPointer), reflect.ValueOf(nilPointer), 0)
	require.NoError(t, err)
	assert.Nil(t, got)
	assert.Equal(t, Invalid, typ)

	for _, tt := range []struct {
		name  string
		value interface{}
	}{
		{name: "nil map", value: map[string]int(nil)},
		{name: "empty map", value: map[string]int{}},
		{name: "nil slice", value: []int(nil)},
		{name: "empty slice", value: []int{}},
		{name: "empty array", value: [0]int{}},
		{name: "empty struct", value: struct{}{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, typ, err := deployment(reflect.TypeOf(tt.value), reflect.ValueOf(tt.value), 0)
			require.NoError(t, err)
			assert.Nil(t, got)
			assert.Equal(t, Interface, typ)
		})
	}
}

func TestIssue36DeploymentAllocationBudgets(t *testing.T) {
	map1 := map[int]*int{0: new(int)}
	map64 := make(map[int]*int, 64)
	for i := 0; i < 64; i++ {
		map64[i] = new(int)
	}

	private1 := issue36DeploymentAllocs(issue36Private1{})
	private32 := issue36DeploymentAllocs(issue36Private32Value)
	exported0 := issue36DeploymentAllocs(struct{}{})
	exported1 := issue36DeploymentAllocs(issue36Exported1{})
	exported32 := issue36DeploymentAllocs(issue36Exported32{})
	mapEmpty := issue36DeploymentAllocs(map[int]*int{})
	mapOne := issue36DeploymentAllocs(map1)
	mapMany := issue36DeploymentAllocs(map64)
	slice1 := issue36DeploymentAllocs(make([]interface{}, 1))
	slice64 := issue36DeploymentAllocs(make([]interface{}, 64))
	array1 := issue36DeploymentAllocs([1]interface{}{})
	array64 := issue36DeploymentAllocs([64]interface{}{})

	t.Logf("allocs/run private1=%.0f private32=%.0f exported0=%.0f exported1=%.0f exported32=%.0f map0=%.0f map1=%.0f map64=%.0f slice1=%.0f slice64=%.0f array1=%.0f array64=%.0f",
		private1, private32, exported0, exported1, exported32, mapEmpty, mapOne, mapMany, slice1, slice64, array1, array64)

	// checkptr adds per-field Interface allocations. Same-run exported and
	// size-0/size-1 controls normalize that instrumentation while leaving the
	// repeated struct copies and result-slice growth allocations observable.
	exported32Budget := 1 + 32*(exported1-exported0-1) + exported0
	map64Budget := 1 + 64*(mapOne-mapEmpty-1) + mapEmpty

	assert.LessOrEqual(t, private32, exported32+36, "private fields must share one addressable struct copy")
	assert.LessOrEqual(t, exported32, exported32Budget+1, "struct results must be preallocated")
	assert.LessOrEqual(t, mapMany, map64Budget+1, "map results must be preallocated")
	assert.LessOrEqual(t, slice64, slice1+1, "slice results must be preallocated")
	assert.LessOrEqual(t, array64, array1+1, "array results must be preallocated")
}

func issue36DeploymentAllocs(value interface{}) float64 {
	typ := reflect.TypeOf(value)
	rv := reflect.ValueOf(value)
	return testing.AllocsPerRun(200, func() {
		issue36ResultSink, issue36TypeSink, issue36ErrorSink = deployment(typ, rv, 0)
	})
}

func BenchmarkIssue36Deployment(b *testing.B) {
	benchmarks := []struct {
		name  string
		value interface{}
	}{
		{name: "private1", value: issue36Private1{}},
		{name: "private8", value: issue36Private8Value},
		{name: "private32", value: issue36Private32Value},
		{name: "exported32", value: issue36Exported32{}},
		{name: "slice64", value: make([]int, 64)},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			typ := reflect.TypeOf(benchmark.value)
			rv := reflect.ValueOf(benchmark.value)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				issue36ResultSink, issue36TypeSink, issue36ErrorSink = deployment(typ, rv, 0)
			}
		})
	}
}

func BenchmarkIssue36GetSelectors(b *testing.B) {
	benchmarks := []struct {
		name      string
		path      string
		wantValue interface{}
		wantLen   int
	}{
		{name: "deep", path: "Level.Branch.Leaf.Name", wantValue: "leaf"},
		{name: "expandPrivateStruct", path: "Level.Branch.Payload#", wantLen: 32},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			got := Get(issue36SelectorValue, benchmark.path)
			if !got.Effective() {
				b.Fatalf("selector %q did not resolve", benchmark.path)
			}
			if benchmark.wantLen > 0 {
				values, ok := got.Value().([]interface{})
				if !ok || len(values) != benchmark.wantLen {
					b.Fatalf("selector %q result = %#v, want %d values", benchmark.path, got.Value(), benchmark.wantLen)
				}
			} else if got.Value() != benchmark.wantValue {
				b.Fatalf("selector %q result = %#v, want %#v", benchmark.path, got.Value(), benchmark.wantValue)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				issue36GetSink = Get(issue36SelectorValue, benchmark.path)
			}
		})
	}
}

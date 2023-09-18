package ognl

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

type Mock struct {
	Name   string
	Age    int
	Hash1  map[string]interface{}
	Hash2  map[int]interface{}
	List   []*Mock
	Array  [3]*Mock
	lName  string
	lAge   int
	lHash1 map[string]interface{}
	lHash2 map[int]interface{}
	lList  []*Mock
	lArray [3]*Mock
}

func TestGet(t *testing.T) {
	var (
		t2 = &Mock{
			Name: "t2",
			Age:  2,
		}
		hash1 = map[string]interface{}{
			"string1": "string",
			"int1":    1,
			"t2":      t2,
		}
		t3 = &Mock{
			Name: "t3",
			Age:  3,
		}
		t4 = &Mock{
			Name:  "t4",
			Age:   4,
			Hash1: map[string]interface{}{},
		}
		hash2 = map[int]interface{}{
			2: t2,
			3: t3,
			4: t4,
		}
		list  = []*Mock{t2, t3, t4}
		array = [3]*Mock{t2, t3, t4}
		t1    = &Mock{
			Name:   "t1",
			lName:  "lt1",
			Age:    1,
			lAge:   11,
			Hash1:  hash1,
			lHash1: hash1,
			Hash2:  hash2,
			lHash2: hash2,
			List:   list,
			lList:  list,
			Array:  array,
			lArray: array,
		}
	)
	hash1["t1"] = t1
	test := []struct {
		query     string
		value     interface{}
		effective bool
	}{
		{"Name", "t1", true},
		{"Age", 1, true},
		{"Hash1.string1", "string", true},
		{"Hash1.int1", 1, true},
		{"Hash1.t2.Name", "t2", true},
		{"Hash2.2.Name", "t2", true},
		{"List.1.Name", "t3", true},
		{"List.0.Name", "t2", true},
		{"Array.0.Name", "t2", true},
		{"hash2.0", nil, false},
		{"Hash1.t1.Hash1.t1.Hash1.t1.Name", "t1", true},
		{"lName", "lt1", true},
		{"lAge", 11, true},
		{"lHash1.string1", "string", true},
		{"lHash1.int1", 1, true},
		{"lHash1.t2.Name", "t2", true},
		{"lHash2.2.Name", "t2", true},
		{"lList.1.Name", "t3", true},
		{"lList.0.Name", "t2", true},
		{"lArray.0.Name", "t2", true},
		{"lhash2.0", nil, false},
		{"lHash1.t1.Hash1.t1.lHash1.t1.Name", "t1", true},

		{"#", []interface{}{"t1", 1, hash1, hash2, list, array, "lt1", 11, hash1, hash2, list, array}, true},
		{"#string1#", []interface{}{"string", "string"}, true},
		{"List#1.Name", []interface{}{"t3", "t3", "t3", "t3"}, true},
	}
	for index, v := range test {
		vv := Get(t1, v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t, index:%d", v.effective, vv.Effective(), index)
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	for index, v := range test {
		vv := Get(*t1, v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t, index:%d", v.effective, vv.Effective(), index)
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	p := Parse(t1)
	for index, v := range test {
		vv := p.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	p = Parse(*t1)
	for index, v := range test {
		vv := p.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	test = []struct {
		query     string
		value     interface{}
		effective bool
	}{
		{"0", t2, true},
		{"1", t3, true},
		{"2", t4, true},
		{"3", nil, false},
		{"0.Name", "t2", true},
		{"0.Age", 2, true},
		{"1.Name", "t3", true},
		{"1.Age", 3, true},
		{"2.Name", "t4", true},
		{"2.Age", 4, true},
		{"#", []interface{}{t2, t3, t4}, true},
	}
	g1 := Get(t1, "List")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	g1 = Get(*t1, "List")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	test = []struct {
		query     string
		value     interface{}
		effective bool
	}{
		{"List", []interface{}{t2, t3, t4}, true},
		{"Array", []interface{}{t2, t3, t4}, true},
	}
	g1 = Get(t1, "")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Values()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	g1 = Get(*t1, "")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Values()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}
}

type Repetition struct {
	Repetition string
}

type RepetitionDetail struct {
	*Repetition
}

func TestRepetition(t *testing.T) {

	r := RepetitionDetail{
		Repetition: &Repetition{
			Repetition: "r1",
		},
	}

	vv := Get(r, "Repetition").Value()
	assert.Equal(t, vv, "r1")
}

type repetition struct {
	repetition string
}

type repetitionDetail struct {
	*repetition
}

func TestRepetition2(t *testing.T) {

	r := repetitionDetail{
		repetition: &repetition{
			repetition: "r2",
		},
	}

	vv := Get(r, "repetition").Value()
	assert.Equal(t, vv, "r2")
}

// TestBaseGet 测试基础类型
func TestBaseType(t *testing.T) {
	p := 1
	mock := struct {
		Bool_t          bool
		bool_t          bool
		Int_t           int
		int_t           int
		Int8_t          int8
		int8_t          int8
		Int16_t         int16
		int16_t         int16
		Int32_t         int32
		int32_t         int32
		Int64_t         int64
		int64_t         int64
		Uint_t          uint
		uint_t          uint
		Uint8_t         uint8
		uint8_t         uint8
		Uint16_t        uint16
		uint16_t        uint16
		Uint32_t        uint32
		uint32_t        uint32
		Uint64_t        uint64
		uint64_t        uint64
		Uintptr_t       uintptr
		uintptr_t       uintptr
		Float32_t       float32
		float32_t       float32
		Float64_t       float64
		float64_t       float64
		Complex64_t     complex64
		complex64_t     complex64
		Complex128_t    complex128
		complex128_t    complex128
		Array_t         [1]int
		array_t         [1]int
		Chan_t          chan string
		chan_t          chan string
		Func_t          func()
		func_t          func()
		Interface_t     interface{}
		interface_t     interface{}
		Map_t           map[string]interface{}
		map_t           map[string]interface{}
		Pointer_t       *int
		pointer_t       *int
		Slice_t         []interface{}
		slice_t         []interface{}
		String_t        string
		string_t        string
		Struct_t        struct{}
		struct_t        struct{}
		UnsafePointer_t unsafe.Pointer
		unsafePointer_t unsafe.Pointer
		Byte_t          byte
		byte_t          byte
		Rune_t          rune
		rune_t          rune
	}{
		Bool_t:          false,
		bool_t:          true,
		Int_t:           13,
		int_t:           14,
		Int8_t:          15,
		int8_t:          16,
		Int16_t:         17,
		int16_t:         18,
		Int32_t:         19,
		int32_t:         20,
		Int64_t:         21,
		int64_t:         22,
		Uint_t:          3,
		uint_t:          4,
		Uint8_t:         5,
		uint8_t:         6,
		Uint16_t:        7,
		uint16_t:        8,
		Uint32_t:        9,
		uint32_t:        10,
		Uint64_t:        11,
		uint64_t:        12,
		Uintptr_t:       31,
		uintptr_t:       32,
		Float32_t:       23,
		float32_t:       23.1,
		Float64_t:       24,
		float64_t:       24.1,
		Complex64_t:     25,
		complex64_t:     25.1,
		Complex128_t:    26,
		complex128_t:    26.1,
		Array_t:         [1]int{1},
		array_t:         [1]int{1},
		Chan_t:          make(chan string, 5),
		chan_t:          make(chan string, 5),
		Func_t:          func() { fmt.Println("666") },
		func_t:          func() { fmt.Println("666") },
		Interface_t:     `1`,
		interface_t:     `2`,
		Map_t:           map[string]interface{}{"1": 1},
		map_t:           map[string]interface{}{"1": 1},
		Pointer_t:       nil,
		pointer_t:       nil,
		Slice_t:         []interface{}{1, "2", "3"},
		slice_t:         []interface{}{1, "2", "3"},
		String_t:        "1",
		string_t:        "2",
		Struct_t:        struct{}{},
		struct_t:        struct{}{},
		UnsafePointer_t: unsafe.Pointer(&p),
		unsafePointer_t: unsafe.Pointer(&p),
		Byte_t:          byte(27),
		byte_t:          byte(28),
		Rune_t:          rune(29),
		rune_t:          rune(30),
	}

	mokeValue := reflect.ValueOf(mock)
	mokeType := reflect.TypeOf(mock)
	mokeField := mokeValue.NumField()

	for i := 0; i < mokeField; i++ {
		structField := mokeType.Field(i)

		cp := reflect.New(mokeValue.Type()).Elem()
		cp.Set(mokeValue)
		rv := cp.FieldByName(structField.Name)
		res := reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Interface()

		// 获取基本数据value
		value := Get(mock, structField.Name).Value()
		if rv.Kind() == reflect.Func {
			v1 := reflect.ValueOf(res)
			v2 := reflect.ValueOf(value)
			if !(v1.Pointer() == v2.Pointer()) {
				t.Fatalf("%s: incorrect response; want: %d, got: %d", structField.Name, v1.Pointer(), v2.Pointer())
			}
		} else {
			assert.Equal(t, res, value)
		}

		// 获取基本数据Type
		Get(mock, structField.Name).Type()
		tp := Get(mock, structField.Name).Type()
		assert.Equal(t, int(rv.Kind()), int(tp))
	}
}

type foo struct {
	Bar *bar
}

type bar struct {
	Name string
}

// 测试选择器转义字符
func TestEscape(t *testing.T) {
	m := make(map[string]any)
	t1 := &foo{Bar: &bar{Name: "bar name 001"}}

	m["Foo.Bar.Name"] = t1
	assert.Equal(t, t1, Get(m, "Foo\\.Bar\\.Name").Value())

	m["Foo"] = &foo{Bar: &bar{Name: "bar name 002"}}
	assert.Equal(t, "bar name 002", Get(m, "Foo.Bar.Name").Value())

	m[".Foo"] = "bar name 003"
	assert.Equal(t, "bar name 003", Get(m, "\\.Foo").Value())

	m["Foo."] = "bar name 004"
	assert.Equal(t, "bar name 004", Get(m, "Foo\\.").Value())

	m["Foo.....Bar"] = "bar name 005"
	assert.Equal(t, "bar name 005", Get(m, "Foo\\.\\.\\.\\.\\.Bar").Value())

	m["Foo.....Bar"] = "bar name 005"
	assert.Equal(t, nil, Get(m, "Foo\\.\\.\\.\\.\\.Bar.1").Value())
}

func TestDeep(t *testing.T) {
	type Test struct {
		First  string
		Middle *Test
		last   int
	}
	t1 := &Test{
		First: "first",
		last:  7,
	}
	t1.Middle = t1
	t.Log(Get(t1, "First").Value())         // first
	t.Log(Get(t1, "last").Value())          // 7
	t.Log(Get(t1, "Middle.Middle").Value()) // 7
	t.Log(Get(t1, "#").Value())             // []interface{}{"first",t1,7}
	t.Log(Get(t1, "##").Value())            // []interface{}{"first","first",t1,7,7}
	t.Log(Get(t1, "##").Values())           // []interface{}{"first","first",t1,7,7}
}

func Test_parseLastKeyIndex(t *testing.T) {
	type args struct {
		selector string
	}
	tests := []struct {
		name    string
		args    args
		wantVal string
	}{
		{
			name: "",
			args: args{
				selector: ".name",
			},
			wantVal: "",
		},
		{
			name: "",
			args: args{
				selector: "First",
			},
			wantVal: "",
		},
		{
			name: "",
			args: args{
				selector: "#",
			},
			wantVal: "",
		},
		{
			name: "",
			args: args{
				selector: "##",
			},
			wantVal: "#",
		},
		{
			name: "",
			args: args{
				selector: "###",
			},
			wantVal: "##",
		},
		{
			name: "",
			args: args{
				selector: "Middle.Middle",
			},
			wantVal: "Middle",
		},
		{
			name: "",
			args: args{
				selector: "Middle.Middle#",
			},
			wantVal: "Middle.Middle",
		},
		{
			name: "",
			args: args{
				selector: "Foo\\.Bar\\.Name",
			},
			wantVal: "",
		},
		{
			name: "",
			args: args{
				selector: "Foo\\.\\.\\.\\.\\.Bar",
			},
			wantVal: "",
		},
		{
			name: "",
			args: args{
				selector: "Foo\\.\\.\\.\\.\\.Bar.1",
			},
			wantVal: "Foo\\.\\.\\.\\.\\.Bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := parseLastKeyIndex(tt.args.selector)
			assert.Equalf(t, tt.wantVal, tt.args.selector[:idx], "parseLastKeyIndex(%v)", tt.args.selector[:idx])
		})
	}
}

func TestSet(t *testing.T) {
	var (
		t2 = &Mock{
			Name: "t2",
			Age:  2,
		}
		hash1 = map[string]interface{}{
			"string1":      "string",
			"int1":         1,
			"t2":           t2,
			"Foo.Bar.Name": "bar name 001",
		}
		t3 = &Mock{
			Name: "t3",
			Age:  3,
		}
		t4 = &Mock{
			Name:  "t4",
			Age:   4,
			Hash1: map[string]interface{}{},
		}
		hash2 = map[int]interface{}{
			2: t2,
			3: t3,
			4: t4,
		}
		list  = []*Mock{t2, t3, t4}
		array = [3]*Mock{t2, t3, t4}
		t1    = &Mock{
			Name:   "t1",
			lName:  "lt1",
			Age:    1,
			lAge:   11,
			Hash1:  hash1,
			lHash1: hash1,
			Hash2:  hash2,
			lHash2: hash2,
			List:   list,
			lList:  list,
			Array:  array,
			lArray: array,
		}
	)
	hash1["t1"] = t1

	type args struct {
		path  string
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "",
			args: args{
				path:  "Name",
				value: "t1change",
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path:  "Age",
				value: uint(10),
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path: "Hash1",
				value: map[string]interface{}{
					"key": "value",
				},
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path: "Array",
				value: [3]*Mock{
					t1,
				},
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path: "List",
				value: []*Mock{
					t3,
				},
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path:  "List.0.Name",
				value: "hhh",
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path:  "Array.0.Name",
				value: "hhh",
			},
			wantErr: nil,
		},
		{
			name: "",
			args: args{
				path:  "Hash1.Foo\\.Bar\\.Name",
				value: "bar name 002",
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Set(t1, tt.args.path, tt.args.value)
			if tt.wantErr != nil {
				tt.wantErr(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

package ognl

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"gotest.tools/assert"
)

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
			assert.DeepEqual(t, res, value)
		}

		// 获取基本数据Type
		Get(mock, structField.Name).Type()
		tp := Get(mock, structField.Name).Type()
		assert.DeepEqual(t, int(rv.Kind()), int(tp))
	}
}

package ognl

import (
	"testing"

	"gotest.tools/assert"
)

// TestBaseGet 测试基础类型
func TestBaseType(t *testing.T) {
	mock := struct {
		String_t     string
		string_t     string
		Uint_t       uint
		uint_t       uint
		Uint8_t      uint8
		uint8_t      uint8
		Uint16_t     uint16
		uint16_t     uint16
		Uint32_t     uint32
		uint32_t     uint32
		Uint64_t     uint64
		uint64_t     uint64
		Int_t        int
		int_t        int
		Int8_t       int8
		int8_t       int8
		Int16_t      int16
		int16_t      int16
		Int32_t      int32
		int32_t      int32
		Int64_t      int64
		int64_t      int64
		Float32_t    float32
		float32_t    float32
		Float64_t    float64
		float64_t    float64
		Complex64_t  complex64
		complex64_t  complex64
		Complex128_t complex128
		complex128_t complex128
		Byte_t       byte
		byte_t       byte
		Rune_t       rune
		rune_t       rune
		Uintptr_t    uintptr
		uintptr_t    uintptr
	}{
		String_t:     "1",
		string_t:     "2",
		Uint_t:       3,
		uint_t:       4,
		Uint8_t:      5,
		uint8_t:      6,
		Uint16_t:     7,
		uint16_t:     8,
		Uint32_t:     9,
		uint32_t:     10,
		Uint64_t:     11,
		uint64_t:     12,
		Int_t:        13,
		int_t:        14,
		Int8_t:       15,
		int8_t:       16,
		Int16_t:      17,
		int16_t:      18,
		Int32_t:      19,
		int32_t:      20,
		Int64_t:      21,
		int64_t:      22,
		Float32_t:    23,
		float32_t:    23.1,
		Float64_t:    24,
		float64_t:    24.1,
		Complex64_t:  25,
		complex64_t:  25.1,
		Complex128_t: 26,
		complex128_t: 26.1,
		Byte_t:       byte(27),
		byte_t:       byte(28),
		Rune_t:       rune(29),
		rune_t:       rune(30),
		Uintptr_t:    31,
		uintptr_t:    32,
	}

	// 获取基本数据value
	func() {
		// Value
		String_t := Get(mock, "String_t").Value()
		assert.Equal(t, String_t, mock.String_t)

		string_t := Get(mock, "string_t").Value()
		assert.Equal(t, string_t, mock.string_t)

		Uint_t := Get(mock, "Uint_t").Value()
		assert.Equal(t, Uint_t, mock.Uint_t)

		uint_t := Get(mock, "uint_t").Value()
		assert.Equal(t, uint_t, mock.uint_t)

		Uint8_t := Get(mock, "Uint8_t").Value()
		assert.Equal(t, Uint8_t, mock.Uint8_t)

		uint8_t := Get(mock, "uint8_t").Value()
		assert.Equal(t, uint8_t, mock.uint8_t)

		Uint16_t := Get(mock, "Uint16_t").Value()
		assert.Equal(t, Uint16_t, mock.Uint16_t)

		uint16_t := Get(mock, "uint16_t").Value()
		assert.Equal(t, uint16_t, mock.uint16_t)

		Uint32_t := Get(mock, "Uint32_t").Value()
		assert.Equal(t, Uint32_t, mock.Uint32_t)

		uint32_t := Get(mock, "uint32_t").Value()
		assert.Equal(t, uint32_t, mock.uint32_t)

		Uint64_t := Get(mock, "Uint64_t").Value()
		assert.Equal(t, Uint64_t, mock.Uint64_t)

		uint64_t := Get(mock, "uint64_t").Value()
		assert.Equal(t, uint64_t, mock.uint64_t)

		Int_t := Get(mock, "Int_t").Value()
		assert.Equal(t, Int_t, mock.Int_t)

		int_t := Get(mock, "int_t").Value()
		assert.Equal(t, int_t, mock.int_t)

		Int8_t := Get(mock, "Int8_t").Value()
		assert.Equal(t, Int8_t, mock.Int8_t)

		int8_t := Get(mock, "int8_t").Value()
		assert.Equal(t, int8_t, mock.int8_t)

		Int16_t := Get(mock, "Int16_t").Value()
		assert.Equal(t, Int16_t, mock.Int16_t)

		int16_t := Get(mock, "int16_t").Value()
		assert.Equal(t, int16_t, mock.int16_t)

		Int32_t := Get(mock, "Int32_t").Value()
		assert.Equal(t, Int32_t, mock.Int32_t)

		int32_t := Get(mock, "int32_t").Value()
		assert.Equal(t, int32_t, mock.int32_t)

		Int64_t := Get(mock, "Int64_t").Value()
		assert.Equal(t, Int64_t, mock.Int64_t)

		int64_t := Get(mock, "int64_t").Value()
		assert.Equal(t, int64_t, mock.int64_t)

		Float32_t := Get(mock, "Float32_t").Value()
		assert.Equal(t, Float32_t, mock.Float32_t)

		float32_t := Get(mock, "float32_t").Value()
		assert.Equal(t, float32_t, mock.float32_t)

		Float64_t := Get(mock, "Float64_t").Value()
		assert.Equal(t, Float64_t, mock.Float64_t)

		float64_t := Get(mock, "float64_t").Value()
		assert.Equal(t, float64_t, mock.float64_t)

		Complex64_t := Get(mock, "Complex64_t").Value()
		assert.Equal(t, Complex64_t, mock.Complex64_t)

		complex64_t := Get(mock, "complex64_t").Value()
		assert.Equal(t, complex64_t, mock.complex64_t)

		Complex128_t := Get(mock, "Complex128_t").Value()
		assert.Equal(t, Complex128_t, mock.Complex128_t)

		complex128_t := Get(mock, "complex128_t").Value()
		assert.Equal(t, complex128_t, mock.complex128_t)

		Byte_t := Get(mock, "Byte_t").Value()
		assert.Equal(t, Byte_t, mock.Byte_t)

		byte_t := Get(mock, "byte_t").Value()
		assert.Equal(t, byte_t, mock.byte_t)

		Rune_t := Get(mock, "Rune_t").Value()
		assert.Equal(t, Rune_t, mock.Rune_t)

		rune_t := Get(mock, "rune_t").Value()
		assert.Equal(t, rune_t, mock.rune_t)

		Uintptr_t := Get(mock, "Uintptr_t").Value()
		assert.Equal(t, Uintptr_t, mock.Uintptr_t)

		uintptr_t := Get(mock, "uintptr_t").Value()
		assert.Equal(t, uintptr_t, mock.uintptr_t)
	}()

	// 获取基本数据类型
	func() {
		// Value
		String_t := Get(mock, "String_t").Type()
		assert.Equal(t, String_t, String)

		string_t := Get(mock, "string_t").Type()
		assert.Equal(t, string_t, String)

		Uint_t := Get(mock, "Uint_t").Type()
		assert.Equal(t, Uint_t, Uint)

		uint_t := Get(mock, "uint_t").Type()
		assert.Equal(t, uint_t, Uint)

		Uint8_t := Get(mock, "Uint8_t").Type()
		assert.Equal(t, Uint8_t, Uint8)

		uint8_t := Get(mock, "uint8_t").Type()
		assert.Equal(t, uint8_t, Uint8)

		Uint16_t := Get(mock, "Uint16_t").Type()
		assert.Equal(t, Uint16_t, Uint16)

		uint16_t := Get(mock, "uint16_t").Type()
		assert.Equal(t, uint16_t, Uint16)

		Uint32_t := Get(mock, "Uint32_t").Type()
		assert.Equal(t, Uint32_t, Uint32)

		uint32_t := Get(mock, "uint32_t").Type()
		assert.Equal(t, uint32_t, Uint32)

		Uint64_t := Get(mock, "Uint64_t").Type()
		assert.Equal(t, Uint64_t, Uint64)

		uint64_t := Get(mock, "uint64_t").Type()
		assert.Equal(t, uint64_t, Uint64)

		Int_t := Get(mock, "Int_t").Type()
		assert.Equal(t, Int_t, Int)

		int_t := Get(mock, "int_t").Type()
		assert.Equal(t, int_t, Int)

		Int8_t := Get(mock, "Int8_t").Type()
		assert.Equal(t, Int8_t, Int8)

		int8_t := Get(mock, "int8_t").Type()
		assert.Equal(t, int8_t, Int8)

		Int16_t := Get(mock, "Int16_t").Type()
		assert.Equal(t, Int16_t, Int16)

		int16_t := Get(mock, "int16_t").Type()
		assert.Equal(t, int16_t, Int16)

		Int32_t := Get(mock, "Int32_t").Type()
		assert.Equal(t, Int32_t, Int32)

		int32_t := Get(mock, "int32_t").Type()
		assert.Equal(t, int32_t, Int32)

		Int64_t := Get(mock, "Int64_t").Type()
		assert.Equal(t, Int64_t, Int64)

		int64_t := Get(mock, "int64_t").Type()
		assert.Equal(t, int64_t, Int64)

		Float32_t := Get(mock, "Float32_t").Type()
		assert.Equal(t, Float32_t, Float32)

		float32_t := Get(mock, "float32_t").Type()
		assert.Equal(t, float32_t, Float32)

		Float64_t := Get(mock, "Float64_t").Type()
		assert.Equal(t, Float64_t, Float64)

		float64_t := Get(mock, "float64_t").Type()
		assert.Equal(t, float64_t, Float64)

		Complex64_t := Get(mock, "Complex64_t").Type()
		assert.Equal(t, Complex64_t, Complex64)

		complex64_t := Get(mock, "complex64_t").Type()
		assert.Equal(t, complex64_t, Complex64)

		Complex128_t := Get(mock, "Complex128_t").Type()
		assert.Equal(t, Complex128_t, Complex128)

		complex128_t := Get(mock, "complex128_t").Type()
		assert.Equal(t, complex128_t, Complex128)

		// Byte_t := Get(mock, "Byte_t").Type()
		// assert.Equal(t, Byte_t, mock.Byte_t)

		// byte_t := Get(mock, "byte_t").Type()
		// assert.Equal(t, byte_t, mock.byte_t)

		// Rune_t := Get(mock, "Rune_t").Type()
		// assert.Equal(t, Rune_t, mock.Rune_t)

		// rune_t := Get(mock, "rune_t").Type()
		// assert.Equal(t, rune_t, mock.rune_t)

		Uintptr_t := Get(mock, "Uintptr_t").Type()
		assert.Equal(t, Uintptr_t, UnsafePointer)

		uintptr_t := Get(mock, "uintptr_t").Type()
		assert.Equal(t, uintptr_t, UnsafePointer)
	}()
}

package ognl

import (
	"errors"
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

// Type is Result type
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

type Result struct {
	// 如果解析使用了 '#' 则 typ 固定为 interface{} raw 为[]interface{}
	typ Type

	// 是否展开为数组,如果是数组,Raw应该是[]interface{}
	raw interface{}

	// 判断是否展开
	deployment bool

	// 收集一些解析过程中的错误,不影响返回值
	diagnosis []error
}

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

func (r Result) Type() Type {
	return r.typ
}

func (r Result) Value() interface{} {
	return r.raw
}

func (r Result) Values() []interface{} {

	if r.deployment {
		if r.raw == nil {
			return nil
		}
		return r.raw.([]interface{})
	}
	return []interface{}{r.raw}
}

func (r Result) Diagnosis() []error {
	return r.diagnosis
}

func (r Result) Get(path string) Result {
	if r.deployment {
		nr := r
		list := nr.raw.([]interface{})
		ln := len(list)
		for i := 0; i < ln; i++ {
			next := Get(list[i], path)
			if next.typ != Invalid {
				list = append(list, next.raw)
			}
			nr.diagnosis = append(nr.diagnosis, next.diagnosis...)
		}
		nr.raw = list[ln:]
		return nr
	}
	return Get(r.raw, path)
}

func Parse(result interface{}) Result {
	return Get(result, "")
}

func Get(value interface{}, path string) Result {
	if value == nil {
		return Result{typ: Interface, raw: value}
	}

	var index int

	var result = Result{
		typ: Interface,
		raw: value,
	}
	tp := reflect.TypeOf(value)
	tv := reflect.ValueOf(value)

	if !tv.IsValid() {
		return result
	}

	for ; index < len(path); index++ {
		switch path[index] {
		case ' ', '\t', '\n', '\r', '.':
			switch result.deployment {
			case true:
				list := result.raw.([]interface{})
				ln := len(list)
				for i := 0; i < ln; i++ {
					next := Get(list[i], path[index+1:])
					if next.typ != Invalid {
						list = append(list, next.raw)
					}
					result.diagnosis = append(result.diagnosis, next.diagnosis...)
				}
				result.raw = list[ln:]
				return result
			default:
				return Get(result.raw, path[index+1:])
			}
		case '#':
			// 转化成slice 类型 平铺
			switch result.deployment {
			case true:
				// queue 展开
				list := result.raw.([]interface{})
				ln := len(list)
				for i := 0; i < ln; i++ {
					raw, typ, err := deployment(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]))
					if err != nil {
						result.diagnosis = append(result.diagnosis, err)
					}
					if typ != Invalid {
						list = append(list, raw...)
					}
				}
				result.raw = list[ln:]
			default:
				result.deployment = true
				var err error
				result.raw, result.typ, err = deployment(tp, tv)
				if err != nil {
					result.diagnosis = append(result.diagnosis, err)
				}
			}

		default:
			start := index
		loop:
			for ; index < len(path); index++ {
				switch path[index] {
				case '.', '#':
					index--
					break loop
				}
			}
			sv := path[start:min(index+1, len(path))]
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
				for i := 0; i < ln; i++ {
					if digit {
						value, tp, err = parseInt(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), v)
					} else {
						value, tp, err = parseString(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), sv)
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, err)
					}
					if tp != Invalid {
						list = append(list, value)
					}
				}
				result.raw = list[ln:]
			default:
				var (
					value interface{}
					tpe   Type
				)
				if digit {
					value, tpe, err = parseInt(tp, tv, v)
				} else {
					value, tpe, err = parseString(tp, tv, sv)
				}
				if err != nil {
					result.diagnosis = append(result.diagnosis, err)
				}
				result.raw = value
				result.typ = tpe
				if tpe == Invalid {
					return result
				}
			}
		}
	}

	return result
}

func GetMany(value interface{}, path ...string) []Result {
	results := make([]Result, 0, len(path))
	for _, s := range path {
		results = append(results, Get(value, s))
	}
	return results
}

func parseString(t reflect.Type, v reflect.Value, value string) (interface{}, Type, error) {
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
		return parseString(t, v.Elem(), value)

	case reflect.Map:
		// MUST map key is string
		if t.Key().Kind() != reflect.String {
			return nil, Invalid, ErrMapKeyMustString
		}

		value := v.MapIndex(reflect.ValueOf(value))
		if !value.IsValid() {
			return nil, Invalid, nil
		}

		return value.Interface(), Type(value.Kind()), nil

	case reflect.Slice, reflect.Array:
		return nil, Invalid, ErrSliceSubscript

	case reflect.Struct:
		v := v.FieldByName(value)
		if !v.IsValid() {
			return nil, Invalid, nil
		}

		if v.CanInterface() {
			return v.Interface(), Type(v.Kind()), nil
		}

		return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface(), Type(v.Kind()), nil

	default:
		return nil, Invalid, ErrInvalidStructure
	}
}

func parseInt(t reflect.Type, v reflect.Value, tokenValue int) (interface{}, Type, error) {
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
		return parseInt(t, v.Elem(), tokenValue)
	case reflect.Map:
		// MUST map key is int
		if t.Key().Kind() != reflect.Int {
			return nil, Invalid, ErrMapKeyMustInt
		}

		value := v.MapIndex(reflect.ValueOf(tokenValue))
		if !value.IsValid() {
			return nil, Invalid, nil
		}

		return value.Interface(), Type(value.Kind()), nil

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

		if v.CanInterface() {
			return value.Interface(), Type(value.Kind()), nil
		}
		return reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface(), Type(value.Kind()), nil

	default:
		return nil, Invalid, ErrParseInt
	}
}

func deployment(t reflect.Type, v reflect.Value) ([]interface{}, Type, error) {
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
		return deployment(t, v.Elem())

	case reflect.Map:
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
		var ret []interface{}

		for i := 0; i < v.NumField(); i++ {
			value := v.Field(i)
			if !value.IsValid() {
				return nil, Invalid, nil
			}

			if value.CanInterface() {
				ret = append(ret, value.Interface())
			} else {
				ret = append(ret, reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface())
			}
		}

		return ret, Interface, nil

	default:
		return []interface{}{v.Interface()}, Type(v.Kind()), nil
	}
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

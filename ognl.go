package ognl

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

var ErrInvalidSet = errors.New("invalid set")

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

	v, t, _ := deployment(reflect.TypeOf(r.raw), reflect.ValueOf(r.raw))
	if t == Invalid {
		return []interface{}{r.raw}
	}
	return v
}

func (r Result) ValuesE() ([]interface{}, error) {

	if r.deployment {
		if r.raw == nil {
			return nil, nil
		}
		return r.raw.([]interface{}), nil
	}

	v, t, err := deployment(reflect.TypeOf(r.raw), reflect.ValueOf(r.raw))
	if err != nil {
		return nil, warpError(err, r.raw, "")
	}
	if t == Invalid {
		return []interface{}{r.raw}, nil
	}
	return v, nil
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

func (r Result) GetE(path string) (Result, error) {
	if r.deployment {
		nr := r
		list := nr.raw.([]interface{})
		ln := len(list)
		for i := 0; i < ln; i++ {
			next, err := GetE(list[i], path)
			if err != nil {
				nr.diagnosis = append(nr.diagnosis, warpError(err, list[i], path))
			}
			if next.typ != Invalid {
				list = append(list, next.raw)
			}
			nr.diagnosis = append(nr.diagnosis, next.diagnosis...)
		}
		nr.raw = list[ln:]
		if len(list[ln:]) == 0 {
			return nr, warpError(ErrInvalidValue, list, path)
		}
		return nr, nil
	}
	return GetE(r.raw, path)
}

func Parse(result interface{}) Result {
	return Get(result, "")
}

func GetE(value interface{}, path string) (Result, error) {
	if value == nil && (len(path) == 0 || validIdentifier(path, 0)) {
		return Result{typ: Interface, raw: value}, nil
	}
	if value == nil {
		return Result{typ: Invalid, raw: value}, warpError(ErrInvalidValue, value, path)
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
		return result, warpError(ErrInvalidValue, value, path)
	}

	for ; index < len(path); index++ {
		switch path[index] {
		case ' ', '\t', '\n', '\r', '.':
			switch result.deployment {
			case true:
				list := result.raw.([]interface{})
				ln := len(list)
				for i := 0; i < ln; i++ {
					next, err := GetE(list[i], path[index+1:])
					if err != nil {
						result.diagnosis = append(result.diagnosis, warpError(err, list[i], path[index+1:]))
					}
					if next.typ != Invalid {
						list = append(list, next.raw)
					}

					result.diagnosis = append(result.diagnosis, next.diagnosis...)
				}
				result.raw = list[ln:]
				if len(list[ln:]) == 0 {
					return result, warpError(ErrInvalidValue, list, string(path[index]))
				}
				return result, nil
			default:
				return GetE(result.raw, path[index+1:])
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
						result.diagnosis = append(result.diagnosis, warpError(err, list[i], "#"))
					}
					if typ != Invalid {
						list = append(list, raw...)
					}
				}
				result.raw = list[ln:]
				if len(list[ln:]) == 0 {
					return result, warpError(ErrInvalidValue, list, "#")
				}
			default:
				result.deployment = true
				var err error
				result.raw, result.typ, err = deployment(tp, tv)
				if err != nil {
					return result, warpError(err, value, "#")
				}
				result.diagnosis = append(result.diagnosis, err)
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
				for i := 0; i < ln; i++ {
					if digit {
						value, tp, err = parseInt(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), v)
					} else {
						value, tp, err = parseString(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), sv)
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, warpError(err, list[i], sv))
					}
					if tp != Invalid {
						list = append(list, value)
					}
				}
				result.raw = list[ln:]
				if len(list[ln:]) == 0 {
					return result, warpError(ErrInvalidValue, list, sv)
				}

			default:
				var (
					nv  interface{}
					tpe Type
				)
				if digit {
					nv, tpe, err = parseInt(tp, tv, v)
				} else {
					nv, tpe, err = parseString(tp, tv, sv)
				}
				if err != nil {
					return result, warpError(err, value, sv)
				}
				if tpe == Invalid {
					return result, warpError(ErrInvalidValue, value, sv)
				}
				result.raw = nv
				result.typ = tpe
			}
		}
	}

	return result, nil
}

func Get(value interface{}, path string) Result {
	if value == nil && (len(path) == 0 || validIdentifier(path, 0)) {
		return Result{typ: Interface, raw: value}
	}
	if value == nil {
		return Result{typ: Invalid, raw: value, diagnosis: []error{warpError(ErrInvalidValue, value, path)}}
	}

	var index int

	var result = Result{
		typ: Interface,
		raw: value,
	}
	tp := reflect.TypeOf(value)
	tv := reflect.ValueOf(value)

	if !tv.IsValid() {
		result.diagnosis = append(result.diagnosis, warpError(ErrInvalidValue, value, path))
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
						result.diagnosis = append(result.diagnosis, warpError(err, list[i], "#"))
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
					result.diagnosis = append(result.diagnosis, warpError(err, value, "#"))
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
				for i := 0; i < ln; i++ {
					if digit {
						value, tp, err = parseInt(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), v)
					} else {
						value, tp, err = parseString(reflect.TypeOf(list[i]), reflect.ValueOf(list[i]), sv)
					}
					if err != nil {
						result.diagnosis = append(result.diagnosis, warpError(err, list[i], sv))
					}
					if tp != Invalid {
						list = append(list, value)
					}
				}
				result.raw = list[ln:]
			default:
				var (
					nv  interface{}
					tpe Type
				)
				if digit {
					nv, tpe, err = parseInt(tp, tv, v)
				} else {
					nv, tpe, err = parseString(tp, tv, sv)
				}
				if err != nil {
					result.diagnosis = append(result.diagnosis, warpError(err, value, sv))
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

func parseLastKeyIndex(selector string) int {
	idx := len(selector) - 1
	for ; idx > 0; idx-- {
		switch selector[idx] {
		case ' ', '\t', '\n', '\r', '.', '#':
			if idx-1 >= 0 && selector[idx-1] == '\\' {
				continue
			} else {
				return idx
			}
		}
	}
	return idx
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
		return parseString(t.Elem(), v.Elem(), value)

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
		rv := v.FieldByName(value)
		if !rv.IsValid() {
			return nil, Invalid, nil
		}

		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		rv = cp.FieldByName(value)

		res := reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Interface()

		rt, _ := t.FieldByName(value)

		if rt.Anonymous {
			nv, nt, ne := parseString(reflect.TypeOf(res), reflect.ValueOf(res), value)
			if ne == nil && nt != Invalid {
				return nv, nt, ne
			}
		}

		return res, Type(rv.Kind()), nil

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
		return parseInt(t.Elem(), v.Elem(), tokenValue)
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

		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		value = cp.Field(tokenValue)

		res := reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface()

		rt := t.Field(tokenValue)

		if rt.Anonymous {
			nv, nt, ne := parseInt(reflect.TypeOf(res), reflect.ValueOf(res), tokenValue)
			if ne == nil && nt != Invalid {
				return nv, nt, ne
			}
		}

		return res, Type(value.Kind()), nil

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
				cp := reflect.New(v.Type()).Elem()
				cp.Set(v)
				value = cp.Field(i)

				ret = append(ret, reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface())
			}
		}

		return ret, Interface, nil

	default:
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

func warpError(err error, object interface{}, path string) error {
	return fmt.Errorf("object:%v,path:%s,err: %w", object, path, err)
}

func Set(obj interface{}, path string, value interface{}) error {
	idx := parseLastKeyIndex(path)
	parentPath := path[:idx]
	offset := 1
	if idx == 0 && len(path) > 0 && (path[0] != '.' && path[0] != '#' && path[0] != ' ') {
		offset = 0
	}
	key := strings.ReplaceAll(path[idx+offset:], "\\", "")

	if key == "" {
		return fmt.Errorf("path:%s target path is empty", parentPath)
	}
	result, err := GetE(obj, parentPath)
	if err != nil {
		return err
	}

	if !result.Effective() {
		return fmt.Errorf("path:%s, invalid parent obj", parentPath)
	}

	if result.deployment {
		list := result.raw.([]interface{})
		ln := len(list)
		for i := 0; i < ln; i++ {
			err = set(list[i], key, value)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return set(result.raw, key, value)
}

func set(obj interface{}, key string, value interface{}) error {

	if IsNil(obj) {
		return ErrInvalidValue
	}

	v, err := strconv.Atoi(key)
	digit := err == nil && v >= 0

	t, f := reflect.TypeOf(obj), reflect.ValueOf(obj)
	if digit {
		return setInt(t, f, v, value)
	}
	return setString(t, f, key, value)
}

func setString(t reflect.Type, v reflect.Value, key string, value interface{}) error {
	if !v.IsValid() {
		return ErrInvalidValue
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return ErrInvalidValue
		}
		if !v.Elem().IsValid() {
			return ErrInvalidValue
		}
		return setString(t.Elem(), v.Elem(), key, value)

	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return ErrMapKeyMustString
		}

		newValue := reflect.ValueOf(value)
		if t.Elem() == newValue.Type() {
			v.SetMapIndex(reflect.ValueOf(key), newValue)
			return nil
		} else if newValue.Type().ConvertibleTo(t.Elem()) {
			v.SetMapIndex(reflect.ValueOf(key), newValue.Convert(t.Elem()))
			return nil
		} else {
			return fmt.Errorf("type mismatch in map assignment: want %s, got %s", t.Elem().String(), newValue.Type().String())
		}

	case reflect.Struct:
		field := v.FieldByName(key)
		if !field.IsValid() {
			return ErrStructIndexOutOfBounds
		}
		if !field.CanSet() {
			return ErrInvalidSet
		}

		newValue := reflect.ValueOf(value)
		if newValue.Type() == field.Type() {
			field.Set(newValue)
			return nil
		} else if newValue.Type().ConvertibleTo(field.Type()) {
			field.Set(newValue.Convert(field.Type()))
			return nil
		} else {
			return fmt.Errorf("type mismatch in struct assignment: want %s, got %s", field.Type().String(), newValue.Type().String())
		}

	default:
		return ErrInvalidStructure
	}
}

func setInt(t reflect.Type, v reflect.Value, key int, value interface{}) error {
	if !v.IsValid() {
		return ErrInvalidValue
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return ErrInvalidValue
		}
		if !v.Elem().IsValid() {
			return ErrInvalidValue
		}

		return setInt(t.Elem(), v.Elem(), key, value)
	case reflect.Map:
		if t.Key().Kind() != reflect.Int {
			return ErrMapKeyMustInt
		}

		if v.IsNil() {
			v.Set(reflect.MakeMap(t))
		}

		newValue := reflect.ValueOf(value)
		if newValue.Type() == t.Elem() {
			v.SetMapIndex(reflect.ValueOf(key), newValue)
			return nil
		} else if newValue.Type().ConvertibleTo(t.Elem()) {
			v.SetMapIndex(reflect.ValueOf(key), newValue.Convert(t.Elem()))
			return nil
		} else {
			return fmt.Errorf("type mismatch in map assignment: want %s, got %s", t.Elem().String(), newValue.Type().String())
		}

	case reflect.Slice, reflect.Array:
		if key < 0 || key >= v.Len() {
			return ErrIndexOutOfBounds
		}

		field := v.Index(key)
		if !field.IsValid() {
			return ErrInvalidSet
		}
		if !field.CanSet() {
			return ErrInvalidSet
		}

		newValue := reflect.ValueOf(value)
		if newValue.Type() == field.Type() {
			field.Set(newValue)
			return nil
		} else if newValue.Type().ConvertibleTo(field.Type()) {
			field.Set(newValue.Convert(field.Type()))
			return nil
		} else {
			return fmt.Errorf("type mismatch in slice assignment: want %s, got %s", field.Type().String(), newValue.Type().String())
		}

	case reflect.Struct:
		if key < 0 || key >= v.NumField() {
			return ErrStructIndexOutOfBounds
		}

		field := v.Field(key)
		if !field.IsValid() {
			return ErrInvalidSet
		}
		if !field.CanSet() {
			return ErrInvalidSet
		}

		newValue := reflect.ValueOf(value)
		if newValue.Type() == field.Type() {
			field.Set(newValue)
			return nil
		} else if newValue.Type().ConvertibleTo(field.Type()) {
			field.Set(newValue.Convert(field.Type()))
			return nil
		} else {
			return fmt.Errorf("type mismatch in struct assignment: want %s, got %s", field.Type().String(), newValue.Type().String())
		}

	default:
		return ErrInvalidStructure
	}
}

func IsNil(value interface{}) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Ptr:
		elem := v.Elem()
		if !elem.IsValid() {
			return true
		}
		return IsNil(elem.Interface())
	default:
		return false
	}
}

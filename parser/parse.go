package parser

import (
	"errors"
	"github.com/songzhibin97/go-ognl/expr"
	"github.com/songzhibin97/go-ognl/lexer"
	"github.com/songzhibin97/go-ognl/token"
	"reflect"
	"strconv"
)

var ErrInvalidToken = errors.New("invalid token")
var ErrInvalidStructure = errors.New("the structure cannot continue")
var ErrSliceSubscript = errors.New("invalid slice subscript")
var ErrMapKeyMustString = errors.New("map key must be string")
var ErrMapKeyMustInt = errors.New("map key must be int")
var ErrIndexOutOfBounds = errors.New("index out of bounds")
var ErrStructIndexOutOfBounds = errors.New("struct index out of bounds")
var ErrParseInt = errors.New("parse int error")
var ErrParseLen = errors.New("parse len error")

func Parse(query string, obj interface{}) (interface{}, error) {
	tokens, err := expr.ParseToken(lexer.NewLexer(query).GetToken())
	if err != nil {
		return nil, err
	}
	return parse(tokens, obj)
}

func parse(tokens []*token.Token, obj interface{}) (interface{}, error) {
	if len(tokens) == 0 || obj == nil {
		return obj, nil
	}
	t, v := reflect.TypeOf(obj), reflect.ValueOf(obj)

	var err error

	switch tokens[0].Type {
	case token.STRING:
		obj, err = parseString(t, v, tokens[0].Value, len(tokens))
	case token.INT:
		i, _ := strconv.Atoi(tokens[0].Value)
		obj, err = parseInt(t, v, i)
	case token.First:
		obj, err = parseInt(t, v, 0)
	case token.Last:
		l, err := parseLen(t, v)
		if err != nil {
			return nil, err
		}
		obj, err = parseInt(t, v, l.(int)-1)
	case token.Len:
		return parseLen(t, v)
	default:
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	return parse(tokens[1:], obj)
}

func parseString(t reflect.Type, v reflect.Value, tokenValue string, tokenLen int) (interface{}, error) {
	if !v.IsValid() {
		return nil, nil
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil, nil
		}

		if !v.Elem().IsValid() {
			return nil, nil
		}
		return parseString(t, v.Elem(), tokenValue, tokenLen)
	case reflect.Map:
		// MUST map key is string
		if t.Key().Kind() != reflect.String {
			return nil, ErrMapKeyMustString
		}
		return v.MapIndex(reflect.ValueOf(tokenValue)).Interface(), nil
	case reflect.Slice, reflect.Array:
		return nil, ErrSliceSubscript
	case reflect.Struct:
		v := v.FieldByName(tokenValue)
		if !v.IsValid() {
			return nil, nil
		}
		return v.Interface(), nil
	default:
		if tokenLen != 1 {
			return nil, ErrInvalidStructure
		}
		return v.Interface(), nil
	}
}

func parseInt(t reflect.Type, v reflect.Value, tokenValue int) (interface{}, error) {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil, nil
		}
		if !v.Elem().IsValid() {
			return nil, nil
		}
		return parseInt(t, v.Elem(), tokenValue)
	case reflect.Map:
		if t.Key().Kind() != reflect.Int {
			return nil, ErrMapKeyMustInt
		}
		// MUST map key is int
		return v.MapIndex(reflect.ValueOf(tokenValue)).Interface(), nil
	case reflect.Slice, reflect.Array:
		if tokenValue < 0 || tokenValue >= v.Len() {
			return nil, ErrIndexOutOfBounds
		}
		return v.Index(tokenValue).Interface(), nil
	case reflect.Struct:
		if tokenValue < 0 || tokenValue >= v.NumField() {
			return nil, ErrStructIndexOutOfBounds
		}
		return v.Field(tokenValue).Interface(), nil
	default:
		return nil, ErrParseInt
	}
}

func parseLen(t reflect.Type, v reflect.Value) (interface{}, error) {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return 0, nil
		}

		if !v.Elem().IsValid() {
			return 0, nil
		}
		return parseLen(t, v.Elem())
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len(), nil
	case reflect.Struct:
		return v.NumField(), nil
	default:
		return 0, ErrParseLen
	}
}

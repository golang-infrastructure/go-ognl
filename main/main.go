package main

import (
	"fmt"
	"reflect"

	"github.com/songzhibin97/go-ognl"
)

type Mock1 struct {
	name      string
	age       int
	strArray  [3]string
	intArray  [3]int
	strSlice  []string
	intSlice  []int
	mock      *Mock1
	mockArray [2]*Mock1
	MockSlice []*Mock1
	hash1     map[string]interface{}
	hash2     map[int]interface{}
	hash3     map[string]map[string]interface{}
}

func GenMokc(name string) Mock1 {
	test := Mock1{
		name:     name,
		age:      1,
		strArray: [3]string{"1", "2", "3"},
		intArray: [3]int{1, 2, 3},
		strSlice: []string{"1", "2", "3", "4"},
		intSlice: []int{1, 2, 3, 4},
		hash1: map[string]interface{}{
			"int":    1,
			"string": "string",
		},
		hash2: map[int]interface{}{
			1: 1,
			2: "string",
		},
		hash3: map[string]map[string]interface{}{
			"test1": {
				"int":    1,
				"string": "string",
			},
		},
	}

	test.mock = &test
	test.mockArray = [2]*Mock1{&test}
	test.MockSlice = []*Mock1{&test}
	return test
}

type Test struct {
	func_t func()
}

func main() {
	test := Test{
		func_t: func() {
			fmt.Println("test")
		},
	}
	// tp := ognl.Get(test, "func_t").Type()
	// fmt.Println(tp)
	value := ognl.Get(test, "func_t").Value()
	fmt.Println(value)

	mokeValue := reflect.ValueOf(test)
	mokeType := reflect.TypeOf(test)
	mokeField := mokeValue.NumField()

	fmt.Println(mokeValue, mokeType, mokeField)

	// for i := 0; i < mokeField; i++ {
	// 	fmt.Println(mokeType.Kind())

	// 	structField := mokeType.Field(i)
	// 	cp := reflect.New(mokeValue.Type()).Elem()
	// 	cp.Set(mokeValue)
	// 	rv := cp.FieldByName(structField.Name)
	// 	reflect.NewAt(rv.Type(), unsafe.Pointer(rv.Pointer())).Elem().Interface()

	// 	// 获取基本数据value
	// 	// value := ognl.Get(test, structField.Name).Value()
	// 	// if rv.Kind() == reflect.Func {
	// 	// 	fmt.Println("===")
	// 	// }

	// }
}

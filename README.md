# go_ognl

关键字符`.`、`#`,遇到都会截断到下一层(`#`和特殊,会将当前层展开,例如一个struct,会将所有字段对应的值展开)

---

1. 直接使用 `go_ognl.Get(obj,path)` 获取需要的值,返回对象提供 `Effective()` 判断是否有有效值, `Value()`
   返回解析后的对象,如果使用`#`将会返回`[]interface{}{}`,`Values()`则直接回返回一个`[]interface{}{}`
   ,同时也提供`.Get(path)`链式调用;
2. 使用`go_ognl.Parse(obj)` 保存为临时,可以在此基础上多次调用 `.Get(path)`

## download
```shell
go get github.com/golang-infrastructure/go-ognl
```

## demo

```go
package main

import (
	"fmt"
	"github.com/songzhibin97/go-ognl"
)

type Test struct {
	First  string
	Middle *Test
	last   int
}

func main() {
	t1 := &Test{
		First: "first",
		last:  7,
	}
	t1.Middle = t1
	fmt.Println(ognl.Get(t1, "First").Value())                     // first
	fmt.Println(ognl.Get(t1, "last").Value())                      // 7
	fmt.Println(ognl.Get(t1, "Middle.Middle.Middle.last").Value()) // 7
	fmt.Println(ognl.Get(t1, "#").Value())                         // []interface{}{"first",t1,7}
	fmt.Println(ognl.Get(t1, "##").Value())                        // []interface{}{"first","first",t1,7,7}
	fmt.Println(ognl.Get(t1, "##").Values())                       // []interface{}{"first","first",t1,7,7}
}

```
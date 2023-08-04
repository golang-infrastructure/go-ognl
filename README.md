# go_ognl

## 介绍

go_ognl允许您在复杂结构下不解析结构的情况下，通过ognl表达式获取结构中的值。

## 特性

1. 使用方便,跟json parser一样方便;
2. 不论是私有属性还是公有属性都可以访问;
3. 支持层级展开,支持数组展开;
4. 轻量级,无特殊依赖;

Ps: 实现依赖反射,结构转化会存在性能损耗

## 安装

要在您的 Go 项目中使用 Go-OGNL，您需要先安装和设置好 Go 环境。然后，您可以使用以下 go get 命令进行安装：

```shell
go get github.com/golang-infrastructure/go-ognl
```

## 使用

1. 导入go_ognl
   ```go
   import "github.com/golang-infrastructure/go-ognl"
   ```
2. 可以直接使用
   ```go
   go_ognl.Get(obj,path)
   ```
3. 也可以直接调用parse方法,在此基础上可以实现多次调用or链式调用
   ```go
   parser := go_ognl.Parse(obj)
   parser.Get(path)
   ```

## 完整示例

```go
package main

import (
	"fmt"
	"github.com/golang-infrastructure/go-ognl"
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

## 关键词说明

1. `.`代表路径分隔符(Q: 那如果我的key就是带`.`怎么办? A:使用`\\`进行转义)
2. `#`表示当前层级进行展开操作,比如我是一个obj list,使用`#`后的path针对每一个obj进行相同的操作

## API

- `Get(value interface{}, path string) Result`
- `GetE(value interface{}, path string) (Result, error)`
- `GetMany(value interface{}, path ...string) []Result`
- `Parse(result interface{}) Result`

1. Result.Effective() // 判断值是否有效
2. Result.Type() // 获取值类型
3. Result.Value() // 获取值
4. Result.Values() // 获取值列表,如果是数组,则展开数组,如果是单个值,那么数组只会有一个
5. Result.ValuesE() // 带有错误返回
6. Result.Diagnosis() // 获取诊断信息
7. Result.Get(path)
8. Result.GetE(path) // 支持链式调用
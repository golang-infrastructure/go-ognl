# go_ognl

## 介绍

go_ognl 允许您通过 ognl 表达式从任意 Go 对象中取值，无需提前定义结构体或做反序列化——直接传入对象和路径即可。

## 特性

1. 使用方便,跟 json parser 一样方便;
2. 不论是私有属性还是公有属性都可以访问;
3. 支持层级展开,支持数组展开;
4. 轻量级,无特殊依赖;

Ps: 实现依赖反射(并使用 `unsafe` 读取私有字段),相比手写字段访问会有性能损耗。

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

1. `.` 代表路径分隔符(Q: 那如果我的 key 就是带 `.` 怎么办? A: 使用 `\\` 进行转义,例如 `Foo\\.Bar` 表示 key 为 `Foo.Bar`)
2. `#` 表示对**当前已下钻到的值**进行展开:若它是一个 list,则 `#` 之后的 path 会作用到每一个元素上(类似 flat-map);`##` 表示展开两层。
   - 路径段为非负整数时按下标取值(slice/array 元素、`map[int]` 的 key、或 struct 的第 N 个字段);否则作为字符串 key 或字段名。

## 并发安全

`Get`/`GetE` 及 `Result` 上的方法都不会修改入参,且每次调用各自分配返回切片,因此可以对同一个对象或同一个 `Result` 并发只读调用(前提是底层对象本身没有被其它地方并发修改)。

## 错误处理

- `Get` 不返回 error:无法解析时 `Result.Effective()` 返回 `false`,过程中的非致命错误记录在 `Result.Diagnosis()` 中。路径长度受限,对抗性路径不会耗尽调用栈。
- `GetE` 返回首个致命错误。
- 所有错误都用 `%w` 包装,可用 `errors.Is(err, ognl.ErrInvalidValue)` 等判断;错误信息**只包含被遍历对象的类型名,不含其值**,避免泄露密码 / token 等敏感数据。

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
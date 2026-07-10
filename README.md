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

`Result` 自有的切片通过访问器返回时会浅拷贝:`#` 展开后的 `Value()`、`Values()`、`ValuesE()` 以及 `Diagnosis()` 每次调用都返回独立的 slice backing,调用方修改切片元素不会污染 `Result` 或其它访问器结果。切片中的对象不会深拷贝,仍保持原有身份。非展开 `Value()` 返回解析出的值本身;若它是 slice 或 map,仍与调用方输入共享所有权。

## 错误处理

- `Get` 不返回 error:无法解析时 `Result.Effective()` 返回 `false`,过程中的非致命错误记录在 `Result.Diagnosis()` 中。递归下钻深度受限,对抗性路径不会耗尽调用栈。
- `GetE` 返回首个致命错误。
- 每次 `Get`/`GetE`（包括 `Result` 上的同名方法）的 `#` 展开最多执行 100,000 次操作，并在整个返回结果树中累计保留 10,000 个结果。超限时 `Get` 返回无效结果并在 `Diagnosis()` 中记录 `ErrExpansionLimit`，`GetE` 返回可由 `errors.Is` 识别的 `ErrExpansionLimit`，且不返回部分展开结果。
- 所有错误都用 `%w` 包装,可用 `errors.Is(err, ognl.ErrInvalidValue)` 等判断;错误信息**只包含被遍历对象的类型名,不含其值**,避免泄露密码 / token 等敏感数据。

## Result 兼容性契约

以下编号不变量固定当前 pre-1.0 行为。`Type` 是遍历元数据,不要把它当作 `Value()` 的动态 Go 类型。

1. **C1 — Type 元数据。** `Parse(v)` 和空路径从 `Interface` 开始;成功解析的非展开字段/下标使用解析结果的 kind。`#` 使用展开过程报告的元素 kind,空展开为 `Interface`;已经展开的结果继续映射时不会重新计算 `Type`。因此非空 `Get([]User{{Name: "a"}}, "#.Name")` 的 `Type` 是 `Struct`,而 empty `Get([]User{}, "#.Name")` 的 `Type` 是 `Interface`。
2. **C2 — Effective 与 typed nil。** `Invalid` 一定无效;展开结果仅在列表非空时有效;非展开结果按保存的 interface 是否为 nil 判断。typed nil pointer/map/slice 保存在非 nil interface 中,所以 `Parse(typedNil).Effective()` 为 `true`,即使 `Value()` 中的动态值为 nil。
3. **C3 — empty flat-map。** 对空集合执行第一个 `#` 会成功得到 `Interface`、empty、ineffective Result,`GetE` 不报错。再对这个已展开的空结果应用字段或第二个 `#` 时没有任何匹配:`Get` 仍返回 ineffective Result,`GetE` 返回包装的 `ErrInvalidValue`。
4. **C4 — scalar Values。** `Values()` 是忽略展开错误的 best-effort API,所以标量 `42` 返回 `[]interface{}{42}`;`ValuesE()` 会报告同一次展开的错误,返回 nil values 和包装的 `ErrUnableExpand`。

下表中的 `User` 表示 `type User struct { Name string }`。

| 表达式 / API | `Type()` | `Effective()` | 其它可观察结果 |
| --- | --- | --- | --- |
| `Parse(42)` | `Interface` | `true` | `Values()==[42]`;`ValuesE()` 返回 nil、`ErrUnableExpand` |
| `Get(User{Name: "a"}, "Name")` | `String` | `true` | `Values()==["a"]` |
| `Get([]User{{Name: "a"}}, "#.Name")` | `Struct` | `true` | `Values()==["a"]` |
| `Get([]User{}, "#")` | `Interface` | `false` | values 为 nil;对应 `GetE` 不报错 |
| `Get([]User{}, "#.Name")` | `Interface` | `false` | values 为 nil;对应 `GetE` 返回 `ErrInvalidValue` |
| `Get([]User{}, "##")` | `Interface` | `false` | values 为 nil;对应 `GetE` 返回 `ErrInvalidValue` |
| `Parse((*User)(nil))`（typed nil map/slice 同理） | `Interface` | `true` | `Value()` 保存 typed nil |

上述矩阵由 [`result_contract_test.go`](./result_contract_test.go) 的 C1–C4 characterization tests 锁定（核对日期:2026-07-10）。

## API

- `Get(value interface{}, path string) Result`
- `GetE(value interface{}, path string) (Result, error)`
- `GetMany(value interface{}, path ...string) []Result`
- `Parse(result interface{}) Result`

1. Result.Effective() // 判断值是否有效
2. Result.Type() // 获取遍历类型元数据(见 C1)
3. Result.Value() // 获取值
4. Result.Values() // 获取值列表,如果是数组,则展开数组,如果是单个值,那么数组只会有一个
5. Result.ValuesE() // 带有错误返回
6. Result.Diagnosis() // 获取诊断信息
7. Result.Get(path)
8. Result.GetE(path) // 支持链式调用

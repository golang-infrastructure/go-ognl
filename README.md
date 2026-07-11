# go-ognl

## 介绍

go-ognl 允许您通过 OGNL 风格的路径表达式从 Go 对象图中取值，无需先将对象转换成其它数据格式。

## 特性

1. 支持 struct、map、slice、array、pointer 和 interface 的层级访问；
2. 可以读取导出和未导出的 struct 字段；
3. 支持对当前值进行一层或多层展开；
4. 包的非测试代码仅依赖 Go 标准库。

Ps: 实现依赖反射(并使用 `unsafe` 读取私有字段),相比手写字段访问会有性能损耗。

## 安装

项目的 canonical module path 是 `github.com/golang-infrastructure/go-ognl`。截至 2026-07-11，latest release 仍是声明旧 module path 的 `v0.0.3`，因此 canonical path 的 `@latest` 尚不可用。Owner 已批准当前 [`LICENSE`](LICENSE)（MIT；Copyright (c) 2022 Golang Infrastructure），但版本号与发布时间仍 deferred，本 PR 合并后也不创建 tag 或 release。`v0.1.0` 只是当前候选版本；若 owner 后续批准并发布该版本，再使用：

```shell
go get github.com/golang-infrastructure/go-ognl@v0.1.0
```

发布前置条件和发布后的 `@latest` 验证步骤见 [RELEASE.md](RELEASE.md)。

## 使用

1. 使用 `ognl` 作为导入名：
   ```go
   import ognl "github.com/golang-infrastructure/go-ognl"
   ```
2. 直接查询：
   ```go
   result := ognl.Get(obj, path)
   ```
3. 使用 `Parse` 后重复或链式查询：
   ```go
   parsed := ognl.Parse(obj)
   result := parsed.Get(path)
   ```

## 完整示例

```go
package main

import (
	"fmt"

	ognl "github.com/golang-infrastructure/go-ognl"
)

func main() {
	type User struct {
		Name  string
		Roles []string
	}

	user := User{Name: "alice", Roles: []string{"admin", "maintainer"}}
	fmt.Println(ognl.Get(user, "Name").Value())

	parsed := ognl.Parse(user)
	fmt.Println(parsed.Get("Roles.1").Value())

	users := []User{{Name: "alice"}, {Name: "bob"}}
	fmt.Println(ognl.Get(users, "#.Name").Values())
}
```

输出：

```text
alice
maintainer
[alice bob]
```

`TestREADMEExample` 会直接从 README 提取上述 Go 程序和输出，编译运行程序并核对实际输出，避免文档示例与测试漂移。

## 关键词说明

1. 未转义的 `.` 是路径分隔符；开头、结尾或连续的 `.` 不表示空 key。当前语法不能访问空字符串 map key。
2. 每个路径段开头未转义的 ASCII 空格、tab、换行和回车会被忽略；路径段开始后，这四个字符是 key 的字面内容。若 key 以这些字符开头，请转义第一个字符，例如 `\ leading`。
3. 数字路径段按当前容器解释：
   - `map[string]`（包括自定义 string key 类型）始终按原始段文本精确查找，因此 `1`、`01`、`+1` 是不同的 key。
   - `map[int]`（包括自定义 int key 类型）、slice、array 和 struct 支持十进制非负下标；允许前导零和一个前导 `+`，并兼容把 `-0`、`-00` 等全零负数解释为下标 0。负非零、溢出和其它文本不是下标。
4. `#` 对**当前已下钻到的值**进行展开：若它是一个 list，则后续路径段会作用到每一个元素上（类似 flat-map）；`##` 表示展开两层。展开后的每个元素会按自己的容器类型解释数字路径段。
5. `\` 是通用转义符，后一个字符按字面量处理。例如 `Foo\.Bar` 表示 key `Foo.Bar`，`\#` 表示字面量 `#`，`\\` 表示一个字面量反斜杠。Unicode key 按 UTF-8 精确匹配，不做归一化或大小写折叠。
6. 选择器末尾单独的反斜杠非法；末尾连续反斜杠为奇数个时非法，偶数个时表示字面量反斜杠。

## 并发安全

`Get`/`GetE` 及 `Result` 上的方法都不会修改入参,且每次调用各自分配返回切片,因此可以对同一个对象或同一个 `Result` 并发只读调用(前提是底层对象本身没有被其它地方并发修改)。

`Result` 自有的切片通过访问器返回时会浅拷贝:`#` 展开后的 `Value()`、`Values()`、`ValuesE()` 以及 `Diagnosis()` 每次调用都返回独立的 slice backing,调用方修改切片元素不会污染 `Result` 或其它访问器结果。切片中的对象不会深拷贝,仍保持原有身份。非展开 `Value()` 返回解析出的值本身;若它是 slice 或 map,仍与调用方输入共享所有权。

## 错误处理

- `Get` 不返回 error：无法解析时 `Result.Effective()` 返回 `false`，过程中的非致命错误记录在 `Result.Diagnosis()` 中。非法选择器会返回 `Type()==Invalid`、无部分结果，并在 Diagnosis 中包装 `ErrInvalidSelector`。
- `GetE` 返回致命错误。对已展开的结果，单个分支错误进入 `Diagnosis()`；只要至少一个分支匹配，调用仍可成功。所有分支都失败时不返回部分值，并返回输入顺序中的第一个失败上下文。非法选择器会在遍历前失败，并返回可由 `errors.Is(err, ognl.ErrInvalidSelector)` 识别的错误和无部分值的 Invalid Result。
- 普通 `.` 下钻使用迭代处理；匿名字段、pointer/interface 展开和 `#.` 等递归分支有深度上限。限制的是递归深度，不是 path 字符串长度。
- 每次 `Get`/`GetE`（包括 `Result` 上的同名方法）的 `#` 展开最多执行 100,000 次操作，并在整个返回结果树中累计保留 10,000 个结果。超限时 `Get` 返回无效结果并在 `Diagnosis()` 中记录 `ErrExpansionLimit`；`GetE` 返回可由 `errors.Is` 识别的 `ErrExpansionLimit`，且不暴露部分结果。
- 对外错误和诊断保留 sentinel unwrap 链，可用 `errors.Is` 检查。错误文本只包含安全编码的实际失败动态类型和数字位置字段 `offset`、`op`、`total_len`，不包含 selector、解码后的 key 或任何对象值；完整文本最多 352 bytes。

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

1. `Result.Effective()`：判断结果是否包含可用值。
2. `Result.Type()`：获取遍历类型元数据，不一定等于 `Value()` 的动态 Go 类型（见 C1）。
3. `Result.Value()`：非展开结果返回原值；展开结果每次返回独立 backing 的浅拷贝 `[]interface{}`。
4. `Result.Values()`：每次返回浅拷贝列表；可展开 slice、array、map 或 struct，标量返回单元素列表，展开错误会被忽略。
5. `Result.ValuesE()`：`Values` 的 error 返回版本；成功时同样返回浅拷贝。
6. `Result.Diagnosis()`：每次返回遍历中非致命错误列表的浅拷贝。
7. `Result.Get(path)`：继续查询当前结果；展开结果保持 flat-map 语义。
8. `Result.GetE(path)`：继续查询当前结果并按上述规则返回致命错误。

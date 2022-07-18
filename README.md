# go-ognl

1. 对象、map 使用 `.` 来获取(对象不确定能不能获取到未导出的值)
2. 数组使用 `[]` 来获取; `.len` 获取数组长度 
3. `first`、'last' 获取列表数组第一个以及最后一个 
4. 只支持map key为 string or int 
5. 可以忽略 `[]` 直接使用 Filed 10 要用空格隔开 
6. 获取到基础数据类型则直接返回

```go

func TestParse(t *testing.T) {

	t2 := &Mock{
		Name: "t2",
		Age:  2,
	}
	t3 := &Mock{
		Name: "t3",
		Age:  3,
	}
	t4 := &Mock{
		Name: "t4",
		Age:  4,
	}
	t1 := &Mock{
		Name: "t1",
		Age:  1,
		Hash1: map[string]interface{}{
			"string1": "string",
			"int1":    1,
			"t2":      t2,
		},
		Hash2: map[int]interface{}{
			2: t2,
			3: t3,
			4: t4,
		},
		List:  []*Mock{t2, t3, t4},
		Array: [3]*Mock{t2, t3, t4},
	}
	test := []struct {
		query string
		value interface{}
	}{
		{".Name", "t1"},
		{".Age", 1},
		{".Hash1.string1", "string"},
		{".Hash1.int1", 1},
		{".Hash1.t2.Name", "t2"},
		{".Hash2[2].Name", "t2"},
		{".List 1.Name", "t3"},
		{".List[0].Name", "t2"},
		{".Array[0].Name", "t2"},
		{".hash2 [0]", nil},
		{".List first.Name", "t2"},
		{".List last.Name", "t4"},
	}
	for index, v := range test {
		vv, err := Parser.parse(v.query, t1)
		if !assert.NoError(t, err) {
			t.Errorf("error:%v index:%d, query:%s, expected:%d, got:%d", err, index, v.query, v.value, vv)
			continue
		}
		if !assert.Equal(t, v.value, vv) {
			t.Errorf("no equal index:%d, query:%s, expected:%d, got:%d", index, v.query, v.value, vv)
		}
	}
}
```
package ognl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Mock struct {
	Name   string
	Age    int
	Hash1  map[string]interface{}
	Hash2  map[int]interface{}
	List   []*Mock
	Array  [3]*Mock
	lName  string
	lAge   int
	lHash1 map[string]interface{}
	lHash2 map[int]interface{}
	lList  []*Mock
	lArray [3]*Mock
}

func TestGet(t *testing.T) {
	var (
		t2 = &Mock{
			Name: "t2",
			Age:  2,
		}
		hash1 = map[string]interface{}{
			"string1": "string",
			"int1":    1,
			"t2":      t2,
		}
		t3 = &Mock{
			Name: "t3",
			Age:  3,
		}
		t4 = &Mock{
			Name:  "t4",
			Age:   4,
			Hash1: map[string]interface{}{},
		}
		hash2 = map[int]interface{}{
			2: t2,
			3: t3,
			4: t4,
		}
		list  = []*Mock{t2, t3, t4}
		array = [3]*Mock{t2, t3, t4}
		t1    = &Mock{
			Name:   "t1",
			lName:  "lt1",
			Age:    1,
			lAge:   11,
			Hash1:  hash1,
			lHash1: hash1,
			Hash2:  hash2,
			lHash2: hash2,
			List:   list,
			lList:  list,
			Array:  array,
			lArray: array,
		}
	)
	hash1["t1"] = t1
	test := []struct {
		query     string
		value     interface{}
		effective bool
	}{
		{"Name", "t1", true},
		{"Age", 1, true},
		{"Hash1.string1", "string", true},
		{"Hash1.int1", 1, true},
		{"Hash1.t2.Name", "t2", true},
		{"Hash2.2.Name", "t2", true},
		{"List.1.Name", "t3", true},
		{"List.0.Name", "t2", true},
		{"Array.0.Name", "t2", true},
		{"hash2.0", nil, false},
		{"Hash1.t1.Hash1.t1.Hash1.t1.Name", "t1", true},
		{"lName", "lt1", true},
		{"lAge", 11, true},
		{"lHash1.string1", "string", true},
		{"lHash1.int1", 1, true},
		{"lHash1.t2.Name", "t2", true},
		{"lHash2.2.Name", "t2", true},
		{"lList.1.Name", "t3", true},
		{"lList.0.Name", "t2", true},
		{"lArray.0.Name", "t2", true},
		{"lhash2.0", nil, false},
		{"lHash1.t1.Hash1.t1.lHash1.t1.Name", "t1", true},

		{"#", []interface{}{"t1", 1, hash1, hash2, list, array, "lt1", 11, hash1, hash2, list, array}, true},
		{"#string1#", []interface{}{"string", "string"}, true},
		{"List#1.Name", []interface{}{"t3", "t3", "t3", "t3"}, true},
	}
	for index, v := range test {
		vv := Get(t1, v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t, index:%d", v.effective, vv.Effective(), index)
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	for index, v := range test {
		vv := Get(*t1, v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t, index:%d", v.effective, vv.Effective(), index)
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	p := Parse(t1)
	for index, v := range test {
		vv := p.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	p = Parse(*t1)
	for index, v := range test {
		vv := p.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	test = []struct {
		query     string
		value     interface{}
		effective bool
	}{
		{"0", t2, true},
		{"1", t3, true},
		{"2", t4, true},
		{"3", nil, false},
		{"0.Name", "t2", true},
		{"0.Age", 2, true},
		{"1.Name", "t3", true},
		{"1.Age", 3, true},
		{"2.Name", "t4", true},
		{"2.Age", 4, true},
		{"#", []interface{}{t2, t3, t4}, true},
	}
	g1 := Get(t1, "List")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	g1 = Get(*t1, "List")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Value()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	test = []struct {
		query     string
		value     interface{}
		effective bool
	}{
		{"List", []interface{}{t2, t3, t4}, true},
		{"Array", []interface{}{t2, t3, t4}, true},
	}
	g1 = Get(t1, "")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Values()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}

	g1 = Get(*t1, "")
	for index, v := range test {
		vv := g1.Get(v.query)
		if !assert.Equal(t, v.effective, vv.Effective()) {
			t.Errorf("effective fault expected:%t, got:%t", v.effective, vv.Effective())
			return
		}
		if !assert.Equal(t, v.value, vv.Values()) {
			t.Errorf("no equal index:%d, query:%s, expected:%v, got:%v", index, v.query, v.value, vv.Value())
		}
	}
}

type Repetition struct {
	Repetition string
}

type RepetitionDetail struct {
	*Repetition
}

func TestRepetition(t *testing.T) {

	r := RepetitionDetail{
		Repetition: &Repetition{
			Repetition: "r1",
		},
	}

	vv := Get(r, "Repetition").Value()
	assert.Equal(t, vv, "r1")
}

type repetition struct {
	repetition string
}

type repetitionDetail struct {
	*repetition
}

func TestRepetition2(t *testing.T) {

	r := repetitionDetail{
		repetition: &repetition{
			repetition: "r2",
		},
	}

	vv := Get(r, "repetition").Value()
	assert.Equal(t, vv, "r2")
}

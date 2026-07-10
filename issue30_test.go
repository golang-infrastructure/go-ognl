package ognl_test

import (
	"errors"
	"sync"
	"testing"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue30Item struct {
	Name string
}

func issue30DeployedResult() (ognl.Result, *issue30Item, *issue30Item) {
	first := &issue30Item{Name: "first"}
	second := &issue30Item{Name: "second"}
	return ognl.Get([]*issue30Item{first, second}, "#"), first, second
}

func issue30ValuesE(t *testing.T, result ognl.Result) []interface{} {
	t.Helper()
	values, err := result.ValuesE()
	require.NoError(t, err)
	return values
}

func TestIssue30DeployedAccessorsReturnIndependentShallowSlices(t *testing.T) {
	result, firstItem, _ := issue30DeployedResult()
	replacement := &issue30Item{Name: "replacement"}

	accessors := []struct {
		name string
		get  func() []interface{}
	}{
		{
			name: "Value",
			get: func() []interface{} {
				return result.Value().([]interface{})
			},
		},
		{name: "Values", get: result.Values},
		{
			name: "ValuesE",
			get: func() []interface{} {
				return issue30ValuesE(t, result)
			},
		},
	}

	for _, accessor := range accessors {
		t.Run(accessor.name, func(t *testing.T) {
			first := accessor.get()
			second := accessor.get()
			require.Len(t, first, 2)
			require.Len(t, second, 2)

			first[0] = replacement

			assert.Same(t, firstItem, second[0], "separate calls must not share backing storage")
			assert.Same(t, firstItem, accessor.get()[0], "caller mutation must not alter the Result")
		})
	}

	value := result.Value().([]interface{})
	value[0] = replacement
	assert.Same(t, firstItem, result.Values()[0], "Value must not alias Values")

	values := result.Values()
	values[0] = replacement
	assert.Same(t, firstItem, issue30ValuesE(t, result)[0], "Values must not alias ValuesE")

	// The copy is intentionally shallow: the element object retains its identity.
	result.Values()[0].(*issue30Item).Name = "updated"
	assert.Equal(t, "updated", result.Value().([]interface{})[0].(*issue30Item).Name)
}

func TestIssue30DiagnosisReturnsIndependentSlice(t *testing.T) {
	result := ognl.Get([]interface{}{[]int{10, 20}, 99}, "#1")
	first := result.Diagnosis()
	second := result.Diagnosis()
	require.Len(t, first, 1)
	require.Len(t, second, 1)
	require.ErrorIs(t, first[0], ognl.ErrParseInt)

	first[0] = errors.New("caller replacement")

	assert.ErrorIs(t, second[0], ognl.ErrParseInt)
	assert.ErrorIs(t, result.Diagnosis()[0], ognl.ErrParseInt)

	mapped := result.Get("")
	require.NotEmpty(t, mapped.Diagnosis())
	assert.ErrorIs(t, mapped.Diagnosis()[0], ognl.ErrParseInt)

	mappedE, err := result.GetE("")
	require.NoError(t, err)
	require.NotEmpty(t, mappedE.Diagnosis())
	assert.ErrorIs(t, mappedE.Diagnosis()[0], ognl.ErrParseInt)
}

func TestIssue30AccessorMutationDoesNotAffectResultGetOrGetE(t *testing.T) {
	accessors := []struct {
		name string
		get  func(t *testing.T, result ognl.Result) []interface{}
	}{
		{
			name: "Value",
			get: func(t *testing.T, result ognl.Result) []interface{} {
				return result.Value().([]interface{})
			},
		},
		{
			name: "Values",
			get: func(t *testing.T, result ognl.Result) []interface{} {
				return result.Values()
			},
		},
		{
			name: "ValuesE",
			get: func(t *testing.T, result ognl.Result) []interface{} {
				return issue30ValuesE(t, result)
			},
		},
	}

	for _, accessor := range accessors {
		t.Run(accessor.name, func(t *testing.T) {
			result, _, _ := issue30DeployedResult()
			accessor.get(t, result)[0] = &issue30Item{Name: "replacement"}

			assert.Equal(t, []interface{}{"first", "second"}, result.Get("Name").Values())

			mapped, err := result.GetE("Name")
			require.NoError(t, err)
			assert.Equal(t, []interface{}{"first", "second"}, mapped.Values())
		})
	}
}

func TestIssue30AccessorCopiesPreserveNilAndEmpty(t *testing.T) {
	nilResult := ognl.Get([]int{}, "#")
	value := nilResult.Value().([]interface{})
	assert.Nil(t, value)
	assert.Nil(t, nilResult.Values())
	valuesE, err := nilResult.ValuesE()
	require.NoError(t, err)
	assert.Nil(t, valuesE)
	assert.Nil(t, nilResult.Diagnosis())

	emptyResult := ognl.Get([]int{1}, "#.missing")
	value = emptyResult.Value().([]interface{})
	require.NotNil(t, value)
	assert.Empty(t, value)

	values := emptyResult.Values()
	require.NotNil(t, values)
	assert.Empty(t, values)

	valuesE, err = emptyResult.ValuesE()
	require.NoError(t, err)
	require.NotNil(t, valuesE)
	assert.Empty(t, valuesE)
}

func TestIssue30NonDeployedValueOwnershipRemainsUnchanged(t *testing.T) {
	slice := []int{1, 2}
	sliceValue := ognl.Parse(slice).Value().([]int)
	sliceValue[0] = 9
	assert.Equal(t, []int{9, 2}, slice)

	mapping := map[string]int{"value": 1}
	mapValue := ognl.Parse(mapping).Value().(map[string]int)
	mapValue["value"] = 9
	assert.Equal(t, 9, mapping["value"])
}

func TestIssue30ConcurrentAccessorMutationAndResultReads(t *testing.T) {
	const size = 16
	items := make([]*issue30Item, size)
	for i := range items {
		items[i] = &issue30Item{Name: "original"}
	}
	result := ognl.Get(items, "#")
	diagnosed := ognl.Get([]interface{}{[]int{10, 20}, 99}, "#1")
	replacement := &issue30Item{Name: "replacement"}
	replacementErr := errors.New("caller replacement")

	var wg sync.WaitGroup
	for i := 0; i < size; i++ {
		i := i
		wg.Add(5)
		go func() {
			defer wg.Done()
			result.Value().([]interface{})[i] = replacement
		}()
		go func() {
			defer wg.Done()
			result.Values()[i] = replacement
		}()
		go func() {
			defer wg.Done()
			values, err := result.ValuesE()
			if err != nil {
				t.Errorf("ValuesE: %v", err)
				return
			}
			values[i] = replacement
		}()
		go func() {
			defer wg.Done()
			diagnosed.Diagnosis()[0] = replacementErr
		}()
		go func() {
			defer wg.Done()
			_ = result.Get("Name")
			_, _ = result.GetE("Name")
		}()
	}
	wg.Wait()

	assert.Equal(t, []interface{}{
		"original", "original", "original", "original",
		"original", "original", "original", "original",
		"original", "original", "original", "original",
		"original", "original", "original", "original",
	}, result.Get("Name").Values())
	assert.ErrorIs(t, diagnosed.Diagnosis()[0], ognl.ErrParseInt)
}

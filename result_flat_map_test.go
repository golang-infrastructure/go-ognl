package ognl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue28Row struct {
	Value int
}

func TestIssue28_ResultGetFlatMap(t *testing.T) {
	t.Run("single child expansion matches combined path", func(t *testing.T) {
		value := [][]int{{1, 2}, {3}}
		want := []interface{}{1, 2, 3}

		combined := Get(value, "##")
		chained := Parse(value).Get("#").Get("#")

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
	})

	t.Run("multi-level child expansion matches combined path", func(t *testing.T) {
		value := [][][]int{{{1, 2}, {3}}, {{4}}}
		want := []interface{}{1, 2, 3, 4}

		combined := Get(value, "###")
		chained := Parse(value).Get("#").Get("##")

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
	})

	t.Run("partial match skips empty and invalid children", func(t *testing.T) {
		value := []interface{}{
			map[string]interface{}{"values": []int{1, 2}},
			map[string]interface{}{"values": []int{}},
			nil,
		}
		want := []interface{}{1, 2}

		combined := Get(value, "#values#")
		chained := Parse(value).Get("#").Get("values#")

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
		require.Len(t, chained.Diagnosis(), 1)
		assert.True(t, errors.Is(chained.Diagnosis()[0], ErrInvalidValue))
	})

	t.Run("deployed child diagnosis is preserved", func(t *testing.T) {
		value := []interface{}{[]int{1, 2}, 3}
		want := []interface{}{1, 2, 3}

		combined := Get(value, "##")
		chained := Parse(value).Get("#").Get("#")

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
		require.Len(t, combined.Diagnosis(), 1)
		require.Len(t, chained.Diagnosis(), 1)
		assert.True(t, errors.Is(combined.Diagnosis()[0], ErrUnableExpand))
		assert.True(t, errors.Is(chained.Diagnosis()[0], ErrUnableExpand))
	})

	t.Run("nil and non-nil empty results stay distinct", func(t *testing.T) {
		var nilValue [][]int
		nilCombined := Get(nilValue, "##")
		nilChained := Parse(nilValue).Get("#").Get("#")

		assert.Nil(t, nilCombined.Value())
		assert.Nil(t, nilChained.Value())
		assert.False(t, nilCombined.Effective())
		assert.False(t, nilChained.Effective())

		emptyValue := [][]int{{}}
		emptyCombined := Get(emptyValue, "##")
		emptyChained := Parse(emptyValue).Get("#").Get("#")

		assert.NotNil(t, emptyCombined.Value())
		assert.NotNil(t, emptyChained.Value())
		assert.Empty(t, emptyCombined.Values())
		assert.Empty(t, emptyChained.Values())
		assert.False(t, emptyCombined.Effective())
		assert.False(t, emptyChained.Effective())
	})

	t.Run("non-deployed child remains one element", func(t *testing.T) {
		value := []issue28Row{{Value: 1}, {Value: 2}}
		want := []interface{}{1, 2}

		combined := Get(value, "#Value")
		chained := Parse(value).Get("#").Get("Value")

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
	})
}

func issue28ChainedBudgetInput(total int) [][]int {
	const firstChunk = 5_000
	return [][]int{
		make([]int, firstChunk),
		make([]int, total-firstChunk),
	}
}

func TestIssue28_ChainedFlatMapResultBudgetBoundary(t *testing.T) {
	const resultLimit = 10_000

	t.Run("exact boundary does not count deployed wrappers", func(t *testing.T) {
		input := issue28ChainedBudgetInput(resultLimit)
		first := Parse(input).Get("#")

		result := first.Get("#")
		require.True(t, result.Effective())
		require.Len(t, result.Values(), resultLimit)
		assertNoExpansionLimitDiagnosis(t, result)

		resultE, err := first.GetE("#")
		require.NoError(t, err)
		require.True(t, resultE.Effective())
		require.Len(t, resultE.Values(), resultLimit)
		assertNoExpansionLimitDiagnosis(t, resultE)
	})

	t.Run("aggregate over limit fails closed", func(t *testing.T) {
		input := issue28ChainedBudgetInput(resultLimit + 1)
		first := Parse(input).Get("#")

		requireExpansionLimitDiagnosis(t, first.Get("#"))

		resultE, err := first.GetE("#")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrExpansionLimit)
		assert.False(t, resultE.Effective())
		assert.Empty(t, resultE.Values())
	})
}

func TestIssue28_ResultGetEFlatMap(t *testing.T) {
	t.Run("single child expansion matches combined path", func(t *testing.T) {
		value := [][]int{{1, 2}, {3}}
		want := []interface{}{1, 2, 3}

		combined, err := GetE(value, "##")
		require.NoError(t, err)
		first, err := GetE(value, "#")
		require.NoError(t, err)
		chained, err := first.GetE("#")
		require.NoError(t, err)

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
	})

	t.Run("multi-level child expansion matches combined path", func(t *testing.T) {
		value := [][][]int{{{1, 2}, {3}}, {{4}}}
		want := []interface{}{1, 2, 3, 4}

		combined, err := GetE(value, "###")
		require.NoError(t, err)
		first, err := GetE(value, "#")
		require.NoError(t, err)
		chained, err := first.GetE("##")
		require.NoError(t, err)

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
	})

	t.Run("partial match skips empty and invalid children", func(t *testing.T) {
		value := []interface{}{
			map[string]interface{}{"values": []int{1, 2}},
			map[string]interface{}{"values": []int{}},
			nil,
		}
		want := []interface{}{1, 2}

		combined, err := GetE(value, "#values#")
		require.NoError(t, err)
		first, err := GetE(value, "#")
		require.NoError(t, err)
		chained, err := first.GetE("values#")
		require.NoError(t, err)

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
		require.Len(t, chained.Diagnosis(), 1)
		assert.True(t, errors.Is(chained.Diagnosis()[0], ErrInvalidValue))
	})

	t.Run("deployed child error becomes diagnosis", func(t *testing.T) {
		value := []interface{}{[]int{1, 2}, 3}
		want := []interface{}{1, 2, 3}

		combined, err := GetE(value, "##")
		require.NoError(t, err)
		first, err := GetE(value, "#")
		require.NoError(t, err)
		chained, err := first.GetE("#")
		require.NoError(t, err)

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
		require.Len(t, combined.Diagnosis(), 1)
		require.Len(t, chained.Diagnosis(), 1)
		assert.True(t, errors.Is(combined.Diagnosis()[0], ErrUnableExpand))
		assert.True(t, errors.Is(chained.Diagnosis()[0], ErrUnableExpand))
	})

	t.Run("nil and non-nil empty results stay distinct", func(t *testing.T) {
		var nilValue [][]int
		nilCombined, err := GetE(nilValue, "##")
		require.Error(t, err)
		first, err := GetE(nilValue, "#")
		require.NoError(t, err)
		nilChained, err := first.GetE("#")
		require.Error(t, err)

		assert.True(t, errors.Is(err, ErrInvalidValue))
		assert.Nil(t, nilCombined.Value())
		assert.Nil(t, nilChained.Value())
		assert.False(t, nilCombined.Effective())
		assert.False(t, nilChained.Effective())

		emptyValue := [][]int{{}}
		emptyCombined, err := GetE(emptyValue, "##")
		require.Error(t, err)
		first, err = GetE(emptyValue, "#")
		require.NoError(t, err)
		emptyChained, err := first.GetE("#")
		require.Error(t, err)

		assert.True(t, errors.Is(err, ErrInvalidValue))
		assert.NotNil(t, emptyCombined.Value())
		assert.NotNil(t, emptyChained.Value())
		assert.Empty(t, emptyCombined.Values())
		assert.Empty(t, emptyChained.Values())
		assert.False(t, emptyCombined.Effective())
		assert.False(t, emptyChained.Effective())
	})

	t.Run("non-deployed child remains one element", func(t *testing.T) {
		value := []issue28Row{{Value: 1}, {Value: 2}}
		want := []interface{}{1, 2}

		combined, err := GetE(value, "#Value")
		require.NoError(t, err)
		first, err := GetE(value, "#")
		require.NoError(t, err)
		chained, err := first.GetE("Value")
		require.NoError(t, err)

		require.Equal(t, want, combined.Values())
		require.Equal(t, want, chained.Values())
	})
}

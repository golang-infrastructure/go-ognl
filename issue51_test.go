package ognl_test

import (
	"testing"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue51NoOpSelectorKeepsNilInput(t *testing.T) {
	for _, selector := range []string{"", ".", "..."} {
		t.Run(selector, func(t *testing.T) {
			result := ognl.Get(nil, selector)
			assert.Equal(t, ognl.Interface, result.Type())
			assert.False(t, result.Effective())
			assert.Nil(t, result.Value())
			assert.Empty(t, result.Diagnosis())

			resultE, err := ognl.GetE(nil, selector)
			require.NoError(t, err)
			assert.Equal(t, ognl.Interface, resultE.Type())
			assert.False(t, resultE.Effective())
			assert.Nil(t, resultE.Value())
			assert.Empty(t, resultE.Diagnosis())
		})
	}
}

func TestIssue51TrailingSeparatorsKeepEmptyExpansion(t *testing.T) {
	for _, selector := range []string{"#", "#.", "#.."} {
		t.Run(selector, func(t *testing.T) {
			result := ognl.Get([]int{}, selector)
			assert.Equal(t, ognl.Interface, result.Type())
			assert.False(t, result.Effective())
			assert.Nil(t, result.Value())
			assert.Empty(t, result.Diagnosis())

			resultE, err := ognl.GetE([]int{}, selector)
			require.NoError(t, err)
			assert.Equal(t, ognl.Interface, resultE.Type())
			assert.False(t, resultE.Effective())
			assert.Nil(t, resultE.Value())
			assert.Empty(t, resultE.Diagnosis())
		})
	}
}

func TestIssue51ResultNoOpPreservesEmptyExpansionState(t *testing.T) {
	nilExpanded, err := ognl.GetE([]int{}, "#")
	require.NoError(t, err)

	nonNilEmpty := ognl.Get([]int{1}, "#.missing")
	require.NotNil(t, nonNilEmpty.Value())
	require.Empty(t, nonNilEmpty.Value())
	require.NotEmpty(t, nonNilEmpty.Diagnosis())

	tests := []struct {
		name       string
		result     ognl.Result
		valueIsNil bool
	}{
		{name: "nil empty", result: nilExpanded, valueIsNil: true},
		{name: "non-nil empty with diagnosis", result: nonNilEmpty, valueIsNil: false},
	}

	for _, tt := range tests {
		for _, selector := range []string{"", ".", "..."} {
			t.Run(tt.name+"/"+selector, func(t *testing.T) {
				got := tt.result.Get(selector)
				assert.Equal(t, tt.result.Type(), got.Type())
				assert.Equal(t, tt.result.Effective(), got.Effective())
				if tt.valueIsNil {
					assert.Nil(t, got.Value())
				} else {
					assert.NotNil(t, got.Value())
					assert.Empty(t, got.Value())
				}
				assert.Equal(t, tt.result.Diagnosis(), got.Diagnosis())

				gotE, err := tt.result.GetE(selector)
				require.NoError(t, err)
				assert.Equal(t, tt.result.Type(), gotE.Type())
				assert.Equal(t, tt.result.Effective(), gotE.Effective())
				if tt.valueIsNil {
					assert.Nil(t, gotE.Value())
				} else {
					assert.NotNil(t, gotE.Value())
					assert.Empty(t, gotE.Value())
				}
				assert.Equal(t, tt.result.Diagnosis(), gotE.Diagnosis())
			})
		}
	}
}

func TestIssue51RealOperationAfterEmptyExpansionStillFails(t *testing.T) {
	empty := []struct{ Name string }{}
	for _, selector := range []string{"#.Name", "#..Name", "##"} {
		t.Run("top-level/"+selector, func(t *testing.T) {
			result, err := ognl.GetE(empty, selector)
			require.Error(t, err)
			assert.ErrorIs(t, err, ognl.ErrInvalidValue)
			assert.False(t, result.Effective())
		})
	}

	expanded, err := ognl.GetE(empty, "#")
	require.NoError(t, err)
	for _, selector := range []string{"Name", ".Name", "#"} {
		t.Run("Result/"+selector, func(t *testing.T) {
			result, err := expanded.GetE(selector)
			require.Error(t, err)
			assert.ErrorIs(t, err, ognl.ErrInvalidValue)
			assert.False(t, result.Effective())
		})
	}
}

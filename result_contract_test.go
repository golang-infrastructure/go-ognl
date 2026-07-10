package ognl_test

import (
	"errors"
	"testing"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resultContractUser struct {
	Name string
}

func TestResultContract_C1TypeMetadata(t *testing.T) {
	tests := []struct {
		name      string
		result    ognl.Result
		wantType  ognl.Type
		effective bool
		values    []interface{}
	}{
		{
			name:      "Parse starts with interface metadata",
			result:    ognl.Parse(42),
			wantType:  ognl.Interface,
			effective: true,
			values:    []interface{}{42},
		},
		{
			name:      "non-expanded field uses resolved kind",
			result:    ognl.Get(resultContractUser{Name: "alice"}, "Name"),
			wantType:  ognl.String,
			effective: true,
			values:    []interface{}{"alice"},
		},
		{
			name:      "expanded projection keeps expansion metadata",
			result:    ognl.Get([]resultContractUser{{Name: "alice"}}, "#.Name"),
			wantType:  ognl.Struct,
			effective: true,
			values:    []interface{}{"alice"},
		},
		{
			name:      "empty expanded projection has interface metadata",
			result:    ognl.Get([]resultContractUser{}, "#.Name"),
			wantType:  ognl.Interface,
			effective: false,
			values:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.result.Type())
			assert.Equal(t, tt.effective, tt.result.Effective())
			assert.Equal(t, tt.values, tt.result.Values())
		})
	}
}

func TestResultContract_C2EffectiveTypedNil(t *testing.T) {
	var (
		pointer *resultContractUser
		mapping map[string]int
		slice   []int
	)

	tests := []struct {
		name           string
		result         ognl.Result
		wantType       ognl.Type
		effective      bool
		valueEqualsNil bool
	}{
		{name: "zero Result", result: ognl.Result{}, wantType: ognl.Invalid, effective: false, valueEqualsNil: true},
		{name: "untyped nil", result: ognl.Parse(nil), wantType: ognl.Interface, effective: false, valueEqualsNil: true},
		{name: "typed nil pointer", result: ognl.Parse(pointer), wantType: ognl.Interface, effective: true, valueEqualsNil: false},
		{name: "typed nil map", result: ognl.Parse(mapping), wantType: ognl.Interface, effective: true, valueEqualsNil: false},
		{name: "typed nil slice", result: ognl.Parse(slice), wantType: ognl.Interface, effective: true, valueEqualsNil: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.result.Type())
			assert.Equal(t, tt.effective, tt.result.Effective())
			assert.Equal(t, tt.valueEqualsNil, tt.result.Value() == nil)
			assert.Nil(t, tt.result.Value())
		})
	}
}

func TestResultContract_C3EmptyFlatMap(t *testing.T) {
	empty := []resultContractUser{}
	tests := []struct {
		path      string
		geteError bool
	}{
		{path: "#", geteError: false},
		{path: "#.Name", geteError: true},
		{path: "##", geteError: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ognl.Get(empty, tt.path)
			assert.Equal(t, ognl.Interface, result.Type())
			assert.False(t, result.Effective())
			assert.Nil(t, result.Values())

			result, err := ognl.GetE(empty, tt.path)
			if tt.geteError {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ognl.ErrInvalidValue))
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, ognl.Interface, result.Type())
			assert.False(t, result.Effective())
			assert.Nil(t, result.Values())
		})
	}

	expanded, err := ognl.GetE(empty, "#")
	require.NoError(t, err)
	for _, path := range []string{"Name", "#"} {
		t.Run("Result.GetE "+path, func(t *testing.T) {
			result, err := expanded.GetE(path)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ognl.ErrInvalidValue))
			assert.Equal(t, ognl.Interface, result.Type())
			assert.False(t, result.Effective())
			assert.Nil(t, result.Values())
		})
	}
}

func TestResultContract_C4ScalarValues(t *testing.T) {
	result := ognl.Parse(42)

	assert.Equal(t, []interface{}{42}, result.Values())

	values, err := result.ValuesE()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrUnableExpand))
	assert.Nil(t, values)
}

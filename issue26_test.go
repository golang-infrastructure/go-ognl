package ognl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue26Inner struct {
	A int
}

type issue26Outer struct {
	*issue26Inner
}

type issue26Middle struct {
	*issue26Inner
}

type issue26MultiOuter struct {
	*issue26Middle
}

type issue26PrivateInner struct {
	A int
}

type issue26PrivateOuter struct {
	*issue26PrivateInner
}

func TestIssue26NilEmbeddedPointerPromotedField(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "single level",
			value: issue26Outer{},
		},
		{
			name: "multiple levels",
			value: issue26MultiOuter{
				issue26Middle: &issue26Middle{},
			},
		},
		{
			name:  "private embedded type",
			value: issue26PrivateOuter{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" Get", func(t *testing.T) {
			var result Result
			require.NotPanics(t, func() {
				result = Get(tt.value, "A")
			})

			assert.False(t, result.Effective())
			require.NotEmpty(t, result.Diagnosis())
			assert.True(t, errors.Is(result.Diagnosis()[0], ErrInvalidValue))
		})

		t.Run(tt.name+" GetE", func(t *testing.T) {
			var err error
			require.NotPanics(t, func() {
				_, err = GetE(tt.value, "A")
			})

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidValue))
		})
	}
}

func TestIssue26NonNilEmbeddedPointerPromotedField(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  int
	}{
		{
			name: "single level",
			value: issue26Outer{
				issue26Inner: &issue26Inner{A: 11},
			},
			want: 11,
		},
		{
			name: "multiple levels",
			value: issue26MultiOuter{
				issue26Middle: &issue26Middle{
					issue26Inner: &issue26Inner{A: 22},
				},
			},
			want: 22,
		},
		{
			name: "private embedded type",
			value: issue26PrivateOuter{
				issue26PrivateInner: &issue26PrivateInner{A: 33},
			},
			want: 33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Get(tt.value, "A").Value())

			result, err := GetE(tt.value, "A")
			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Value())
		})
	}
}

package ognl_test

import (
	"errors"
	"testing"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Issue31ExportedInner struct {
	Name  string
	Count int
}

type issue31PrivateInner struct {
	Name  string
	Count int
}

type issue31ExportedValueOuter struct {
	Issue31ExportedInner
	Tail bool
}

type issue31ExportedPointerOuter struct {
	*Issue31ExportedInner
	Tail bool
}

type issue31PrivateValueOuter struct {
	issue31PrivateInner
	Tail bool
}

type issue31PrivatePointerOuter struct {
	*issue31PrivateInner
	Tail bool
}

type issue31EmbeddingCase struct {
	name             string
	value            interface{}
	embedded         interface{}
	embeddedSelector string
	promotedName     string
	wantType         ognl.Type
}

func issue31EmbeddingCases() []issue31EmbeddingCase {
	exportedPointer := &Issue31ExportedInner{Name: "exported-pointer", Count: 2}
	privatePointer := &issue31PrivateInner{Name: "private-pointer", Count: 4}

	return []issue31EmbeddingCase{
		{
			name:             "exported value",
			value:            issue31ExportedValueOuter{Issue31ExportedInner: Issue31ExportedInner{Name: "exported-value", Count: 1}, Tail: true},
			embedded:         Issue31ExportedInner{Name: "exported-value", Count: 1},
			embeddedSelector: "Issue31ExportedInner",
			promotedName:     "exported-value",
			wantType:         ognl.Struct,
		},
		{
			name:             "exported pointer",
			value:            issue31ExportedPointerOuter{Issue31ExportedInner: exportedPointer, Tail: true},
			embedded:         exportedPointer,
			embeddedSelector: "Issue31ExportedInner",
			promotedName:     "exported-pointer",
			wantType:         ognl.Pointer,
		},
		{
			name:             "private value",
			value:            issue31PrivateValueOuter{issue31PrivateInner: issue31PrivateInner{Name: "private-value", Count: 3}, Tail: true},
			embedded:         issue31PrivateInner{Name: "private-value", Count: 3},
			embeddedSelector: "issue31PrivateInner",
			promotedName:     "private-value",
			wantType:         ognl.Struct,
		},
		{
			name:             "private pointer",
			value:            issue31PrivatePointerOuter{issue31PrivateInner: privatePointer, Tail: true},
			embedded:         privatePointer,
			embeddedSelector: "issue31PrivateInner",
			promotedName:     "private-pointer",
			wantType:         ognl.Pointer,
		},
	}
}

func TestIssue31_StructIndexReturnsDirectEmbeddedField(t *testing.T) {
	for _, tt := range issue31EmbeddingCases() {
		t.Run(tt.name+"/Get", func(t *testing.T) {
			embedded := ognl.Get(tt.value, "0")
			assert.Equal(t, tt.wantType, embedded.Type())
			assert.Equal(t, tt.embedded, embedded.Value())
			assert.Equal(t, true, ognl.Get(tt.value, "1").Value())
		})

		t.Run(tt.name+"/GetE", func(t *testing.T) {
			embedded, err := ognl.GetE(tt.value, "0")
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, embedded.Type())
			assert.Equal(t, tt.embedded, embedded.Value())

			tail, err := ognl.GetE(tt.value, "1")
			require.NoError(t, err)
			assert.Equal(t, true, tail.Value())
		})
	}
}

func TestIssue31_StructNameSelectorsRemainUnchanged(t *testing.T) {
	for _, tt := range issue31EmbeddingCases() {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.promotedName, ognl.Get(tt.value, "Name").Value())
			assert.Equal(t, tt.embedded, ognl.Get(tt.value, tt.embeddedSelector).Value())

			promoted, err := ognl.GetE(tt.value, "Name")
			require.NoError(t, err)
			assert.Equal(t, tt.promotedName, promoted.Value())

			embedded, err := ognl.GetE(tt.value, tt.embeddedSelector)
			require.NoError(t, err)
			assert.Equal(t, tt.embedded, embedded.Value())
		})
	}
}

type Issue31NameCollision struct {
	Issue31NameCollision string
}

type issue31NameCollisionOuter struct {
	*Issue31NameCollision
}

func TestIssue31_AnonymousNameSelectorPromotionRemainsUnchanged(t *testing.T) {
	value := issue31NameCollisionOuter{
		Issue31NameCollision: &Issue31NameCollision{Issue31NameCollision: "promoted"},
	}

	assert.Equal(t, "promoted", ognl.Get(value, "Issue31NameCollision").Value())
	result, err := ognl.GetE(value, "Issue31NameCollision")
	require.NoError(t, err)
	assert.Equal(t, "promoted", result.Value())
}

func TestIssue31_StructIndexOutOfBoundsRemainsTyped(t *testing.T) {
	value := issue31ExportedValueOuter{}

	result := ognl.Get(value, "2")
	require.NotEmpty(t, result.Diagnosis())
	assert.True(t, errors.Is(result.Diagnosis()[0], ognl.ErrStructIndexOutOfBounds))

	_, err := ognl.GetE(value, "2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrStructIndexOutOfBounds))
}

func TestIssue31_NilEmbeddedPointerIndexRemainsDirect(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{name: "exported", value: issue31ExportedPointerOuter{Tail: true}},
		{name: "private", value: issue31PrivatePointerOuter{Tail: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ognl.Get(tt.value, "0")
			assert.Equal(t, ognl.Pointer, result.Type())
			assert.True(t, result.Effective())
			assert.Nil(t, result.Value())
			assert.Equal(t, true, ognl.Get(tt.value, "1").Value())

			result, err := ognl.GetE(tt.value, "0")
			require.NoError(t, err)
			assert.Equal(t, ognl.Pointer, result.Type())
			assert.True(t, result.Effective())
			assert.Nil(t, result.Value())

			tail, err := ognl.GetE(tt.value, "1")
			require.NoError(t, err)
			assert.Equal(t, true, tail.Value())
		})
	}
}

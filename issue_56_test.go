package ognl

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue56ParseSelectorStopsAtExpansionTokenLimit(t *testing.T) {
	path := strings.Repeat("#", maxExpansionOperations+1) + `\`

	tokens, err := parseSelector(path)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, errors.Is(err, ErrInvalidSelector))
	assert.Nil(t, tokens)
}

func TestIssue56ParseSelectorExpansionTokenBoundary(t *testing.T) {
	t.Run("exact boundary", func(t *testing.T) {
		path := strings.Repeat("#", maxExpansionOperations)

		tokens, err := parseSelector(path)

		require.NoError(t, err)
		require.Len(t, tokens, maxExpansionOperations)
		assert.Equal(t, uint64(maxExpansionOperations-1), tokens[len(tokens)-1].operation)

		result := Get([]int{}, path)
		assert.Equal(t, Interface, result.Type())
		assert.False(t, result.Effective())
		assert.Empty(t, result.Diagnosis())
	})

	t.Run("segments do not consume expansion token allowance", func(t *testing.T) {
		path := "field" + strings.Repeat("#", maxExpansionOperations)

		tokens, err := parseSelector(path)

		require.NoError(t, err)
		require.Len(t, tokens, maxExpansionOperations+1)
		assert.Equal(t, selectorSegmentToken, tokens[0].kind)
		assert.Equal(t, uint64(maxExpansionOperations), tokens[len(tokens)-1].operation)
	})

	t.Run("exact boundary still scans invalid suffix", func(t *testing.T) {
		path := strings.Repeat("#", maxExpansionOperations) + `\`

		tokens, err := parseSelector(path)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSelector)
		assert.False(t, errors.Is(err, ErrExpansionLimit))
		assert.Nil(t, tokens)
	})
}

func TestIssue56PublicAPIsFailClosedAtParserExpansionLimit(t *testing.T) {
	path := strings.Repeat("#", maxExpansionOperations+1) + `\`
	wantLocation := fmt.Sprintf(",offset=%d,op=%d,total_len=%d: ", maxExpansionOperations, maxExpansionOperations, len(path))
	value := []int{}

	assertGet := func(t *testing.T, result Result) {
		t.Helper()
		assert.Equal(t, Invalid, result.Type())
		assert.False(t, result.Effective())
		assert.Nil(t, result.Value())
		require.Len(t, result.Diagnosis(), 1)
		contextual := result.Diagnosis()[0]
		assert.ErrorIs(t, contextual, ErrExpansionLimit)
		assert.False(t, errors.Is(contextual, ErrInvalidSelector))
		assert.Contains(t, contextual.Error(), wantLocation)
		assert.LessOrEqual(t, len(contextual.Error()), maxContextErrorLen)
		assert.True(t, utf8.ValidString(contextual.Error()))
	}
	assertGetE := func(t *testing.T, result Result, err error) {
		t.Helper()
		assert.Equal(t, Invalid, result.Type())
		assert.False(t, result.Effective())
		assert.Nil(t, result.Value())
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrExpansionLimit)
		assert.False(t, errors.Is(err, ErrInvalidSelector))
		assert.Contains(t, err.Error(), wantLocation)
		assert.LessOrEqual(t, len(err.Error()), maxContextErrorLen)
		assert.True(t, utf8.ValidString(err.Error()))
	}

	t.Run("Get", func(t *testing.T) {
		assertGet(t, Get(value, path))
	})

	t.Run("GetE", func(t *testing.T) {
		result, err := GetE(value, path)
		assertGetE(t, result, err)
	})

	parsed := Parse(value)
	t.Run("Result.Get", func(t *testing.T) {
		assertGet(t, parsed.Get(path))
	})

	t.Run("Result.GetE", func(t *testing.T) {
		result, err := parsed.GetE(path)
		assertGetE(t, result, err)
	})
}

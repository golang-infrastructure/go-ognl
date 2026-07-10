package ognl

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These literals intentionally duplicate the documented contract instead of
// referencing production constants, so implementation drift makes tests fail.
const (
	issue27MaxExpansionOperations = 100_000
	issue27MaxExpansionResults    = 10_000
)

type issue27Node struct {
	Left  *issue27Node
	Right *issue27Node
}

func issue27CyclicNode() *issue27Node {
	root := &issue27Node{}
	root.Left = root
	root.Right = root
	return root
}

func requireExpansionLimitDiagnosis(t *testing.T, result Result) {
	t.Helper()
	require.False(t, result.Effective())
	require.Empty(t, result.Values(), "an expansion-limit failure must not expose partial output")
	require.NotEmpty(t, result.Diagnosis())
	assert.True(t, errors.Is(result.Diagnosis()[len(result.Diagnosis())-1], ErrExpansionLimit))
}

func assertNoExpansionLimitDiagnosis(t *testing.T, result Result) {
	t.Helper()
	for _, diagnosis := range result.Diagnosis() {
		assert.False(t, errors.Is(diagnosis, ErrExpansionLimit), "unexpected expansion-limit diagnosis: %v", diagnosis)
	}
}

func TestIssue27_ExpansionBudgetBoundsExponentialHash(t *testing.T) {
	root := issue27CyclicNode()

	// Twelve binary expansions are deliberately below the fixed output limit
	// and lock the existing successful expansion behavior.
	normalPath := strings.Repeat("#", 12)
	normal := Get(root, normalPath)
	require.True(t, normal.Effective())
	require.Len(t, normal.Values(), 1<<12)
	assertNoExpansionLimitDiagnosis(t, normal)

	normalE, err := GetE(root, normalPath)
	require.NoError(t, err)
	require.Len(t, normalE.Values(), 1<<12)

	// The issue's 18-level reproducer previously allocated and returned 262144
	// elements. It must now fail closed before the result crosses the cap.
	overPath := strings.Repeat("#", 18)
	requireExpansionLimitDiagnosis(t, Get(root, overPath))

	overE, err := GetE(root, overPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, overE.Effective())
	assert.Empty(t, overE.Values())
}

func TestIssue27_ExpansionOutputBoundary(t *testing.T) {
	exactInput := make([]int, issue27MaxExpansionResults)
	exact := Get(exactInput, "#")
	require.True(t, exact.Effective())
	require.Len(t, exact.Values(), issue27MaxExpansionResults)
	assertNoExpansionLimitDiagnosis(t, exact)

	exactE, err := GetE(exactInput, "#")
	require.NoError(t, err)
	require.Len(t, exactE.Values(), issue27MaxExpansionResults)

	overInput := make([]int, issue27MaxExpansionResults+1)
	requireExpansionLimitDiagnosis(t, Get(overInput, "#"))

	overE, err := GetE(overInput, "#")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, overE.Effective())
	assert.Empty(t, overE.Values())
}

func TestIssue27_AggregateOutputLimitDiscardsPartialResult(t *testing.T) {
	chunkSize := issue27MaxExpansionResults/2 + 1
	input := []interface{}{make([]int, chunkSize), make([]int, chunkSize)}

	requireExpansionLimitDiagnosis(t, Get(input, "##"))

	result, err := GetE(input, "##")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, result.Effective())
	assert.Empty(t, result.Values())
}

func TestIssue27_ExpansionOperationBoundary(t *testing.T) {
	// A one-element cycle keeps the output size fixed while every '#' consumes
	// one operator operation and scans one element. This isolates the operation
	// budget from the output budget.
	cycle := make([]interface{}, 1)
	cycle[0] = cycle
	exactPath := strings.Repeat("#", issue27MaxExpansionOperations/2)
	exact := Get(cycle, exactPath)
	require.True(t, exact.Effective())
	require.Len(t, exact.Values(), 1)
	assertNoExpansionLimitDiagnosis(t, exact)

	exactE, err := GetE(cycle, exactPath)
	require.NoError(t, err)
	require.True(t, exactE.Effective())
	require.Len(t, exactE.Values(), 1)

	overPath := exactPath + "#"
	requireExpansionLimitDiagnosis(t, Get(cycle, overPath))

	overE, err := GetE(cycle, overPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, overE.Effective())
	assert.Empty(t, overE.Values())
}

func TestIssue27_EmptyAndPartialExpansionRemainCompatible(t *testing.T) {
	empty := Get([]int{}, "#")
	assert.False(t, empty.Effective())
	assertNoExpansionLimitDiagnosis(t, empty)

	emptyE, err := GetE([]int{}, "#")
	require.NoError(t, err)
	assert.False(t, emptyE.Effective())

	input := []interface{}{[]int{1, 2}, 42}
	partial := Get(input, "##")
	require.Equal(t, []interface{}{1, 2, 42}, partial.Values())
	require.NotEmpty(t, partial.Diagnosis())
	assert.ErrorIs(t, partial.Diagnosis()[0], ErrUnableExpand)
	assertNoExpansionLimitDiagnosis(t, partial)

	partialE, err := GetE(input, "##")
	require.NoError(t, err)
	assert.Equal(t, []interface{}{1, 2, 42}, partialE.Values())
	require.NotEmpty(t, partialE.Diagnosis())
	assert.ErrorIs(t, partialE.Diagnosis()[0], ErrUnableExpand)
}

func TestIssue27_ResultMethodsShareOneCallBudget(t *testing.T) {
	cycle := make([]interface{}, 1)
	cycle[0] = cycle
	base := Get([]interface{}{cycle, cycle}, "#")
	require.True(t, base.Effective())

	// Each item alone stays below the operation limit. Sharing one budget across
	// both items is what makes the Result.Get/Result.GetE call exceed the cap.
	path := strings.Repeat("#", issue27MaxExpansionOperations/4+1)
	requireExpansionLimitDiagnosis(t, base.Get(path))

	result, err := base.GetE(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, result.Effective())
	assert.Empty(t, result.Values())
}

func issue27NestedFanoutInput() []interface{} {
	input := make([]interface{}, 10)
	for i := range input {
		input[i] = make([]int, 9_000)
	}
	return input
}

func TestIssue27_GetAccumulatesRetainedResults(t *testing.T) {
	requireExpansionLimitDiagnosis(t, Get(issue27NestedFanoutInput(), "#.#"))
}

func TestIssue27_GetEAccumulatesRetainedResults(t *testing.T) {
	result, err := GetE(issue27NestedFanoutInput(), "#.#")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, result.Effective())
	assert.Empty(t, result.Values())
}

func TestIssue27_ResultGetAccumulatesRetainedResults(t *testing.T) {
	requireExpansionLimitDiagnosis(t, Parse(issue27NestedFanoutInput()).Get("#.#"))
}

func TestIssue27_ResultGetEAccumulatesRetainedResults(t *testing.T) {
	result, err := Parse(issue27NestedFanoutInput()).GetE("#.#")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpansionLimit)
	assert.False(t, result.Effective())
	assert.Empty(t, result.Values())
}

func TestIssue27_PublicCallsHaveIndependentResultBudgets(t *testing.T) {
	input := make([]int, 9_000)

	for i := 0; i < 2; i++ {
		require.Len(t, Get(input, "#").Values(), 9_000)

		resultE, err := GetE(input, "#")
		require.NoError(t, err)
		require.Len(t, resultE.Values(), 9_000)

		require.Len(t, Parse(input).Get("#").Values(), 9_000)

		resultMethodE, err := Parse(input).GetE("#")
		require.NoError(t, err)
		require.Len(t, resultMethodE.Values(), 9_000)
	}
}

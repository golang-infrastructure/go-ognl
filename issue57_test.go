package ognl_test

import (
	"testing"
	"time"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Issue57Node struct {
	*Issue57Node
	ID int
}

type Issue57NamedPointerCycle *Issue57NamedPointerCycle

type Issue57AnonymousValue interface{}

type issue57AnonymousInterfaceHolder struct {
	Issue57AnonymousValue
}

func assertIssue57DirectAnonymousField(t *testing.T, value, want *Issue57Node) {
	t.Helper()

	result := ognl.Get(value, "Issue57Node")
	require.Equal(t, ognl.Pointer, result.Type())
	assert.True(t, result.Effective())
	assert.Same(t, want, result.Value())

	resultE, err := ognl.GetE(value, "Issue57Node")
	require.NoError(t, err)
	require.Equal(t, ognl.Pointer, resultE.Type())
	assert.True(t, resultE.Effective())
	assert.Same(t, want, resultE.Value())
}

func TestIssue57RecursiveAnonymousNameReturnsDirectField(t *testing.T) {
	tail := &Issue57Node{ID: 3}
	middle := &Issue57Node{Issue57Node: tail, ID: 2}
	root := &Issue57Node{Issue57Node: middle, ID: 1}

	assertIssue57DirectAnonymousField(t, root, middle)
	assertIssue57DirectAnonymousField(t, middle, tail)
}

func TestIssue57RecursiveAnonymousNameReturnsSelfCycle(t *testing.T) {
	cycle := &Issue57Node{ID: 1}
	cycle.Issue57Node = cycle

	assertIssue57DirectAnonymousField(t, cycle, cycle)
}

func TestIssue57RecursiveAnonymousNamePreservesTypedNil(t *testing.T) {
	value := &Issue57Node{ID: 1}

	result := ognl.Get(value, "Issue57Node")
	require.Equal(t, ognl.Pointer, result.Type())
	assert.True(t, result.Effective())
	assert.Nil(t, result.Value())

	resultE, err := ognl.GetE(value, "Issue57Node")
	require.NoError(t, err)
	require.Equal(t, ognl.Pointer, resultE.Type())
	assert.True(t, resultE.Effective())
	assert.Nil(t, resultE.Value())
}

func TestIssue57AnonymousInterfaceNamedPointerCycleReturns(t *testing.T) {
	var pointer Issue57NamedPointerCycle
	value := issue57AnonymousInterfaceHolder{Issue57AnonymousValue: pointer}
	type outcome struct {
		result  ognl.Result
		resultE ognl.Result
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		result := ognl.Get(value, "Issue57AnonymousValue")
		resultE, err := ognl.GetE(value, "Issue57AnonymousValue")
		done <- outcome{result: result, resultE: resultE, err: err}
	}()

	select {
	case got := <-done:
		require.NoError(t, got.err)
		for _, result := range []ognl.Result{got.result, got.resultE} {
			require.Equal(t, ognl.Interface, result.Type())
			assert.True(t, result.Effective())
			actual, ok := result.Value().(Issue57NamedPointerCycle)
			require.True(t, ok)
			assert.Nil(t, actual)
		}
	case <-time.After(time.Second):
		t.Fatal("anonymous interface lookup did not return for a named pointer type cycle")
	}
}

package ognl_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const issue57PointerCycleScenario = "GO_OGNL_ISSUE57_POINTER_CYCLE_SCENARIO"

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
	if os.Getenv(issue57PointerCycleScenario) != "" {
		runIssue57AnonymousInterfaceNamedPointerCycle(t)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestIssue57AnonymousInterfaceNamedPointerCycleReturns$", "-test.count=1")
	cmd.Env = append(os.Environ(), issue57PointerCycleScenario+"=child")
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("anonymous interface lookup did not return before timeout; child output:\n%s", output)
	}
	require.NoError(t, err, "anonymous interface lookup child failed; output:\n%s", output)
}

func runIssue57AnonymousInterfaceNamedPointerCycle(t *testing.T) {
	var pointer Issue57NamedPointerCycle
	value := issue57AnonymousInterfaceHolder{Issue57AnonymousValue: pointer}
	result := ognl.Get(value, "Issue57AnonymousValue")
	resultE, err := ognl.GetE(value, "Issue57AnonymousValue")
	require.NoError(t, err)
	for _, result := range []ognl.Result{result, resultE} {
		require.Equal(t, ognl.Interface, result.Type())
		assert.True(t, result.Effective())
		actual, ok := result.Value().(Issue57NamedPointerCycle)
		require.True(t, ok)
		assert.Nil(t, actual)
	}
}

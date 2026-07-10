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

const selectorPointerCycleScenario = "GO_OGNL_POINTER_CYCLE_SCENARIO"

type selectorPointerCycle *selectorPointerCycle

type selectorStringMap map[string]string
type selectorStringMapPointer *selectorStringMap

type selectorIntMap map[int]string
type selectorIntMapPointer *selectorIntMap

func TestSelectorNamedPointerCycleCompletes(t *testing.T) {
	if scenario := os.Getenv(selectorPointerCycleScenario); scenario != "" {
		runSelectorPointerCycleScenario(t, scenario)
		return
	}

	for _, scenario := range []string{
		"top-level-get",
		"top-level-gete",
		"result-get",
		"result-gete",
		"deployed-result-get",
		"deployed-result-gete",
		"nil-top-level-get",
	} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSelectorNamedPointerCycleCompletes$", "-test.count=1")
			cmd.Env = append(os.Environ(), selectorPointerCycleScenario+"="+scenario)
			output, err := cmd.CombinedOutput()
			if ctx.Err() == context.DeadlineExceeded {
				t.Fatalf("selector API %q did not complete before timeout; child output:\n%s", scenario, output)
			}
			require.NoError(t, err, "selector API %q failed; child output:\n%s", scenario, output)
		})
	}
}

func runSelectorPointerCycleScenario(t *testing.T, scenario string) {
	var value selectorPointerCycle
	if scenario != "nil-top-level-get" {
		value = selectorPointerCycle(&value)
	}

	switch scenario {
	case "top-level-get", "nil-top-level-get":
		_ = ognl.Get(value, "x")
	case "top-level-gete":
		_, _ = ognl.GetE(value, "x")
	case "result-get":
		_ = ognl.Parse(value).Get("x")
	case "result-gete":
		_, _ = ognl.Parse(value).GetE("x")
	case "deployed-result-get":
		_ = ognl.Get([]selectorPointerCycle{value}, "#").Get("x")
	case "deployed-result-gete":
		_, _ = ognl.Get([]selectorPointerCycle{value}, "#").GetE("x")
	default:
		t.Fatalf("unknown child scenario %q", scenario)
	}
}

func TestSelectorPointerDispatchCompatibility(t *testing.T) {
	stringMap := selectorStringMap{"01": "string-key"}
	for name, value := range map[string]interface{}{
		"ordinary pointer": &stringMap,
		"named pointer":    selectorStringMapPointer(&stringMap),
	} {
		assert.Equal(t, "string-key", ognl.Get(value, "01").Value(), name)
	}

	intMap := selectorIntMap{1: "int-key"}
	for name, value := range map[string]interface{}{
		"ordinary pointer": &intMap,
		"named pointer":    selectorIntMapPointer(&intMap),
	} {
		assert.Equal(t, "int-key", ognl.Get(value, "01").Value(), name)
	}
}

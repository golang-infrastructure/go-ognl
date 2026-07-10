package ognl

import (
	"strings"
	"testing"
)

func FuzzSelectorNoPanic(f *testing.F) {
	seeds := []string{
		"",
		"escaped\\.key",
		"users#.name",
		strings.Repeat(".", 4096) + "users",
		string([]byte{0xff, '\\', '.', '#', 0xfe, 0x00}),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, selector string) {
		value := map[string]interface{}{
			"escaped.key": "escaped",
			"nil":         nil,
			"users": []interface{}{
				map[string]interface{}{"name": "alice", "tags": []string{"admin"}},
				map[string]interface{}{"name": "bob", "tags": []string{"viewer"}},
			},
		}

		results := []Result{Get(value, selector)}
		result, _ := GetE(value, selector)
		results = append(results, result)

		parsed := Parse(value)
		results = append(results, parsed.Get(selector))
		result, _ = parsed.GetE(selector)
		results = append(results, result)

		deployed := Get(value, "users#")
		results = append(results, deployed.Get(selector))
		result, _ = deployed.GetE(selector)
		results = append(results, result)

		for _, result := range results {
			_ = result.Effective()
			_ = result.Type()
			_ = result.Value()
			_ = result.Values()
			_, _ = result.ValuesE()
			_ = result.Diagnosis()
		}
	})
}

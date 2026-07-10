package ognl_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	ognl "github.com/golang-infrastructure/go-ognl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type selectorNamedString string
type selectorNamedInt int

type selectorPair struct {
	Zero string
	One  string
}

func TestSelectorGrammar_B01PathOperatorsAndIdentity(t *testing.T) {
	value := map[string]interface{}{
		"a": map[string]interface{}{"b": 1},
		"users": []interface{}{
			map[string]interface{}{"name": "alice"},
			map[string]interface{}{"name": "bob"},
		},
	}

	assert.Equal(t, value, ognl.Get(value, "").Value())
	assert.Equal(t, 1, ognl.Get(value, "...a..b...").Value())
	assert.Equal(t, []interface{}{"alice", "bob"}, ognl.Get(value, "users#.name").Values())
}

func TestSelectorGrammar_B02NumericDispatchByContainer(t *testing.T) {
	stringValues := map[string]string{
		"1":   "plain",
		"01":  "leading-zero",
		"+1":  "plus",
		"-0":  "negative-zero",
		"-00": "negative-double-zero",
	}
	namedStringValues := make(map[selectorNamedString]string, len(stringValues))
	for key, value := range stringValues {
		namedStringValues[selectorNamedString(key)] = value
	}
	for selector, want := range stringValues {
		assert.Equal(t, want, ognl.Get(stringValues, selector).Value(), "map[string], selector %q", selector)
		assert.Equal(t, want, ognl.Get(namedStringValues, selector).Value(), "named string map, selector %q", selector)
	}

	indexedCases := []struct {
		name  string
		value interface{}
	}{
		{"map[int]", map[int]string{0: "zero", 1: "one"}},
		{"named int map", map[selectorNamedInt]string{0: "zero", 1: "one"}},
		{"slice", []string{"zero", "one"}},
		{"array", [2]string{"zero", "one"}},
		{"struct", selectorPair{Zero: "zero", One: "one"}},
	}
	indexSelectors := map[string]string{
		"0":   "zero",
		"00":  "zero",
		"+0":  "zero",
		"-0":  "zero",
		"-00": "zero",
		"1":   "one",
		"01":  "one",
		"+1":  "one",
		"+01": "one",
	}
	for _, tc := range indexedCases {
		for selector, want := range indexSelectors {
			r := ognl.Get(tc.value, selector)
			assert.Equal(t, want, r.Value(), "%s, selector %q", tc.name, selector)
			assert.Equal(t, ognl.String, r.Type(), "%s, selector %q", tc.name, selector)
		}
	}
}

func TestSelectorGrammar_B03ExpandedNumericDispatchPerElement(t *testing.T) {
	value := []interface{}{
		map[string]string{"01": "string-map"},
		map[int]string{1: "int-map"},
		[]string{"slice-zero", "slice-one"},
		selectorPair{Zero: "struct-zero", One: "struct-one"},
	}

	want := []interface{}{"string-map", "int-map", "slice-one", "struct-one"}
	assert.Equal(t, want, ognl.Get(value, "#.01").Values())
	gotE, err := ognl.GetE(value, "#.01")
	require.NoError(t, err)
	assert.Equal(t, want, gotE.Values())
}

func TestSelectorGrammar_B04NonIndexTextAndOverflow(t *testing.T) {
	overflow := strings.Repeat("9", 100)
	cases := []struct {
		segment  string
		selector string
	}{
		{"-1", "-1"},
		{"-01", "-01"},
		{"1.0", `1\.0`},
		{"+", "+"},
		{"-", "-"},
		{overflow, overflow},
	}
	stringValues := make(map[string]string, len(cases))
	for _, tc := range cases {
		stringValues[tc.segment] = "value:" + tc.segment
	}

	indexedValues := []interface{}{
		map[int]string{-1: "must-not-resolve", 0: "zero"},
		[]string{"zero"},
		[1]string{"zero"},
		selectorPair{Zero: "zero", One: "one"},
	}
	for _, tc := range cases {
		assert.Equal(t, "value:"+tc.segment, ognl.Get(stringValues, tc.selector).Value(), "string selector %q", tc.selector)
		for _, value := range indexedValues {
			assert.NotPanics(t, func() {
				r := ognl.Get(value, tc.selector)
				assert.False(t, r.Effective(), "value %T, selector %q", value, tc.selector)
				_, err := ognl.GetE(value, tc.selector)
				assert.Error(t, err, "value %T, selector %q", value, tc.selector)
				assert.False(t, errors.Is(err, ognl.ErrInvalidSelector), "value %T, selector %q", value, tc.selector)
			})
		}
	}
}

func TestSelectorGrammar_B05LeadingWhitespace(t *testing.T) {
	value := map[string]string{
		"key":       "plain",
		"a b":       "space",
		"a\tb":      "tab",
		"a\nb":      "line-feed",
		"a\rb":      "carriage-return",
		" leading":  "escaped-space",
		"\tleading": "escaped-tab",
		"\vkey":     "vertical-tab",
		"\fkey":     "form-feed",
		"\u00a0key": "non-breaking-space",
	}

	for _, prefix := range []string{" ", "\t", "\n", "\r", " \t\n\r "} {
		assert.Equal(t, "plain", ognl.Get(value, prefix+"key").Value(), "prefix %q", prefix)
	}
	for selector, want := range map[string]string{
		"a b":         "space",
		"a\tb":        "tab",
		"a\nb":        "line-feed",
		"a\rb":        "carriage-return",
		"\\ leading":  "escaped-space",
		"\\\tleading": "escaped-tab",
		"\vkey":       "vertical-tab",
		"\fkey":       "form-feed",
		"\u00a0key":   "non-breaking-space",
	} {
		assert.Equal(t, want, ognl.Get(value, selector).Value(), "selector %q", selector)
	}
}

func TestSelectorGrammar_B06GeneralEscape(t *testing.T) {
	value := map[string]string{
		"q":        "ordinary",
		" leading": "leading-space",
		"界":        "unicode",
		"aqb":      "middle",
	}

	for selector, want := range map[string]string{
		`\q`:        "ordinary",
		`\ leading`: "leading-space",
		`\界`:        "unicode",
		`a\qb`:      "middle",
	} {
		assert.Equal(t, want, ognl.Get(value, selector).Value(), "selector %q", selector)
	}
}

func TestSelectorGrammar_B07ReservedLiteralEscapes(t *testing.T) {
	value := map[string]string{
		".":      "dot",
		"#":      "hash",
		"\\":     "backslash",
		"a.#\\z": "combined",
	}

	for selector, want := range map[string]string{
		`\.`:       "dot",
		`\#`:       "hash",
		`\\`:       "backslash",
		`a\.\#\\z`: "combined",
	} {
		assert.Equal(t, want, ognl.Get(value, selector).Value(), "selector %q", selector)
	}
}

func TestSelectorGrammar_B08DanglingEscapeParity(t *testing.T) {
	value := map[string]string{
		`key\`:  "one-backslash",
		`key\\`: "two-backslashes",
	}

	assert.Equal(t, "one-backslash", ognl.Get(value, `key\\`).Value())
	assert.Equal(t, "two-backslashes", ognl.Get(value, `key\\\\`).Value())
	for _, selector := range []string{`key\`, `key\\\`} {
		assert.NotPanics(t, func() {
			r, err := ognl.GetE(value, selector)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ognl.ErrInvalidSelector))
			assert.Equal(t, ognl.Invalid, r.Type())
			assert.False(t, r.Effective())
			assert.Nil(t, r.Value())
		})
	}
}

func TestSelectorGrammar_B09GetEInvalidSelectorFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		path  string
	}{
		{"partially resolved", map[string]interface{}{"ok": map[string]int{"present": 1}}, `ok.missing\`},
		{"nil input", nil, `x\`},
		{"earlier missing", map[string]int{"present": 1}, `missing.x\`},
		{"empty expansion", []interface{}{}, `#.x\`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ognl.GetE(tc.value, tc.path)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ognl.ErrInvalidSelector))
			assert.Equal(t, ognl.Invalid, r.Type())
			assert.False(t, r.Effective())
			assert.Nil(t, r.Value())
		})
	}
}

func TestSelectorGrammar_B10GetInvalidSelectorDiagnosis(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		path  string
	}{
		{"partially resolved", map[string]interface{}{"ok": map[string]int{"present": 1}}, `ok.missing\`},
		{"nil input", nil, `x\`},
		{"earlier missing", map[string]int{"present": 1}, `missing.x\`},
		{"empty expansion", []interface{}{}, `#.x\`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ognl.Get(tc.value, tc.path)
			assert.Equal(t, ognl.Invalid, r.Type())
			assert.False(t, r.Effective())
			assert.Nil(t, r.Value())
			require.NotEmpty(t, r.Diagnosis())
			assert.True(t, diagnosisContains(r.Diagnosis(), ognl.ErrInvalidSelector))
		})
	}
}

func TestSelectorGrammar_B11ResultMethodsMirrorTopLevel(t *testing.T) {
	stringMap := map[string]string{"01": "string-map", ".": "dot"}
	assert.Equal(t, "string-map", ognl.Get(stringMap, "01").Value())
	assert.Equal(t, "string-map", ognl.Parse(stringMap).Get("01").Value())
	assert.Equal(t, "dot", ognl.Get(stringMap, `\.`).Value())
	assert.Equal(t, "dot", ognl.Parse(stringMap).Get(`\.`).Value())
	topLevelE, err := ognl.GetE(stringMap, "01")
	require.NoError(t, err)
	resultE, err := ognl.Parse(stringMap).GetE("01")
	require.NoError(t, err)
	assert.Equal(t, topLevelE.Value(), resultE.Value())
	topLevelEscapeE, err := ognl.GetE(stringMap, `\.`)
	require.NoError(t, err)
	resultEscapeE, err := ognl.Parse(stringMap).GetE(`\.`)
	require.NoError(t, err)
	assert.Equal(t, topLevelEscapeE.Value(), resultEscapeE.Value())

	deployed := ognl.Get([]interface{}{
		map[string]string{"01": "string-map"},
		[]string{"zero", "slice-one"},
	}, "#")
	assert.Equal(t, []interface{}{"string-map", "slice-one"}, deployed.Get("01").Values())
	deployedE, err := deployed.GetE("01")
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"string-map", "slice-one"}, deployedE.Values())

	invalid := deployed.Get(`missing\`)
	assert.Equal(t, ognl.Invalid, invalid.Type())
	assert.Nil(t, invalid.Value())
	require.NotEmpty(t, invalid.Diagnosis())
	assert.True(t, diagnosisContains(invalid.Diagnosis(), ognl.ErrInvalidSelector))
	invalidE, err := deployed.GetE(`missing\`)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrInvalidSelector))
	assert.Equal(t, ognl.Invalid, invalidE.Type())
	assert.Nil(t, invalidE.Value())
}

func TestSelectorGrammar_B12UnicodeExactUTF8(t *testing.T) {
	nfc := "é"
	nfd := "e\u0301"
	value := map[string]string{
		nfc:         "nfc",
		nfd:         "nfd",
		"Case":      "upper",
		"case":      "lower",
		"\u00a0key": "non-breaking-space",
		"emoji-😀":   "emoji",
		"escaped-界": "escaped-unicode",
	}

	for selector, want := range map[string]string{
		nfc:          "nfc",
		nfd:          "nfd",
		"Case":       "upper",
		"case":       "lower",
		"\u00a0key":  "non-breaking-space",
		"emoji-😀":    "emoji",
		`escaped-\界`: "escaped-unicode",
	} {
		assert.Equal(t, want, ognl.Get(value, selector).Value(), "selector %q", selector)
	}
}

func TestSelectorGrammar_B13EmptyStringKeyUnavailable(t *testing.T) {
	value := map[string]interface{}{"": "empty-key", "a": 1}
	for _, selector := range []string{"", ".", "..", "...."} {
		r := ognl.Get(value, selector)
		assert.Equal(t, value, r.Value(), "selector %q", selector)
		assert.NotEqual(t, "empty-key", r.Value(), "selector %q", selector)
	}
}

func TestSelectorGrammar_B14CompatibilityAndNoAlternateSyntax(t *testing.T) {
	type namedStringKey string
	type namedIntKey int
	value := map[string]interface{}{
		"a":        map[string]interface{}{"b": "dotted"},
		"a.b":      "escaped-dot",
		"items":    []string{"zero", "one"},
		"[a]":      "bracket-bytes",
		`"a`:       map[string]string{`b"`: "quotes-are-bytes"},
		`"a.b"`:    "quotes-do-not-group",
		"expanded": []interface{}{map[string]int{"v": 1}, map[string]int{"v": 2}},
	}

	assert.Equal(t, "dotted", ognl.Get(value, "a.b").Value())
	assert.Equal(t, "escaped-dot", ognl.Get(value, `a\.b`).Value())
	assert.Equal(t, "one", ognl.Get(value, "items.1").Value())
	assert.Equal(t, "bracket-bytes", ognl.Get(value, "[a]").Value())
	assert.Equal(t, "quotes-are-bytes", ognl.Get(value, `"a.b"`).Value())
	assert.Equal(t, []interface{}{1, 2}, ognl.Get(value, "expanded#.v").Values())
	assert.Equal(t, "named-string", ognl.Get(map[namedStringKey]string{"k": "named-string"}, "k").Value())
	assert.Equal(t, "named-int", ognl.Get(map[namedIntKey]string{1: "named-int"}, "1").Value())
}

func TestSelectorGrammar_B15ResolutionErrorsAndTypesRemainDistinct(t *testing.T) {
	success := ognl.Get(map[string]int{"a": 1}, "a")
	assert.Equal(t, ognl.Int, success.Type())
	assert.True(t, success.Effective())
	assert.Equal(t, 1, success.Value())
	trailingSeparator := ognl.Get(map[string]int{"a": 1}, "a.")
	assert.Equal(t, ognl.Interface, trailingSeparator.Type())
	assert.True(t, trailingSeparator.Effective())
	assert.Equal(t, 1, trailingSeparator.Value())

	missingSource := map[string]int{"a": 1}
	missing := ognl.Get(missingSource, "missing")
	assert.Equal(t, ognl.Invalid, missing.Type())
	assert.False(t, missing.Effective())
	assert.Nil(t, missing.Value())
	assert.Empty(t, missing.Diagnosis())
	missingE, err := ognl.GetE(missingSource, "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrInvalidValue))
	assert.False(t, errors.Is(err, ognl.ErrInvalidSelector))
	assert.Equal(t, ognl.Invalid, missingE.Type())
	assert.False(t, missingE.Effective())
	assert.Nil(t, missingE.Value())

	outOfRangeSource := []int{1}
	outOfRange := ognl.Get(outOfRangeSource, "2")
	assert.Equal(t, ognl.Invalid, outOfRange.Type())
	assert.False(t, outOfRange.Effective())
	require.Len(t, outOfRange.Diagnosis(), 1)
	assert.True(t, errors.Is(outOfRange.Diagnosis()[0], ognl.ErrIndexOutOfBounds))
	outOfRangeE, err := ognl.GetE(outOfRangeSource, "2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrIndexOutOfBounds))
	assert.False(t, errors.Is(err, ognl.ErrInvalidSelector))
	assert.Equal(t, ognl.Invalid, outOfRangeE.Type())
	assert.False(t, outOfRangeE.Effective())
	assert.Nil(t, outOfRangeE.Value())

	_, err = ognl.GetE(1, "field")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrInvalidStructure))
	assert.False(t, errors.Is(err, ognl.ErrInvalidSelector))

	overflow := strings.Repeat("9", 100)
	overflowResult := ognl.Get(outOfRangeSource, overflow)
	assert.Equal(t, ognl.Invalid, overflowResult.Type())
	require.Len(t, overflowResult.Diagnosis(), 1)
	assert.True(t, errors.Is(overflowResult.Diagnosis()[0], ognl.ErrSliceSubscript))
	_, err = ognl.GetE(outOfRangeSource, overflow)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrSliceSubscript))
	assert.False(t, errors.Is(err, ognl.ErrInvalidSelector))

	nilSource := map[string]interface{}{"k": nil}
	adjacentExpansion, err := ognl.GetE(nilSource, "k#")
	require.NoError(t, err)
	assert.Equal(t, ognl.Invalid, adjacentExpansion.Type())
	assert.False(t, adjacentExpansion.Effective())
	separatedExpansion, err := ognl.GetE(nilSource, "k.#")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ognl.ErrInvalidValue))
	assert.False(t, errors.Is(err, ognl.ErrInvalidSelector))
	assert.Equal(t, ognl.Invalid, separatedExpansion.Type())
}

func TestSelectorGrammar_GetEFailedElementsDoNotReenterResults(t *testing.T) {
	values := []interface{}{
		map[string]string{"target": "found"},
		[]int{99},
	}

	topLevel, err := ognl.GetE(values, "#.target")
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"found"}, topLevel.Values())
	require.Len(t, topLevel.Diagnosis(), 1)
	assert.True(t, errors.Is(topLevel.Diagnosis()[0], ognl.ErrSliceSubscript))

	expanded := ognl.Get(values, "#")
	require.True(t, expanded.Effective())
	chained, err := expanded.GetE("target")
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"found"}, chained.Values())
	require.Len(t, chained.Diagnosis(), 1)
	assert.True(t, errors.Is(chained.Diagnosis()[0], ognl.ErrSliceSubscript))
}

func FuzzSelectorGrammar(f *testing.F) {
	for _, seed := range []string{"", ".", "#", `a\.b`, `key\`, "\u00a0key", strings.Repeat(".", 1024)} {
		f.Add(seed)
	}
	value := struct {
		A     string
		Items []string
	}{A: "value", Items: []string{"zero", "one"}}
	f.Fuzz(func(t *testing.T, selector string) {
		got1 := ognl.Get(value, selector)
		got2 := ognl.Get(value, selector)
		if got1.Type() != got2.Type() || got1.Effective() != got2.Effective() || !reflect.DeepEqual(got1.Value(), got2.Value()) {
			t.Fatalf("Get is not deterministic for selector %q", selector)
		}

		gotE1, err1 := ognl.GetE(value, selector)
		gotE2, err2 := ognl.GetE(value, selector)
		if gotE1.Type() != gotE2.Type() || gotE1.Effective() != gotE2.Effective() || !reflect.DeepEqual(gotE1.Value(), gotE2.Value()) || errorText(err1) != errorText(err2) {
			t.Fatalf("GetE is not deterministic for selector %q", selector)
		}
	})
}

func BenchmarkSelectorGrammar(b *testing.B) {
	value := struct{ A int }{A: 1}
	for _, size := range []int{1 << 10, 16 << 10, 256 << 10} {
		selector := strings.Repeat(".", size-1) + "A"
		b.Run(fmt.Sprintf("%dKiB", size>>10), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ognl.Get(value, selector)
			}
		})
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func diagnosisContains(diagnosis []error, target error) bool {
	for _, err := range diagnosis {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

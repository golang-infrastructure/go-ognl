package ognl

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue33Leaf struct{}

type issue33Inner struct {
	X []int
}

type issue33Root struct {
	I issue33Inner
}

func issue33Context(typeToken string, offset, operation, totalLen uint64, sentinel error) string {
	return fmt.Sprintf("type=%s,offset=%d,op=%d,total_len=%d: %s", typeToken, offset, operation, totalLen, sentinel)
}

func TestIssue33ActualFailureObjectType(t *testing.T) {
	typedNil := (*issue33Leaf)(nil)
	tests := []struct {
		name      string
		value     interface{}
		path      string
		typeToken string
		offset    uint64
		operation uint64
		sentinel  error
		diagnosed bool
	}{
		{
			name:      "resolved slice",
			value:     issue33Root{},
			path:      "I.X.nope",
			typeToken: `"[]int"`,
			offset:    4,
			operation: 2,
			sentinel:  ErrSliceSubscript,
			diagnosed: true,
		},
		{
			name:      "untyped nil",
			value:     map[string]interface{}{"x": nil},
			path:      "x.missing",
			typeToken: `"<nil>"`,
			offset:    2,
			operation: 1,
			sentinel:  ErrInvalidValue,
			diagnosed: true,
		},
		{
			name:      "typed nil",
			value:     map[string]interface{}{"x": typedNil},
			path:      "x.missing",
			typeToken: strconv.QuoteToASCII(reflect.TypeOf(typedNil).String()),
			offset:    2,
			operation: 1,
			sentinel:  ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetE(tt.value, tt.path)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.sentinel)
			assert.Equal(t, issue33Context(tt.typeToken, tt.offset, tt.operation, uint64(len(tt.path)), tt.sentinel), err.Error())
			assert.False(t, result.Effective())

			getResult := Get(tt.value, tt.path)
			if tt.diagnosed {
				require.NotEmpty(t, getResult.Diagnosis())
				assert.True(t, errors.Is(getResult.Diagnosis()[0], tt.sentinel))
				assert.Equal(t, err.Error(), getResult.Diagnosis()[0].Error())
			} else {
				assert.Empty(t, getResult.Diagnosis())
			}
		})
	}
}

func TestIssue33FirstExpansionInvalidContext(t *testing.T) {
	typedNil := (*issue33Leaf)(nil)
	typeToken := strconv.QuoteToASCII(reflect.TypeOf(typedNil).String())
	tests := []struct {
		name      string
		value     interface{}
		path      string
		offset    uint64
		operation uint64
	}{
		{name: "direct", value: typedNil, path: "#", offset: 0, operation: 0},
		{name: "nested", value: map[string]interface{}{"x": typedNil}, path: "x#", offset: 1, operation: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := issue33Context(typeToken, tt.offset, tt.operation, uint64(len(tt.path)), ErrInvalidValue)

			t.Run("GetE", func(t *testing.T) {
				result, err := GetE(tt.value, tt.path)
				require.EqualError(t, err, want)
				assert.ErrorIs(t, err, ErrInvalidValue)
				assert.False(t, result.Effective())
			})

			t.Run("Get", func(t *testing.T) {
				result := Get(tt.value, tt.path)
				assert.False(t, result.Effective())
				require.Len(t, result.Diagnosis(), 1)
				assert.Equal(t, want, result.Diagnosis()[0].Error())
				assert.ErrorIs(t, result.Diagnosis()[0], ErrInvalidValue)
			})

			t.Run("Result methods", func(t *testing.T) {
				parsed := Parse(tt.value)
				result, err := parsed.GetE(tt.path)
				require.EqualError(t, err, want)
				assert.False(t, result.Effective())

				getResult := parsed.Get(tt.path)
				assert.False(t, getResult.Effective())
				require.Len(t, getResult.Diagnosis(), 1)
				assert.Equal(t, want, getResult.Diagnosis()[0].Error())
			})
		})
	}

	t.Run("chained Result uses fresh origin", func(t *testing.T) {
		resolved, err := GetE(map[string]interface{}{"x": typedNil}, "x")
		require.NoError(t, err)
		want := issue33Context(typeToken, 0, 0, 1, ErrInvalidValue)

		result, err := resolved.GetE("#")
		require.EqualError(t, err, want)
		assert.False(t, result.Effective())

		getResult := resolved.Get("#")
		assert.False(t, getResult.Effective())
		require.Len(t, getResult.Diagnosis(), 1)
		assert.Equal(t, want, getResult.Diagnosis()[0].Error())
	})
}

func TestIssue33DereferencedFailureType(t *testing.T) {
	leaf := []int{}
	tests := []struct {
		name      string
		value     interface{}
		path      string
		offset    uint64
		operation uint64
		sentinel  error
		deployed  bool
	}{
		{name: "direct key", value: &leaf, path: "missing", offset: 0, operation: 0, sentinel: ErrSliceSubscript},
		{name: "direct index", value: &leaf, path: "0", offset: 0, operation: 0, sentinel: ErrIndexOutOfBounds},
		{name: "nested key", value: map[string]interface{}{"x": &leaf}, path: "x.missing", offset: 2, operation: 1, sentinel: ErrSliceSubscript},
		{name: "deployed direct key", value: []interface{}{&leaf}, path: "#missing", offset: 1, operation: 1, sentinel: ErrSliceSubscript, deployed: true},
		{name: "deployed direct index", value: []interface{}{&leaf}, path: "#0", offset: 1, operation: 1, sentinel: ErrIndexOutOfBounds, deployed: true},
		{name: "deployed recursive key", value: []interface{}{&leaf}, path: "#.missing", offset: 2, operation: 1, sentinel: ErrSliceSubscript, deployed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := issue33Context(`"[]int"`, tt.offset, tt.operation, uint64(len(tt.path)), tt.sentinel)

			result, err := GetE(tt.value, tt.path)
			require.EqualError(t, err, want)
			assert.ErrorIs(t, err, tt.sentinel)
			assert.False(t, result.Effective())
			if tt.deployed {
				require.Len(t, result.Diagnosis(), 1)
				assert.Equal(t, want, result.Diagnosis()[0].Error())
			}

			getResult := Get(tt.value, tt.path)
			require.Len(t, getResult.Diagnosis(), 1)
			assert.Equal(t, want, getResult.Diagnosis()[0].Error())
			assert.ErrorIs(t, getResult.Diagnosis()[0], tt.sentinel)

			parsed := Parse(tt.value)
			methodResult, methodErr := parsed.GetE(tt.path)
			require.EqualError(t, methodErr, want)
			assert.False(t, methodResult.Effective())
			if tt.deployed {
				require.Len(t, methodResult.Diagnosis(), 1)
				assert.Equal(t, want, methodResult.Diagnosis()[0].Error())
			}
			methodGetResult := parsed.Get(tt.path)
			require.Len(t, methodGetResult.Diagnosis(), 1)
			assert.Equal(t, want, methodGetResult.Diagnosis()[0].Error())
		})
	}
}

func TestIssue33OriginalSelectorLocationFields(t *testing.T) {
	value := map[string]interface{}{
		"π": []issue33Inner{{}},
	}
	path := "π#.X.nope"
	want := issue33Context(`"[]int"`, 6, 3, uint64(len(path)), ErrSliceSubscript)

	_, err := GetE(value, path)
	require.EqualError(t, err, want)
	assert.Equal(t, want, Get(value, path).Diagnosis()[0].Error())
}

func TestIssue33OperationAccounting(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		path      string
		typeToken string
		offset    uint64
		operation uint64
		sentinel  error
	}{
		{
			name:      "plain key",
			value:     map[string]int{},
			path:      "missing",
			typeToken: `"map[string]int"`,
			offset:    0,
			operation: 0,
			sentinel:  ErrInvalidValue,
		},
		{
			name:      "numeric index",
			value:     [][]int{{}},
			path:      "0.5",
			typeToken: `"[]int"`,
			offset:    2,
			operation: 1,
			sentinel:  ErrIndexOutOfBounds,
		},
		{
			name:      "separators do not count",
			value:     map[string]interface{}{"a": []int{}},
			path:      "a...missing",
			typeToken: `"[]int"`,
			offset:    4,
			operation: 1,
			sentinel:  ErrSliceSubscript,
		},
		{
			name:      "unescaped expansion",
			value:     []interface{}{[]int{}},
			path:      "#.5",
			typeToken: `"[]int"`,
			offset:    2,
			operation: 1,
			sentinel:  ErrIndexOutOfBounds,
		},
		{
			name:      "operation begins with escape",
			value:     map[string]int{},
			path:      `\.`,
			typeToken: `"map[string]int"`,
			offset:    0,
			operation: 0,
			sentinel:  ErrInvalidValue,
		},
		{
			name:      "later escape keeps operation start",
			value:     map[string]int{},
			path:      `a\.b`,
			typeToken: `"map[string]int"`,
			offset:    0,
			operation: 0,
			sentinel:  ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetE(tt.value, tt.path)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.sentinel)
			assert.Equal(t, issue33Context(tt.typeToken, tt.offset, tt.operation, uint64(len(tt.path)), tt.sentinel), err.Error())
		})
	}
}

type issue33StringerValue struct {
	value string
}

func (v issue33StringerValue) String() string {
	return v.value
}

type issue33ErrorValue struct {
	value string
}

func (v issue33ErrorValue) Error() string {
	return v.value
}

type issue33RedactionObject struct {
	Secret   string
	Stringer issue33StringerValue
	Err      issue33ErrorValue
	Items    []int
}

func TestIssue33RedactsSelectorKeysAndObjectValues(t *testing.T) {
	const (
		prefixKey     = "ISSUE33_PREFIX.CANARY"
		failingKey    = "ISSUE33_FAILING#CANARY"
		objectCanary  = "ISSUE33_OBJECT_VALUE_CANARY"
		stringCanary  = "ISSUE33_STRINGER_VALUE_CANARY"
		errorCanary   = "ISSUE33_ERROR_VALUE_CANARY"
		encodedPrefix = `ISSUE33_PREFIX\.CANARY`
		encodedFail   = `ISSUE33_FAILING\#CANARY`
	)
	path := encodedPrefix + ".Items." + encodedFail
	value := map[string]interface{}{
		prefixKey: issue33RedactionObject{
			Secret:   objectCanary,
			Stringer: issue33StringerValue{value: stringCanary},
			Err:      issue33ErrorValue{value: errorCanary},
			Items:    []int{},
		},
	}

	_, err := GetE(value, path)
	require.Error(t, err)
	getResult := Get(value, path)
	require.Len(t, getResult.Diagnosis(), 1)

	for _, message := range []string{err.Error(), getResult.Diagnosis()[0].Error()} {
		for _, canary := range []string{
			path,
			prefixKey,
			failingKey,
			encodedPrefix,
			encodedFail,
			objectCanary,
			stringCanary,
			errorCanary,
		} {
			assert.NotContains(t, message, canary)
		}
	}
}

func TestIssue33TypeTokenEncodingAndLimit(t *testing.T) {
	short := "type\"\\\n\t\x00界"
	assert.Equal(t, strconv.QuoteToASCII(short), encodeTypeToken(short, maxContextTypeTokenLen))

	raw := short + strings.Repeat("长", 200)
	encoded := encodeTypeToken(raw, maxContextTypeTokenLen)
	assert.LessOrEqual(t, len(encoded), maxContextTypeTokenLen)
	for i := 0; i < len(encoded); i++ {
		assert.Less(t, encoded[i], byte(0x80), "byte %d was not ASCII", i)
	}
	assert.Contains(t, encoded, `\"`)
	assert.Contains(t, encoded, `\\`)
	assert.Contains(t, encoded, `\n`)
	assert.Contains(t, encoded, `\t`)
	assert.Contains(t, encoded, `\x00`)
	assert.Contains(t, encoded, `\u754c`)
	assert.Contains(t, encoded, contextTruncatedMarker)
	decoded, err := strconv.Unquote(encoded)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(decoded, contextTruncatedMarker))
}

func TestIssue33InternalErrorStringLimit(t *testing.T) {
	maxUint64 := ^uint64(0)
	typeName := strings.Repeat("a", 400)
	initialToken := encodeTypeToken(typeName, maxContextTypeTokenLen)
	require.Len(t, initialToken, maxContextTypeTokenLen)
	location := resolutionLocation{offset: maxUint64, operation: maxUint64, totalLen: maxUint64}
	suffix := fmt.Sprintf(",offset=%d,op=%d,total_len=%d: %s", maxUint64, maxUint64, maxUint64, ErrInvalidStructure)
	uncapped := "type=" + initialToken + suffix
	require.Len(t, uncapped, 375)

	contextual := &resolutionError{err: ErrInvalidStructure, typeName: typeName, location: location}
	message := contextual.Error()
	assert.Len(t, message, maxContextErrorLen)
	assert.True(t, utf8.ValidString(message))
	assert.True(t, strings.HasSuffix(message, ErrInvalidStructure.Error()))
	assert.Contains(t, message, fmt.Sprintf(",offset=%d,op=%d,total_len=%d: ", maxUint64, maxUint64, maxUint64))
	assert.ErrorIs(t, contextual, ErrInvalidStructure)

	separator := strings.Index(message, ",offset=")
	require.Positive(t, separator)
	typeToken := strings.TrimPrefix(message[:separator], "type=")
	assert.Contains(t, typeToken, contextTruncatedMarker)
	decoded, err := strconv.Unquote(typeToken)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(decoded, contextTruncatedMarker))
}

func TestIssue33GetAndGetEContextParity(t *testing.T) {
	value := []int{}
	path := "missing"
	_, err := GetE(value, path)
	require.Error(t, err)
	result := Get(value, path)
	require.Len(t, result.Diagnosis(), 1)
	assert.Equal(t, err.Error(), result.Diagnosis()[0].Error())
}

func TestIssue33ContextPreservesErrorsIs(t *testing.T) {
	sentinels := []error{
		ErrInvalidStructure,
		ErrSliceSubscript,
		ErrMapKeyMustString,
		ErrMapKeyMustInt,
		ErrIndexOutOfBounds,
		ErrStructIndexOutOfBounds,
		ErrParseInt,
		ErrUnableExpand,
		ErrInvalidValue,
		ErrExpansionLimit,
		ErrInvalidSelector,
	}
	for _, sentinel := range sentinels {
		t.Run(sentinel.Error(), func(t *testing.T) {
			contextual := wrapResolutionError(sentinel, reflect.TypeOf(0), resolutionLocation{offset: 1, operation: 2, totalLen: 3})
			assert.ErrorIs(t, contextual, sentinel)
		})
	}
}

type issue33ExpansionSecret struct {
	Secret string
}

func TestIssue33ExpansionLimitContext(t *testing.T) {
	const (
		selectorCanary = "ISSUE33_EXPANSION_SELECTOR_CANARY"
		objectCanary   = "ISSUE33_EXPANSION_OBJECT_CANARY"
	)
	overLimit := make([]issue33ExpansionSecret, 10_001)
	overLimit[0].Secret = objectCanary
	input := map[string]interface{}{selectorCanary: overLimit}
	directPath := selectorCanary + "#"

	assertContext := func(t *testing.T, contextual error, path string, offset, operation uint64) {
		t.Helper()
		require.Error(t, contextual)
		assert.ErrorIs(t, contextual, ErrExpansionLimit)
		assert.Contains(t, contextual.Error(), fmt.Sprintf(",offset=%d,op=%d,total_len=%d: ", offset, operation, len(path)))
		assert.LessOrEqual(t, len(contextual.Error()), maxContextErrorLen)
		assert.True(t, utf8.ValidString(contextual.Error()))
		assert.Equal(t, 1, strings.Count(contextual.Error(), "type="))
		assert.NotContains(t, contextual.Error(), selectorCanary)
		assert.NotContains(t, contextual.Error(), objectCanary)
	}
	assertGet := func(t *testing.T, result Result, path string, offset, operation uint64) {
		t.Helper()
		assert.False(t, result.Effective())
		assert.Empty(t, result.Values())
		require.Len(t, result.Diagnosis(), 1)
		assertContext(t, result.Diagnosis()[0], path, offset, operation)
	}
	assertGetE := func(t *testing.T, result Result, err error, path string, offset, operation uint64) {
		t.Helper()
		assert.False(t, result.Effective())
		assert.Empty(t, result.Values())
		assertContext(t, err, path, offset, operation)
	}

	t.Run("top-level", func(t *testing.T) {
		offset := uint64(len(selectorCanary))
		assertGet(t, Get(input, directPath), directPath, offset, 1)
		result, err := GetE(input, directPath)
		assertGetE(t, result, err, directPath, offset, 1)
	})

	t.Run("top-level expanded branch", func(t *testing.T) {
		path := "#." + directPath
		offset := uint64(len("#.") + len(selectorCanary))
		assertGet(t, Get([]interface{}{input}, path), path, offset, 2)
		result, err := GetE([]interface{}{input}, path)
		assertGetE(t, result, err, path, offset, 2)
	})

	t.Run("expanded Result methods", func(t *testing.T) {
		expanded := Get([]interface{}{input}, "#")
		require.True(t, expanded.Effective())
		offset := uint64(len(selectorCanary))
		assertGet(t, expanded.Get(directPath), directPath, offset, 1)
		result, err := expanded.GetE(directPath)
		assertGetE(t, result, err, directPath, offset, 1)
	})
}

func TestIssue33ResultMethodParityAndFreshOrigin(t *testing.T) {
	value := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": []int{},
		},
	}
	inner, err := GetE(value, "outer.inner")
	require.NoError(t, err)

	_, methodErr := inner.GetE("missing")
	require.Error(t, methodErr)
	_, topLevelErr := GetE([]int{}, "missing")
	require.Error(t, topLevelErr)
	assert.Equal(t, topLevelErr.Error(), methodErr.Error())
	assert.Equal(t, issue33Context(`"[]int"`, 0, 0, 7, ErrSliceSubscript), methodErr.Error())

	methodResult := inner.Get("missing")
	topLevelResult := Get([]int{}, "missing")
	require.Len(t, methodResult.Diagnosis(), 1)
	require.Len(t, topLevelResult.Diagnosis(), 1)
	assert.Equal(t, topLevelResult.Diagnosis()[0].Error(), methodResult.Diagnosis()[0].Error())
}

func TestIssue33DeployedContextAnchoredOnce(t *testing.T) {
	value := []issue33Inner{{}}
	path := "#.X.missing"
	want := issue33Context(`"[]int"`, 4, 2, uint64(len(path)), ErrSliceSubscript)

	result, err := GetE(value, path)
	require.EqualError(t, err, want)
	require.Len(t, result.Diagnosis(), 1)
	assert.Equal(t, want, result.Diagnosis()[0].Error())
	assert.Equal(t, 1, strings.Count(err.Error(), "type="))
	assert.Equal(t, 1, strings.Count(result.Diagnosis()[0].Error(), "type="))
}

func TestIssue33PartialDeploymentDiagnostics(t *testing.T) {
	value := []interface{}{
		map[string]int{"ok": 1},
		[]int{},
	}
	path := "#.ok"
	wantDiagnosis := issue33Context(`"[]int"`, 2, 1, uint64(len(path)), ErrSliceSubscript)

	result, err := GetE(value, path)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{1}, result.Values())
	require.Len(t, result.Diagnosis(), 1)
	assert.Equal(t, wantDiagnosis, result.Diagnosis()[0].Error())

	getResult := Get(value, path)
	assert.Equal(t, []interface{}{1}, getResult.Values())
	require.Len(t, getResult.Diagnosis(), 1)
	assert.Equal(t, wantDiagnosis, getResult.Diagnosis()[0].Error())
}

func TestIssue33AllDeploymentFailuresSelectFirstContext(t *testing.T) {
	value := []interface{}{
		[]int{},
		map[int]int{},
		42,
	}
	path := "#.missing"
	want := []struct {
		typeToken string
		sentinel  error
	}{
		{typeToken: `"[]int"`, sentinel: ErrSliceSubscript},
		{typeToken: `"map[int]int"`, sentinel: ErrMapKeyMustString},
		{typeToken: `"int"`, sentinel: ErrInvalidStructure},
	}

	result, err := GetE(value, path)
	require.Error(t, err)
	assert.False(t, result.Effective())
	assert.Empty(t, result.Values())
	require.Len(t, result.Diagnosis(), len(want))
	for i, expected := range want {
		message := issue33Context(expected.typeToken, 2, 1, uint64(len(path)), expected.sentinel)
		assert.Equal(t, message, result.Diagnosis()[i].Error())
		assert.ErrorIs(t, result.Diagnosis()[i], expected.sentinel)
	}
	assert.Equal(t, result.Diagnosis()[0].Error(), err.Error())
	assert.ErrorIs(t, err, want[0].sentinel)
}

func TestIssue33HostileValidSelectorsAndMissingResolution(t *testing.T) {
	const longCanary = "ISSUE33_SELECTOR_SECRET_CANARY"
	longPath := strings.Repeat(longCanary, (1<<20)/len(longCanary))
	longPath += strings.Repeat("X", (1<<20)-len(longPath))
	require.Len(t, longPath, 1<<20)

	tests := []struct {
		name     string
		path     string
		canaries []string
	}{
		{name: "escaped", path: `ISSUE33_ESCAPED\.SELECTOR`, canaries: []string{`ISSUE33_ESCAPED\.SELECTOR`, "ISSUE33_ESCAPED.SELECTOR"}},
		{name: "unicode", path: "秘密字段", canaries: []string{"秘密字段"}},
		{name: "escaped control", path: "ISSUE33_CONTROL\\\nSELECTOR", canaries: []string{"ISSUE33_CONTROL", "SELECTOR"}},
		{name: "one mebibyte", path: longPath, canaries: []string{longCanary}},
		{name: "ordinary terminal missing", path: "ordinary-terminal-missing-canary", canaries: []string{"ordinary-terminal-missing-canary"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := map[string]int{}
			var getEError error
			var getResult Result
			assert.NotPanics(t, func() {
				_, getEError = GetE(value, tt.path)
				getResult = Get(value, tt.path)
			})
			require.Error(t, getEError)
			assert.False(t, getResult.Effective())
			assert.Empty(t, getResult.Diagnosis(), "valid map misses preserve selector-contract B15")
			assert.ErrorIs(t, getEError, ErrInvalidValue)
			assert.LessOrEqual(t, len(getEError.Error()), maxContextErrorLen)
			assert.True(t, utf8.ValidString(getEError.Error()))
			for _, canary := range tt.canaries {
				assert.NotContains(t, getEError.Error(), canary)
			}
		})
	}
}

func FuzzIssue33ErrorContextSafety(f *testing.F) {
	for _, seed := range []string{"missing", `escaped\.key`, "秘密", "#.#.missing", "control\nkey"} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, selectorBytes []byte) {
		const (
			selectorCanary = "ISSUE33_FUZZ_SELECTOR_CANARY"
			objectCanary   = "ISSUE33_FUZZ_OBJECT_CANARY"
		)
		value := map[string]interface{}{
			"safe": issue33RedactionObject{Secret: objectCanary},
		}
		parsed := Parse(value)
		selectors := []string{string(selectorBytes), selectorCanary + string(selectorBytes)}
		for _, selector := range selectors {
			getResult := Get(value, selector)
			getEResult, getEError := GetE(value, selector)
			methodResult := parsed.Get(selector)
			methodEResult, methodEError := parsed.GetE(selector)
			errorsToCheck := []error{getEError, methodEError}
			errorsToCheck = append(errorsToCheck, getResult.Diagnosis()...)
			errorsToCheck = append(errorsToCheck, getEResult.Diagnosis()...)
			errorsToCheck = append(errorsToCheck, methodResult.Diagnosis()...)
			errorsToCheck = append(errorsToCheck, methodEResult.Diagnosis()...)
			for _, contextual := range errorsToCheck {
				if contextual == nil || !issue33HasPackageSentinel(contextual) {
					continue
				}
				if len(contextual.Error()) > maxContextErrorLen {
					t.Fatalf("context error length = %d", len(contextual.Error()))
				}
				if !utf8.ValidString(contextual.Error()) {
					t.Fatalf("context error was not valid UTF-8")
				}
				if strings.Contains(contextual.Error(), objectCanary) {
					t.Fatalf("context error disclosed object canary")
				}
				if strings.Contains(selector, selectorCanary) && strings.Contains(contextual.Error(), selectorCanary) {
					t.Fatalf("context error disclosed selector canary")
				}
			}
		}
	})
}

func TestIssue33ConcurrentContextIsolation(t *testing.T) {
	value := map[string]interface{}{"a": []int{}}
	parsed := Parse(value)
	paths := []string{"a.x", "..a.longer", "a...last"}
	failures := make(chan string, 64)
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		for _, path := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				operationOffset := uint64(strings.LastIndex(path, ".") + 1)
				want := issue33Context(`"[]int"`, operationOffset, 1, uint64(len(path)), ErrSliceSubscript)
				for iteration := 0; iteration < 50; iteration++ {
					_, topError := GetE(value, path)
					topGet := Get(value, path)
					_, methodError := parsed.GetE(path)
					methodGet := parsed.Get(path)
					if topError == nil || topError.Error() != want {
						failures <- fmt.Sprintf("GetE(%q) = %v", path, topError)
						return
					}
					for name, diagnosis := range map[string][]error{
						"Get":        topGet.Diagnosis(),
						"Result.Get": methodGet.Diagnosis(),
					} {
						if len(diagnosis) != 1 || diagnosis[0].Error() != want {
							failures <- fmt.Sprintf("%s(%q) = %v", name, path, diagnosis)
							return
						}
					}
					if methodError == nil || methodError.Error() != want {
						failures <- fmt.Sprintf("Result.GetE(%q) = %v", path, methodError)
						return
					}
				}
			}(path)
		}
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

func issue33HasPackageSentinel(err error) bool {
	for _, sentinel := range []error{
		ErrInvalidStructure,
		ErrSliceSubscript,
		ErrMapKeyMustString,
		ErrMapKeyMustInt,
		ErrIndexOutOfBounds,
		ErrStructIndexOutOfBounds,
		ErrParseInt,
		ErrUnableExpand,
		ErrInvalidValue,
		ErrExpansionLimit,
		ErrInvalidSelector,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

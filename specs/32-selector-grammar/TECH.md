# Selector Grammar Technical Specification

## Context

The user-visible contract is defined in [PRODUCT.md](./PRODUCT.md). This document records the implementation architecture and verification contract that ship with it.

The implementation baseline was `origin/main@3b0a02a`:

- `README.md:71-75` and package documentation at `ognl.go:6-15` describe dots, expansion, numeric indices, and escaped dots, but do not define whitespace, literal hashes/backslashes, invalid escapes, Unicode, or the empty-key limitation.
- `getE` and `get` each interpret separator bytes directly and independently at `ognl.go:342-474` and `ognl.go:514-625`.
- Both walkers call `strconv.Atoi` before inspecting the current container at `ognl.go:424-428` and `ognl.go:579-583`. This makes a numeric-looking segment select the integer path even for a string-keyed map.
- `parseNextKey` at `ognl.go:631-660` consumes any escaped byte, but silently drops a final unmatched backslash.
- Container resolution is split between `parseString` and `parseInt` beginning at `ognl.go:681`; this is the correct layer for container-kind-aware segment dispatch after the selector has been parsed.
- Existing compatibility evidence includes ordinary and numeric paths in `ognl_test.go:76-96`, escaped-dot paths in `ognl_test.go:426-445`, expansion paths in `verify_test.go:64-80`, named string/int keys in `verify_test.go:82-93`, and long-path/no-panic coverage in `verify_test.go:19-44`.

At that baseline, the two public walkers duplicated grammar decisions. Changing only one walker, or reparsing separately for each expanded element, would have risked Get/GetE drift and repeated selector work; the implemented design below removes those two failure modes.

## Implemented design

1. The exported sentinel required by B08-B11 lives alongside the existing stable errors.
2. A private single-pass scanner replaces `parseNextKey` for B01 and B05-B08 while preserving B12-B14. Its private tokens retain segment, dot-separator, and expansion boundaries needed by the existing traversal semantics.
3. Each `Get`, `GetE`, `Result.Get`, and `Result.GetE` call parses and validates once, then reuses the tokens during traversal and expansion for B09-B11 and B15.
4. Both traversal variants use one shared, container-aware segment resolver for B02-B04 and B15.
5. All four entry points centralize selector-validation failure construction for B08-B11, distinct from resolution failures under B15.
6. README and package/API godoc match B01-B15. The implementation adds no public parser, parser option, or alternate lookup entry point.

## Testing and validation

All contract tests live in the external `ognl_test` package and exercise exported APIs. Each retained Behavior has exactly one primary test and one behavior-specific mutation:

| Behavior | Primary test | Required coverage | Behavior-specific mutation that must make the primary test fail |
| --- | --- | --- | --- |
| B01 | `TestSelectorGrammar_B01PathOperatorsAndIdentity` | Empty-selector identity, dotted paths, expansion, and leading/trailing/repeated dots. | Treat an unescaped `#` as ordinary segment text instead of expansion. |
| B02 | `TestSelectorGrammar_B02NumericDispatchByContainer` | Every supported string-keyed and index-capable container, including named key kinds and every accepted compatibility spelling. | Dispatch numeric-looking text before inspecting the current container kind. |
| B03 | `TestSelectorGrammar_B03ExpandedNumericDispatchPerElement` | One expanded heterogeneous value in which the same segment resolves under different container rules. | Cache the first expanded element's dispatch mode and reuse it for later elements. |
| B04 | `TestSelectorGrammar_B04NonIndexTextAndOverflow` | Negative non-zero, decimal-looking, sign-only, and overflow text across string-keyed and index-capable containers. | Accept a negative non-zero segment as a positional index. |
| B05 | `TestSelectorGrammar_B05LeadingWhitespace` | The exact ignorable leading set, internal/trailing whitespace, escaped leading whitespace, and other whitespace. | Ignore one of the four ASCII whitespace characters after a segment has started. |
| B06 | `TestSelectorGrammar_B06GeneralEscape` | Escapes before ordinary ASCII, leading whitespace, Unicode, and multiple escaped characters. | Restrict escape handling to dot, hash, and backslash. |
| B07 | `TestSelectorGrammar_B07ReservedLiteralEscapes` | Literal dot, hash, backslash, combined escapes, and a valid trailing literal backslash. | Leave `\\` encoded as two backslashes instead of decoding it to one. |
| B08 | `TestSelectorGrammar_B08DanglingEscapeParity` | Odd and even trailing backslash runs, sentinel identity, and panic freedom. | Silently discard a final unmatched backslash. |
| B09 | `TestSelectorGrammar_B09GetEInvalidSelectorFailsClosed` | Invalid-selector precedence over nil, missing, partially resolved, and empty-expanded inputs through `GetE`. | Traverse before validating so an earlier resolution outcome masks the syntax error. |
| B10 | `TestSelectorGrammar_B10GetInvalidSelectorDiagnosis` | The complete invalid-selector contract through `Get` over the same input states as B09. | Build the `Get` diagnosis without an `ErrInvalidSelector` error chain. |
| B11 | `TestSelectorGrammar_B11ResultMethodsMirrorTopLevel` | Representative numeric, escape, and invalid cases through deployed and non-deployed `Result` methods. | Keep `Result.GetE` on the legacy parser while top-level methods use the shared parser. |
| B12 | `TestSelectorGrammar_B12UnicodeExactUTF8` | NFC/NFD distinctions, case variants, non-ASCII whitespace, emoji, and escaped Unicode. | Normalize segment text to NFC before lookup. |
| B13 | `TestSelectorGrammar_B13EmptyStringKeyUnavailable` | Empty and repeated-dot selectors against a string map containing an empty key. | Convert an empty segment between repeated dots into an empty-key lookup. |
| B14 | `TestSelectorGrammar_B14CompatibilityAndNoAlternateSyntax` | Existing successful selector matrix plus quotes and brackets as ordinary bytes. | Give quotes grouping semantics that protect a dot from segment splitting. |
| B15 | `TestSelectorGrammar_B15ResolutionErrorsAndTypesRemainDistinct` | Successful, missing, incompatible, out-of-range, and overflow resolution through both APIs, including Invalid/ineffective/nil `GetE` failures. | Retain the pre-resolution raw value and Interface type when `GetE` resolution fails. |

Against `origin/main@3b0a02a`, the primary-test baseline was RED for B02, B03, B08-B11, and the current B15 failure-state contract; it was GREEN for B01 and B04-B07 and B12-B14. B04 uses `1\.0` to express decoded segment text `1.0`; the unescaped dot remains a B01 separator. The B08-B11 baseline was rerun after adding only the sentinel declaration and remained RED before scanner implementation.

Supporting gates are deliberately outside the 1:1 Behavior mapping:

- `FuzzSelectorGrammar` exercises arbitrary bytes, separator/escape-heavy inputs, and long inputs for panic freedom and deterministic output. It is supporting robustness evidence, not a primary Behavior test.
- `BenchmarkSelectorGrammar` records non-gating baseline time and allocation data at 1 KiB, 16 KiB, and 256 KiB. It has no pass/fail threshold and is not used to claim complexity; static review verifies that the scanner is a single pass with no recursion or global mutable parser state, while expanded-list traversal retains the existing `maxResolveDepth` bound.
- Preserve the existing million-segment regression and run race-enabled tests to cover long-input and concurrent-call safety without creating another Product Behavior.

### Verification evidence

- Every B01-B15 behavior-specific mutation made its one primary test fail; the unmutated B01-B15 suite passes.
- `gofmt`, `go vet ./...`, `go test ./... -count=1`, and `go test -race ./... -count=1` pass.
- A 10-second `FuzzSelectorGrammar` run passes after about 2.9 million executions; the final suffix-path compatibility adjustment also passes a 5-second run after about 1.4 million executions.
- The non-gating benchmark baseline is 1,308 ns/op at 1 KiB, 17,981 ns/op at 16 KiB, and 287,880 ns/op at 256 KiB, with 336 B/op and 3 allocs/op at each size on darwin/arm64 Apple M3 Max.
- `golangci-lint run --new-from-rev origin/main ./...` reports zero issues, and `GOTOOLCHAIN=go1.18.10 go test ./... -count=1` passes.
- README and godoc were reconciled claim-by-claim on 2026-07-10 against `ognl.go` and the B01-B15 tests in `selector_grammar_test.go`.

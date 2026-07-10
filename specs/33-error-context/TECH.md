# Issue #33: Redacted error context — technical specification

## Context

The behavior contract is defined in [PRODUCT.md](./PRODUCT.md), B01–B14.

- `ognl.go:312-477` and `ognl.go:485-628` contain the top-level `GetE`/`Get` walkers. They currently pass sliced selector strings through recursion and wrap several failures after the actual failing value has been lost.
- `ognl.go:242-300` implements the `Result.Get`/`Result.GetE` collection layer. Deployed branches call the top-level APIs again and may add another wrapper around a child error.
- `ognl.go:639-659` parses the next key while consuming escapes, but returns only decoded text and an ending index; it does not preserve source-byte location or operation count.
- `ognl.go:919-926` is the single formatter. It uses `%T` for the caller-provided object and interpolates the supplied path verbatim.
- `verify_test.go:148-164` protects object-value redaction and `errors.Is`, but it does not cover selector secrecy, source locations, bounded formatting, branch anchoring, or `Result` parity.
- Issue #25 is implemented by PR #38, `fix/issue-25-gete-expansion-failure` at head `188d4a62d0df25b2af6bb0e0f57f3ad3088d8cfb`. That change owns failed-result invalidation, partial-success filtering, all-failed collection, and their top-level/`Result` consistency. B11 and B12 depend on that behavior; issue #33 must not copy or take ownership of the #25 state/value logic.
- Issue #32 owns selector grammar decisions, including trailing escapes and `ErrInvalidSelector`; this change must consume the grammar currently exposed by the walker without redefining it.

## Proposed changes

1. Introduce an unexported per-call resolution state shared by `Get` and `GetE`. It carries only the original selector byte length, current source-byte base, and current operation index; selector or decoded-key text must never be copied into an error value.
2. Extend the internal selector step result to return source metadata alongside the decoded lookup token. The walker advances metadata for key/index operations and unescaped expansion operations, while existing separator and escape recognition remains unchanged. The parser change is metadata-only and must not claim issue #32 grammar.
3. Pass the per-call state through pointer descent, deployed recursion, and `Result` collection. A `Result` public method creates a fresh state for its supplied selector. Recursive calls use the same source base rather than passing a sliced selector as a new origin.
4. Add one unexported contextual wrapper with `Error()` and `Unwrap()`. Its constructor receives the leaf error, current object type, and numeric source metadata; it does not accept an object value, selector, or decoded key.
5. Mark contextual errors internally so collection layers can reuse them rather than wrap them again. Parent branches may append an existing contextual error to diagnostics, but only the leaf that knows the failing object and location may create the context.
6. Centralize type-token serialization and final message assembly. Emit the exact nil token from B01 and first serialize non-nil dynamic types to an ASCII-safe quoted token within the 256-byte token budget. Assemble the complete message, then enforce the 352-byte whole-message budget by safely shortening the encoded type token further when necessary. Both truncation stages must retain `<truncated>`, balanced quoting, valid escapes, and valid UTF-8; numeric metadata and the sentinel message remain complete, and `%w`/`Unwrap` remains independent of display truncation.
7. Route every package-sentinel failure produced by `get`, `getE`, and their `Result` collectors through the same constructor. Build on PR #38's partial-success and all-failed result state without duplicating its invalidation or collection logic; issue #33 adds ordered context selection and formatting only.
8. Keep all helpers, state, and wrapper types unexported. Do not add options, public context objects, signatures, or a new selector sentinel. Keep dangling-escape parsing outside this change and use the issue #32 integration gate below for its eventual grammar tests.

## Testing and validation

Each PRODUCT invariant has one primary test and exactly one mutation target. Primary tests belong in `issue_33_test.go` unless an external-package API snapshot is required.

| Behavior | Primary test | Mutation target that must make the primary test fail |
| --- | --- | --- |
| B01 | `TestIssue33ActualFailureObjectType` is table-driven over a failing `[]int`, an untyped nil current object with exact token `"<nil>"`, and a typed nil current object with its dynamic type token. | Fall back to the entry/root type whenever the current object is nil or typed nil. |
| B02 | `TestIssue33OriginalSelectorLocationFields` asserts the exact ordered prefix and byte-based `offset`, `op`, and `total_len` for a multi-step failure. | Reset the recursive byte base or operation index to zero. |
| B03 | `TestIssue33OperationAccounting` is table-driven over plain keys, numeric indexes, separators, unescaped `#`, an operation beginning with an escape, and an operation with a later escape whose offset remains at the operation's first source byte. | Replace the operation offset when a later escape byte is encountered. |
| B04 | `TestIssue33RedactsSelectorKeysAndObjectValues` uses distinct canaries in the successful prefix, failing key, object fields, `Stringer`, and `error` values and rejects every canary in errors and diagnoses. | Restore raw selector interpolation in the contextual message. |
| B05 | `TestIssue33TypeTokenEncodingAndLimit` exercises the serializer with quotes, slashes, controls, Unicode, and an overlong token and asserts the 256-byte escaped form and marker. | Remove the 256-byte type-token cap. |
| B06 | `TestIssue33InternalErrorStringLimit` combines a complete 256-byte encoded type token, three 20-digit maximum-width unsigned decimal metadata values (`offset`, `op`, and `total_len`), and the current longest 29-byte package-sentinel message (`ErrInvalidStructure`), producing an exact uncapped length of 375 bytes. It asserts further safe type shortening, `<truncated>`, valid quoting/escapes/UTF-8, complete metadata and sentinel text, `errors.Is`, and the 352-byte ceiling. | Remove the 352-byte whole-message cap so the 375-byte fixture escapes unchanged. |
| B07 | `TestIssue33GetAndGetEContextParity` triggers one failure through both APIs and compares the full contextual text. | Keep a legacy `Get`-only or `GetE`-only formatter call site. |
| B08 | `TestIssue33ContextPreservesErrorsIs` checks `errors.Is` for each package sentinel after contextual wrapping. | Replace wrapping/`Unwrap` with `%v` text composition. |
| B09 | `TestIssue33ResultMethodParityAndFreshOrigin` compares top-level and `Result` methods, then chains a second call whose location restarts from its own selector. | Reuse the previous call's source base in a chained `Result` method. |
| B10 | `TestIssue33DeployedContextAnchoredOnce` fails below an expanded branch and asserts the original location plus exactly one context prefix. | Re-wrap the contextual child error in the deployed parent. |
| B11 | `TestIssue33PartialDeploymentDiagnostics` uses one successful and one failing branch and asserts preserved output, non-fatal `GetE`, and one contextual diagnosis for the failed branch. | Promote the first branch error to a fatal `GetE` return. |
| B12 | `TestIssue33AllDeploymentFailuresSelectFirstContext` uses heterogeneous branch types and failure sentinels, asserting no partial output, diagnoses in input order, and a fatal `GetE` context equal to the first failed branch's context. | Select the last failed branch as the fatal context. |
| B13 | `TestIssue33HostileValidSelectorsAndMissingResolution` is table-driven over valid escaped segments, Unicode, control bytes, a 1 MiB secret-bearing selector, and an ordinary terminal missing-resolution failure. | Reinsert the raw 1 MiB selector into the contextual error text. |
| B14 | `TestIssue33PublicAPISurfaceSnapshot` uses `go/types` from an external test package to compare exported identifiers and signatures with the baseline snapshot. | Export the contextual wrapper type. |

Supporting verification, intentionally outside the one-to-one primary mapping:

- `TestIssue33FirstExpansionInvalidContext` covers a typed-nil first `#` operation through direct, nested, and fresh `Result` calls. It requires `Invalid` plus a nil deployment error to become one contextual `ErrInvalidValue`; removing that mapping from either walker must fail the test.
- `TestIssue33DereferencedFailureType` covers key and index failures after transparent pointer dereference through top-level, nested, deployed-branch, and `Result` calls. It requires the parse helpers to propagate the dereferenced failure type while preserving typed-nil pointer identity; replacing that type with the caller's pre-dereference type must fail the test.
- `FuzzIssue33ErrorContextSafety` feeds arbitrary selector bytes and nested values through `Get`, `GetE`, `Result.Get`, and `Result.GetE`; it asserts no panic, valid bounded output for package sentinels, and absence of injected selector/object canaries.
- `TestIssue33ConcurrentContextIsolation` runs the four public entry paths against shared read-only inputs under `go test -race`, asserting that locations and diagnostics never cross calls.
- PR #38 stacking gate: before implementation begins, ensure `fix/issue-33-error-context` is based on PR #38 head `188d4a62d0df25b2af6bb0e0f57f3ad3088d8cfb`, then open its stacked PR with base `fix/issue-25-gete-expansion-failure`. Do not copy #25 implementation or tests into #33. After PR #38 merges, fetch its merged `main`, rebase the #33 branch onto `main`, retarget the stacked PR to `main`, and rerun the complete presubmit and B11/B12 tests before review continues.
- Issue #32 integration gate: after issue #32 merges, rerun B02, B03, and B13 against its grammar and add the dangling-escape syntax tests required by the #32 specification. This specification does not predict the dangling-escape result or sentinel.
- Presubmit: `gofmt` for future Go changes, `go test ./...`, `go test -race ./...`, `go vet ./...`, repository lint if configured, and `git diff --check`.

# Issue #33: Redacted error context — technical specification

## Context

The behavior contract is defined in [PRODUCT.md](./PRODUCT.md), B01–B14.

- The integration baseline is `main@8e90af47285fc0802e1aa4a879ea0bebf81df018`, which already contains the issue #25 result-state fix, issue #27 expansion budgets, issue #28 flat-map accounting, accessor/index fixes, deployment allocation improvements, and issue #32's tokenized selector grammar.
- Issue #32 validates each selector once into private `selectorToken` values and owns all syntax, escape, whitespace, numeric-dispatch, and `ErrInvalidSelector` behavior. Issue #33 may attach immutable source-byte offset and operation metadata to those tokens, but must not reparse or redefine the grammar.
- `get`, `getE`, and their `Result` collectors already share an independent `expansionBudget` plus `retainedResults` accounting. Error-context tracking must travel beside that budget; it must never replace, reset, or duplicate resource accounting.
- Issue #25 owns failed-result invalidation, partial-success filtering, all-failed collection, and their top-level/`Result` consistency. B11 and B12 add ordered contextual errors to that state machine without changing which values survive.
- Selector-contract B15 remains authoritative for valid missing lookups: `Get` preserves its existing Diagnosis rules (including no diagnosis for an ordinary map miss), while `GetE` returns an Invalid result and contextual `ErrInvalidValue`.

## Proposed changes

1. Introduce an unexported, immutable per-call `resolutionState` shared by `Get` and `GetE`. It carries only the original selector byte length; selector or decoded-key text is never copied into an error value.
2. Extend issue #32's private `selectorToken` with immutable original-byte offset and operation index. The existing tokenizer locks a segment's offset at its first started source byte (the backslash for a leading escape), ignores leading ASCII whitespace, gives separators no operation, and records an unescaped `#` at its original byte index.
3. Pass selector tokens, `resolutionState`, and `expansionBudget` independently through top-level walkers, deployed recursion, and `Result` collection. Each public call creates fresh state and budget; copied immutable tokens/state give every branch the same original origin while the shared budget continues aggregating work and retained results.
4. Add one unexported contextual wrapper with `Error()` and `Unwrap()`. Its constructor receives the leaf error, current object type, and numeric source metadata; it does not accept an object value, selector, or decoded key.
5. Mark contextual errors internally so collection layers can reuse them rather than wrap them again. Parent branches may append an existing contextual error to diagnostics, but only the leaf that knows the failing object and location may create the context.
6. Centralize type-token serialization and final message assembly. Emit the exact nil token from B01 and first serialize non-nil dynamic types to an ASCII-safe quoted token within the 256-byte token budget. Assemble the complete message, then enforce the 352-byte whole-message budget by safely shortening the encoded type token further when necessary. Both truncation stages must retain `<truncated>`, balanced quoting, valid escapes, and valid UTF-8; numeric metadata and the sentinel message remain complete, and `%w`/`Unwrap` remains independent of display truncation.
7. Route every package-sentinel failure produced by selector validation, `get`, `getE`, expansion-budget operations, and their `Result` collectors through the same constructor. Contextual children are reused rather than wrapped again; issue #33 adds formatting and first-context selection only.
8. Keep all new helpers, state, token metadata, and wrapper types unexported. Do not add options, public context objects, signatures, or another selector sentinel; the already-exported `ErrInvalidSelector` and `ErrExpansionLimit` remain in the API snapshot.

## Testing and validation

Each PRODUCT invariant has one primary test and exactly one mutation target. Primary tests belong in `issue_33_test.go` unless an external-package API snapshot is required.

| Behavior | Primary test | Mutation target that must make the primary test fail |
| --- | --- | --- |
| B01 | `TestIssue33ActualFailureObjectType` is table-driven over a failing `[]int`, an untyped nil current object with exact token `"<nil>"`, and a typed nil current object with its dynamic type token; `Get` assertions follow the existing selector-contract Diagnosis rules. | Fall back to the entry/root type whenever the current object is nil or typed nil. |
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
| B13 | `TestIssue33HostileValidSelectorsAndMissingResolution` is table-driven over valid escaped segments, Unicode, control bytes, a 1 MiB secret-bearing selector, and an ordinary terminal map miss; it checks contextual `GetE` while preserving B15's empty `Get` Diagnosis. | Reinsert the raw 1 MiB selector into the contextual error text. |
| B14 | `TestIssue33PublicAPISurfaceSnapshot` uses `go/types` from an external test package to compare exported identifiers and signatures with the current baseline, including `ErrExpansionLimit` and `ErrInvalidSelector`. | Export the contextual wrapper type. |

Supporting verification, intentionally outside the one-to-one primary mapping:

- `TestIssue33FirstExpansionInvalidContext` covers a typed-nil first `#` operation through direct, nested, and fresh `Result` calls. It requires `Invalid` plus a nil deployment error to become one contextual `ErrInvalidValue`; removing that mapping from either walker must fail the test.
- `TestIssue33DereferencedFailureType` covers key and index failures after transparent pointer dereference through top-level, nested, deployed-branch, and `Result` calls. It requires the parse helpers to propagate the dereferenced failure type while preserving typed-nil pointer identity; replacing that type with the caller's pre-dereference type must fail the test.
- `TestIssue33ExpansionLimitContext` covers top-level, top-level-expanded, `Result.Get`, and `Result.GetE` budget failures. It requires one bounded redacted context, `Diagnosis()` parity where applicable, and preserved `errors.Is(ErrExpansionLimit)`.
- `FuzzIssue33ErrorContextSafety` feeds arbitrary selector bytes and nested values through `Get`, `GetE`, `Result.Get`, and `Result.GetE`; it asserts no panic, valid bounded output for package sentinels, and absence of injected selector/object canaries.
- `TestIssue33ConcurrentContextIsolation` runs the four public entry paths against shared read-only inputs under `go test -race`, asserting that locations and diagnostics never cross calls.
- Stacking gate completed: the three issue #33 commits were rebased from the old PR #38 head onto `main@8e90af4`; the PR base is retargeted to `main` only after all gates pass.
- Issue #32 integration gate: run its complete B01-B15 suite plus issue #33 B02/B03/B13. Dangling escapes remain `ErrInvalidSelector`, are contextualized without selector text, and do not alter issue #32 precedence or B15 resolution behavior.
- Presubmit: targeted issue #25/selector B01-B15/issue #33 tests, `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, checkptr, Go 1.18, issue #33 fuzzing, mutation checks, and `git diff --check`.

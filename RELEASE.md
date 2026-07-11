# v0.1.0 release readiness

`v0.1.0` is the current candidate name for the first release using the
canonical module path `github.com/golang-infrastructure/go-ognl`. On
2026-07-11, the owner approved the current MIT `LICENSE` and its
`Copyright (c) 2022 Golang Infrastructure` notice. The version and publication
timing remain deferred. This PR does not create a tag or publish a release.
Merging this PR does not authorize or trigger either action.

## Planned release notes

- Establish the canonical module path for tagged consumers if this candidate is
  approved and published.
- Carry the owner-approved MIT `LICENSE` and copyright notice into the release.
- Ship the README API scenario whose complete Go program and expected output are
  extracted, compiled, run, and compared by `TestREADMEExample`.
- Include the accepted Result compatibility contract, deterministic selector
  grammar, and bounded redacted resolution-error context already present in the
  candidate baseline.
- Summarize only accepted changes that are present in the exact tagged commit.

## Current draft reconciliation status

- [x] Rebased this draft onto `main` commit
  `fa635ecf5c897367a68c7000bde170848085538d`.
- [x] Preserved the expansion limits from PR #40, chained flat-map behavior from
  PR #41, and Result accessor ownership contract from PR #43.
- [x] Confirmed PR #44, PR #45, and PR #48 are present in that `main` history.
- [x] Confirmed issues #29, #32, and #33 are closed and their accepted PRs #42,
  #47, and #49 are merged into that `main` history. Their Result, selector, and
  error-context contracts are reflected in README and GoDoc.
- [x] Confirmed the published tags are still `v0.0.1` through `v0.0.3`, with
  `v0.0.3` still the latest release.
- [x] Ran the README example, Result/selector/error-context contract, full, race,
  vet, checkptr, Go 1.18, godoc, formatting, and module gates on this reconciled
  Draft on 2026-07-11. The exact tag commit must run them again.
- [x] Owner approved the current MIT `LICENSE`, copyright holder, and year on
  2026-07-11.
- [ ] Owner decides whether the first canonical-path release is `v0.1.0` and
  when it should be published.

## Before tagging

- [ ] For every issue the owner chooses for this release, verify its accepted PR
  is merged into the exact commit to tag.
- [ ] Start from an up-to-date `main` checkout and confirm
  `git status --porcelain` is empty.
- [ ] Confirm the GitHub Actions matrix is green on that exact `main` commit.
- [ ] Run the local release gates from the clean checkout:

  ```shell
  test -z "$(gofmt -l .)"
  go vet ./...
  go test ./... -count=1
  go test -race ./... -count=1
  go test -gcflags=all=-d=checkptr=2 ./... -count=1
  go test ./... -run '^TestREADMEExample$' -count=1
  ```

- [ ] Review the final diff and release notes against the exact commit to tag.

## Publish

- [ ] Create the annotated owner-approved version tag (currently `v0.1.0` is
  only the candidate) on the verified `main` commit.
- [ ] Push the tag and create the GitHub release from that tag.
- [ ] Do not describe an issue as shipped unless its accepted change is present
  in the tagged commit.

## After publishing

- [ ] Create a new temporary directory outside this repository.
- [ ] Initialize a fresh module without a `replace` directive and fetch the
  public release:

  ```shell
  go mod init example.com/go-ognl-smoke
  go get github.com/golang-infrastructure/go-ognl@latest
  go list -m -f '{{.Path}} {{.Version}}' github.com/golang-infrastructure/go-ognl
  ```

- [ ] Confirm `go list` reports the canonical path and the exact version approved
  and published by the owner.
- [ ] Copy the complete README example into `main.go`, run `go run .`, and
  confirm its documented output.

## Documentation audit baseline

The license decision above was reconciled on 2026-07-11 from the owner's
explicit approval of the current `LICENSE`. The remaining release-readiness
claims were reconciled on 2026-07-11 against `main` commit
`fa635ecf5c897367a68c7000bde170848085538d`, `go.mod`, `ognl.go`, the test suite,
closed issues #29/#32/#33, merged PRs #42/#47/#49, and published tags `v0.0.1`
through `v0.0.3`. Re-run this audit against the exact final commit before
creating any release tag.

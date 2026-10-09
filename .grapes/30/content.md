## Goal

Release vineyard 0.1.3, built on the grapes v0.1.14 release, so installed vineyards get issue prompt templates (#29).

## Description

#29 merged with `go.mod` pinned to grapes' PR commit `fd6bdfe` (`v0.1.14-0.20261009192140-fd6bdfeb133c`). Grapes #41 has since merged and released v0.1.14 (`df108cc`), and `docs/development.md` says to pin a grapes release. Vineyard's `version` is still 0.1.2, the release before #19, #26, #27, #28, and #29, so none of them has been released.

`main` also fails `go vet ./...`: the merge of `main` into #28's branch (`99e0650`) dropped the `internal/config` import from `internal/tui/app_grapes_test.go`, which #29's `exampleTemplates` uses, so the `tui` tests do not compile.

## Context

- `go.mod`: `github.com/Mibokess/grapes v0.1.14-0.20261009192140-fd6bdfeb133c`.
- `internal/tui/app_grapes_test.go`: `exampleTemplates` calls `config.Prepare` and `config.LoadTemplates` without importing `config`.
- `main.go`: `var version = "0.1.2"`. A merge to `main` that changes it tags `v<version>` and runs GoReleaser (`docs/development.md`, Releases).

## Acceptance Criteria

- [x] `go.mod` requires `github.com/Mibokess/grapes v0.1.14`.
- [x] `version` is 0.1.3, and `vineyard version` prints it.
- [x] `go vet ./...` and `go test ./...` pass on the branch.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
go build -o .grapes/30/tmp/vineyard . && .grapes/30/tmp/vineyard version
```

## Pass Criteria

`gofmt` prints nothing; vet and tests pass; `vineyard version` prints `0.1.3`.

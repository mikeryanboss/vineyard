## Goal

Nobody has to remember to release: every merge to `main` that changes the program is tagged with the next patch version and released.

## Description

A release needs someone to bump `var version` in `main.go`, and that was forgotten twice: after grapes #29 (fixed by #30) and after #31–#35 (fixed by #36, v0.1.4). The user chose automatic releases over a PR check or a written rule.

Auto Tag stops reading `main.go`. On a push to `main` that changes the program, it tags the pushed commit with the highest `v*` tag's patch plus one, then calls `release.yml` as #36 set up. The binary takes its version from that tag: GoReleaser stamps it into `main.version`, and builds without GoReleaser use the module version Go stamps in.

## Context

- `.github/workflows/auto-tag.yml`: runs on pushes to `main` that change `main.go`, reads `var version`, tags `v<version>` if missing, and calls `release.yml` when it created the tag (`created` output, from #36).
- `.github/workflows/release.yml`: GoReleaser on a tag push or `workflow_call`. Unchanged.
- `.goreleaser.yaml`: `-X main.version={{.Version}}`. `-X` sets only a string variable whose initial value is a constant, so `version` must stay one.
- `main.go`: `var version = "0.1.4"`, used by `vineyard version`, `vineyard debug`, and the TUI header (`tui.Options.Version`, `renderHeader` truncates the header row to the width).
- What goes into the binary: Go files other than tests and `testdata`, `go.mod`, `go.sum`, and `internal/config/templates/*.md` (`//go:embed` in `internal/config/templates.go`).
- Docs: `docs/development.md` (Releases), `docs/README.md` ("Release a version" row, the `.github/workflows/` line).
- Issue #36's last criterion (the v0.1.4 release after merge) is verified and gets ticked here.

## Acceptance Criteria

- [ ] Auto Tag runs on pushes to `main` that change non-test Go code, `go.mod`, `go.sum`, or the embedded templates, and not on docs, tests, or issue files alone.
- [x] It tags an untagged HEAD with the highest `v*` tag's patch plus one, and calls `release.yml`.
- [x] It creates no tag, and calls no release, when HEAD already has a `v*` tag.
- [x] After a hand-pushed tag such as v0.2.0, the next automatic tag is v0.2.1.
- [ ] Runs do not overlap, so two runs cannot compute the same tag.
- [x] `vineyard version` prints the GoReleaser-stamped version when stamped, and the module version Go stamped in otherwise, without the leading `v`.
- [x] The docs describe the automatic release and how to release a minor or major version.
- [x] #36's last criterion is ticked with its verification.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
actionlint .github/workflows/*.yml
go build -o .grapes/37/tmp/vineyard . && .grapes/37/tmp/vineyard version
go build -ldflags "-X main.version=0.1.5" -o .grapes/37/tmp/vineyard-stamped . && .grapes/37/tmp/vineyard-stamped version
```

Run the tag step's script, taken from `auto-tag.yml`, in scratch clones under `.grapes/37/tmp/`: an untagged HEAD after v0.1.4, a HEAD tagged v0.1.4, and an untagged HEAD after a v0.2.0 tag. Use a local bare repository as `origin`.

After merge: the merge's Auto Tag run tags v0.1.5 and publishes its release, and the release's binary prints `0.1.5`.

## Pass Criteria

`gofmt` and actionlint print nothing; vet and tests pass. The unstamped build prints a pseudo-version of the form `0.1.5-0.<timestamp>-<commit>[+dirty]`; the stamped build prints `0.1.5`. The scratch runs create v0.1.5, nothing, and v0.2.1, and write `created=true`, `created=false`, `created=true`.

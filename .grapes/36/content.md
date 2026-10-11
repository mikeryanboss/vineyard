## Goal

Release vineyard 0.1.4 with the subagent and mouse work, and make the release actually publish the downloads the README points to.

## Description

`version` in `main.go` is still 0.1.3, the release before #31–#35 (PRs #23–#27): subagent rows, collapsed subagents, clickable screens, and double clicks. None of them is released.

Releasing has also never published anything. `.github/workflows/auto-tag.yml` pushes `v<version>` with the workflow's `GITHUB_TOKEN`, and GitHub starts no workflow for events made with that token, so `.github/workflows/release.yml` (on tag push) has never run. Tags v0.1.1–v0.1.3 exist; `gh run list --workflow release.yml` prints `[]` and `gh release list` prints nothing. `go install …@latest` works, since it needs only the tag, but the README's "Download a Linux or macOS build from the releases" leads to an empty page.

Fix: `release.yml` also runs as a reusable workflow (`workflow_call`), and Auto Tag calls it after it creates a tag. The called run checks out the pushed commit, which the new tag points at, so GoReleaser releases that tag. A tag pushed by hand still runs `release.yml` directly.

## Context

- `main.go`: `var version = "0.1.3"`.
- `.github/workflows/auto-tag.yml`: job `tag`, steps `version`, `check` (`exists`), `Create tag`.
- `.github/workflows/release.yml`: `on: push: tags: v*`; GoReleaser `release --clean`.
- `docs/development.md`, Releases: says the tag runs `release.yml`.
- grapes stays at v0.1.14 in `go.mod`; #31–#35 changed no grapes code.

## Acceptance Criteria

- [x] `version` is 0.1.4, and `vineyard version` prints it.
- [x] Auto Tag calls `release.yml` when, and only when, it created the tag.
- [x] `release.yml` still runs on a tag push.
- [x] `docs/development.md` describes how a release runs.
- [ ] After merge: the v0.1.4 tag exists and a GitHub release v0.1.4 has the four archives and `checksums.txt`.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
go build -o .grapes/36/tmp/vineyard . && .grapes/36/tmp/vineyard version
.grapes/36/tmp/bin/actionlint .github/workflows/*.yml
```

After merge:

```bash
gh run list --limit 3
gh release view v0.1.4 --json tagName,assets --jq '{tagName, assets: [.assets[].name]}'
```

## Pass Criteria

`gofmt` and actionlint print nothing; vet and tests pass; `vineyard version` prints `0.1.4`. After merge, the Auto Tag run succeeds, including its release job, and the release lists `vineyard_0.1.4_{linux,darwin}_{amd64,arm64}.tar.gz` and `checksums.txt`.

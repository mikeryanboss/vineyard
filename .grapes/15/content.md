## Goal

Publish vineyard releases the way grapes does: bumping the version on `main` tags it, and the tag publishes binaries. The first release is v0.1.1.

## Description

Copy grapes' release setup (`.github/workflows/auto-tag.yml`, `.github/workflows/release.yml`, `.goreleaser.yaml`) and adapt GoReleaser to vineyard. The binary is `vineyard`, and there is no Windows target: sessions run in tmux, which Windows lacks, and WSL uses the Linux build. Bump `main.go` from 0.1.0 (never tagged) to 0.1.1, because auto-tag runs only when a push to `main` changes `main.go`.

## Context

- `main.go:25`: `var version = "0.1.0"`; `-X main.version` stamps it, as in grapes.
- Vineyard had no `.github/` and no tags.
- All six linux/darwin/windows × amd64/arm64 targets compile with `CGO_ENABLED=0` (`internal/session/lock_other.go` covers non-Unix), but tmux makes Windows unusable.

## Acceptance Criteria

- [x] `.goreleaser.yaml` passes `goreleaser check`; a snapshot release builds linux and darwin archives for amd64 and arm64, plus checksums.
- [x] A snapshot binary prints the stamped version.
- [x] `main.go` declares 0.1.1.
- [x] `docs/development.md`, `docs/README.md`, and `README.md` describe releases.
- [ ] After merge, tag `v0.1.1` and its GitHub release with four archives exist.

## Verify

Run from the worktree root:

```bash
goreleaser check
goreleaser release --snapshot --clean --skip=publish && tar -xzf dist/vineyard_*_linux_amd64.tar.gz vineyard && ./vineyard version
gh release view v0.1.1 --json assets -q '.assets[].name'   # after merge
```

## Pass Criteria

The check validates one file; the snapshot prints `0.0.0-SNAPSHOT-<sha>`; after merge, the release lists four `vineyard_0.1.1_*.tar.gz` archives and `checksums.txt`.

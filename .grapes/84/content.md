## Goal
Ensure an updated Vineyard binary starts a new server daemon with the `#82` color-attach fix instead of reusing a running `vineyard-p6` daemon from an earlier build.

## Description
`attach` runs inside Vineyard's background server process (`internal/vterm/daemon.go`, `internal/vterm/attach.go`). Commit `80d0b3a` (#82, released in v0.1.44) fixed `newScreen` in `internal/vterm/attach.go` to detect the viewer's terminal color profile, but kept `protocol = 6` in `internal/vterm/daemon.go`.

When a `vineyard-p6` server from v0.1.42 or v0.1.43 is already running (as verified via `vineyard debug`: PID 2579 running `/usr/local/bin/vineyard` v0.1.43 on `/tmp/vineyard-1000/vineyard-p6`), launching v0.1.44 connects to that existing `vineyard-p6` daemon both to start new sessions and to run `attach`—leaving even newly created sessions black-and-white until the older daemon exits.

Per `internal/vterm/daemon.go:22` ("Change it with any change to the calls, their arguments, or attaching"), bump `protocol` from `6` to `7` while keeping `sameCallsSince = 1`. Updated Vineyards then start their own `vineyard-p7` server for new and resumed sessions while keeping running `p6` sessions reachable side by side via `NewOlderClient` and `AttachThroughServer`.

## Context
- `internal/vterm/daemon.go`:
  - `const protocol = 6` (line 27) and comment on line 31 (`Protocols 1 to 6 differ only in attaching`). Raise `protocol` to `7` and update the comment; keep `sameCallsSince = 1`.
- `main_test.go`:
  - `TestVineyards` (lines 293–295) starts a fake server speaking `vterm.Protocol()` on `vy-p7` and looks it up via `v.Older(7)` to test `Older` on another socket speaking the current protocol. With `protocol = 7`, `vy-p7` collides with the current protocol's socket; use `p := vterm.Protocol() + 1` (`vy-p<p>`, `v.Older(p)`) so it stays distinct across future protocol bumps.
- `docs/architecture.md`:
  - Line 477 notes `protocols 1 to 6 differ only in attaching`; update to `protocols 1 to 7`.

## Acceptance Criteria
- [x] `vterm.Protocol()` returns `7` and `vterm.SameCalls` covers protocols `1` through `7`.
- [x] Starting a session on the new build while a `vineyard-p6` server runs starts a separate `vineyard-p7` server (`os.Executable()`) whose `attach` renders colors, while `p6` sessions remain visible via `Older(6)`.
- [x] `TestVineyards` in `main_test.go` exercises `Older` speaking `vterm.Protocol()` on a non-current protocol number (`vterm.Protocol() + 1`), and `gofmt -l .`, `go vet ./...`, and `go test ./...` pass.

## Verify
Run from `/workspaces/vineyard/.vineyard/worktrees/bug-e54065`:
```bash
gofmt -l . && go vet ./... && go test -count=1 . ./internal/vterm/... ./internal/session/...
go test ./...
```

## Pass Criteria
`gofmt -l .` prints nothing, `go vet ./...` exits 0, and `go test` passes across all packages.

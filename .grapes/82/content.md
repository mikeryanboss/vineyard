## Goal
Preserve ANSI colors in both the program viewport and the status bar when attached to a session on Vineyard's server.

## Description
Commit `bcbe5f6` (#79) changed `newScreen` in `internal/vterm/attach.go` to pass a `*bytes.Buffer` to `uv.NewTerminalRenderer(out, env)` so frames render into memory under `t.mu` before being written to the viewer's terminal outside the lock. Inside `uv.NewTerminalRenderer`, `colorprofile.Detect(w, env)` checks whether `w` implements `term.File` and `term.IsTerminal(w.Fd())`. Because `*bytes.Buffer` is not a terminal file, `colorprofile.Detect` sees `isatty == false` (unless `TTY_FORCE=1` is set), selects `colorprofile.NoTTY`, and strips all ANSI colors from attached frames—even when `TERM=xterm-256color` or `COLORTERM=truecolor` is set.

Detect the color profile against the viewer's real `out *os.File` and `env` via `colorprofile.Detect(tty, env)` and apply it with `renderer.SetColorProfile(...)`, while keeping `*bytes.Buffer` as the renderer's backing writer.

## Context
- `internal/vterm/attach.go`:
  - `attach` (line 71) receives `in, out *os.File` (passed from the client over `SCM_RIGHTS`) and `env []string`, and calls `newScreen(env)` at line 117 before acquiring `t.mu`.
  - `newScreen` (lines 393–396) creates `out := new(bytes.Buffer)` and `uv.NewTerminalRenderer(out, env)`.
  - Change `newScreen(tty *os.File, env []string) *screen` (planned signature) to call `r.SetColorProfile(colorprofile.Detect(tty, env))` on the renderer backed by `*bytes.Buffer`. `colorprofile.Detect` inspects `tty.Fd()` via `term.IsTerminal` without writing to `tty`.
- `go.mod`:
  - `github.com/charmbracelet/colorprofile v0.4.3` is already in the module graph as an indirect dependency of `ultraviolet`; importing it in `internal/vterm/attach.go` promotes it to a direct dependency.
- `internal/vterm/daemon.go`:
  - Wire calls and `AttachRequest` are unchanged, so `protocol` remains `6`.
- `internal/vterm/vterm_test.go`:
  - `attachTestTerm` attaches on a real `creack/pty` pair (`TERM=xterm-256color`, no `TTY_FORCE`) mirrored into `outerTerm.emu` (`*vt.Emulator`), where `emu.Render()` retains SGR color sequences. Add `TestAttachKeepsColours` (planned) verifying color output in the attached viewport and status bar, plus `NO_COLOR=1` suppression.

## Acceptance Criteria
- [x] `newScreen` sets the `uv.TerminalRenderer` color profile using `colorprofile.Detect` against the viewer's output `*os.File` and `env`, while continuing to write rendered frames into `*bytes.Buffer`.
- [x] Attaching to a terminal on Vineyard's server renders ANSI colors for both program output and the status bar on a terminal fd without `TTY_FORCE`.
- [x] `NO_COLOR=1` in the attaching environment strips ANSI color sequences on the attached screen.
- [x] `TestAttachKeepsColours` in `internal/vterm/vterm_test.go` covers color preservation and `NO_COLOR=1`, and `gofmt -l .`, `go vet ./...`, and `go test ./...` pass.

## Verify
Run from `/workspaces/vineyard/.vineyard/worktrees/bug-e54065`:
```bash
go test ./internal/vterm/ -run TestAttachKeepsColours -count=1 -v
gofmt -l . && go vet ./... && go test -count=1 ./...
```

## Pass Criteria
`TestAttachKeepsColours` passes, `gofmt -l .` prints nothing, `go vet ./...` exits 0, and `go test -count=1 ./...` passes.

## Goal
Keep the overview TUI aligned after detaching from an attached session (`ctrl+q`) or external diff tool and across terminal resizes, without requiring a manual window wiggle to recover.

## Description
Returning from an attached session (`tea.Exec`) to Vineyard's overview sometimes leaves headers, borders, and panes misaligned at the wrong rows and columns until the user resizes the terminal window slightly:

1. **Unicode Mode 2027 (`?2027`) not restored on detach.** At startup, `charm.land/bubbletea/v2` queries DEC Mode 2027 (`?2027$p`) and enables `\x1b[?2027h` (`SetModeUnicodeCore`) with `s.cellbuf.Method = ansi.GraphemeWidth` on supporting terminals. When `tea.Exec` releases the terminal, `cursedRenderer.close()` writes `\x1b[?2027l` (`ResetModeUnicodeCore`), but `cursedRenderer.start()` never re-enables `\x1b[?2027h` on `RestoreTerminal()`. Vineyard (`preview.go`, `lipgloss/v2`, `vterm`) and `cellbuf` continue measuring lines at `GraphemeWidth` up to 100% of the terminal width, while the terminal falls back to `wcwidth`. Clusters wider under `wcwidth` (such as ZWJ or skin-tone emojis) wrap onto the next row, and `ultraviolet` (`20260703`) only re-anchored the horizontal column (`\x1b[...G`), shifting every subsequent row down across relative `\r\n` moves and desyncing `curbuf` until a window resize triggered `Erase()`.
2. **Dropped `Erase()` and unlatched resizes in `bubbletea v2.0.8` / `ultraviolet 20260703`.** In `bubbletea v2.0.8`, `cursedRenderer.resize()` and `clearScreen()` called `s.scr.Erase()` without setting `pendingErase`, so `flush()` skipped the redraw whenever `viewEquals(s.lastView, &view) && frameArea == s.cellbuf.Bounds()` (for example after a shrink-and-restore resize or `tea.ClearScreen` on an unchanged view). `ultraviolet 20260703` also ignored `height` in `TerminalRenderer.Resize` and did not latch `termResized` or disable autowrap on drift-prone lines.
3. **Stale issue tab width on `WindowSizeMsg`.** `Model.Update` on `tea.WindowSizeMsg` in `internal/tui/app.go` called `applySizes`, `refreshRecap`, and `refreshTranscript`, omitting `refreshIssue` (unlike `resizeLayout`).

Changes:
1. Upgrade `charm.land/bubbletea/v2` to `v2.1.1` and `github.com/charmbracelet/ultraviolet` to `v0.0.0-20261008173134-6b8d4baf91b4` (`pendingErase` in `cursedRenderer`, latched `TerminalRenderer.Resize`, autowrap disabled on drift-prone lines, and full 2D cursor re-anchoring `\x1b[y;xH`).
2. Track `tea.ModeReportMsg` for `ansi.ModeUnicodeCore` on `tui.Model` (`unicodeCore` field) and emit a `restoreScreenCmd` (`tea.Sequence(tea.Raw(ansi.SetModeUnicodeCore), tea.ClearScreen)` when `unicodeCore` is true, else `tea.ClearScreen`) from `attachDoneMsg` and `diffToolDoneMsg`.
3. Call `m.resizeLayout()` in `Model.Update`'s `tea.WindowSizeMsg` branch so the Issue tab also re-wraps on terminal resize.
4. Add regression tests in `internal/tui/app_test.go` covering Mode 2027 restoration + screen clear after `tea.Exec` detach (both unit and end-to-end PTY attach/resize/detach) and Issue tab re-rendering on `tea.WindowSizeMsg`.

## Context
- `go.mod`, `go.sum`: `charm.land/bubbletea/v2` (`v2.0.8` -> `v2.1.1`), `github.com/charmbracelet/ultraviolet` (`v0.0.0-20260703014108-f5a850f9c2b7` -> `v0.0.0-20261008173134-6b8d4baf91b4`).
- `internal/tui/app.go`: `Model` (`unicodeCore` field), `Model.Update` (`tea.WindowSizeMsg`, `tea.ModeReportMsg`, `attachDoneMsg`, `diffToolDoneMsg`), `Model.resizeLayout`, `Model.restoreScreenCmd`.
- `internal/tui/app_test.go`: `TestApp_RestoreScreenCmd` and `TestApp_DetachRestoresUnicodeCoreAndRedrawsAfterResize`.
- `internal/tui/app_grapes_test.go`: `TestIssues_WindowResizeRefreshesIssueTab`.

## Acceptance Criteria
- [x] Detaching from a session (`attachDoneMsg`) or returning from an external diff tool (`diffToolDoneMsg`) forces `tea.ClearScreen`, and re-enables `\x1b[?2027h` (`ansi.SetModeUnicodeCore`) beforehand when the terminal reported Mode 2027 support via `tea.ModeReportMsg`.
- [x] In an end-to-end PTY run on a Mode 2027-capable terminal, Mode 2027 is re-enabled after attaching and detaching (`ctrl+q`) with a window resize during the session, and the rendered overview matches a post-wiggle redraw.
- [x] `charm.land/bubbletea/v2` is upgraded to `v2.1.1` and `github.com/charmbracelet/ultraviolet` to `v0.0.0-20261008173134-6b8d4baf91b4`.
- [x] `tea.WindowSizeMsg` re-renders the Issue tab at the new pane width via `resizeLayout()`.
- [x] `gofmt -l .`, `go vet ./...`, and `go test ./...` pass in under 10 seconds.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test -count=1 ./...
```
Then exercise a development build in tmux via `scripts/drive` across session attach, window resize while attached, and `ctrl+q` detach.

## Pass Criteria
`gofmt -l .` prints nothing, `go vet ./...` and `go test -count=1 ./...` pass in under 10 seconds, and the real-terminal `scripts/drive` session returns from `ctrl+q` after an in-session resize with aligned header, pane borders, preview, and status bar.

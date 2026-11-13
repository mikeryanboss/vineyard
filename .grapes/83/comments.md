### 2026-11-13T16:05
[FINDINGS] Verified Claude Code 2.1.323 in a detached tmux session (`80x10` and `80x24`):
- Claude Code enters the alternate screen and enables SGR mouse tracking by default (`alternate_on=1`, `mouse_any_flag=1`, `mouse_standard_flag=0`, `mouse_sgr_flag=1`, `history_size=0`), even without `"tui": "fullscreen"` in `~/.claude/settings.json`.
- Sending SGR wheel-up (`\x1b[<64;10;3M`) and wheel-down (`\x1b[<65;10;3M`) sequences scrolls Claude Code's viewport up (showing its `Jump to bottom (ctrl+End) ↓` indicator) and back down to the live prompt.
- `tmux send-keys -H -t =<name>:` delivers raw hex bytes (`0x00`–`0xff`) to the pane without UTF-8 mangling, covering both SGR and X10 coordinates above 94.

### 2026-11-13T16:06
[STARTED] Implementing `Wheel` across `internal/vterm` (protocol 7), `internal/tmux`, `internal/tui/backend.go`, `internal/tui/preview`, and `internal/tui/app.go`, plus tests and docs.

### 2026-11-13T16:24
[DECISION] Kept `sameCallsSince = 1` when bumping `protocol` to `7` by treating `"can't find method"` from `Terminals.Wheel` in `(*vterm.Client).Wheel` as unhandled (`false, nil`), matching `Client.Info`'s handling of missing `Terminals.List`, so sessions running on older protocol 1–6 servers continue working across an upgrade. In `(*vterm.Server).Wheel`, wait up to one 60 Hz frame (`frame`) on `t.watch()` after writing wheel sequences so the TUI's immediate `Capture` sees the scrolled frame without waiting for the next 150 ms preview tick.

### 2026-11-13T16:24
[VERIFY] PASS. From `/workspaces/vineyard/.vineyard/worktrees/scroll-96e22b`: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed across all packages; `go test -race -count=1 ./internal/vterm/...` passed. Manual run with `scripts/drive` on `.grapes/83/tmp/demo` (`claude --model haiku`, protocol 7): generated `[line 01]..[line 45]`, sent 6 SGR wheel-up ticks (`\x1b[<64;70;12M`) over the preview pane without attaching and confirmed the live preview scrolled up to `[line 19]..[line 41]` with Claude Code's sticky prompt header and `Jump to bottom (ctrl+End) ↓` indicator; sent 6 SGR wheel-down ticks (`\x1b[<65;70;12M`) to scroll back down; focused the pane (`l`), pressed `g` to reach `Claude Code v2.1.323` and `● [line 01]` at the top, and pressed `G` + `Escape` to return to the bottom. Screenshots: `.grapes/83/tmp/preview-bottom.png`, `.grapes/83/tmp/preview-scrolled.png` (published on `pr-evidence` at `62ffa2b72f28f60f7d42ef0048309360d4df9f40`).

### 2026-11-13T16:24
[DONE] Updated `internal/vterm` (`attach.go`, `vterm.go`, `daemon.go`, `client.go`, protocol 7), `vineyards.go`, `internal/tmux/tmux.go`, `internal/tui/backend.go`, `internal/tui/common/messages.go`, `internal/tui/preview/preview.go`, `internal/tui/app.go`, tests, `README.md`, and `docs/architecture.md`.

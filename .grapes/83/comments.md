### 2026-11-13T16:05
[FINDINGS] Verified Claude Code 2.1.323 in a detached tmux session (`80x10` and `80x24`):
- Claude Code enters the alternate screen and enables SGR mouse tracking by default (`alternate_on=1`, `mouse_any_flag=1`, `mouse_standard_flag=0`, `mouse_sgr_flag=1`, `history_size=0`), even without `"tui": "fullscreen"` in `~/.claude/settings.json`.
- Sending SGR wheel-up (`\x1b[<64;10;3M`) and wheel-down (`\x1b[<65;10;3M`) sequences scrolls Claude Code's viewport up (showing its `Jump to bottom (ctrl+End) ↓` indicator) and back down to the live prompt.
- `tmux send-keys -H -t =<name>:` delivers raw hex bytes (`0x00`–`0xff`) to the pane without UTF-8 mangling, covering both SGR and X10 coordinates above 94.

### 2026-11-13T16:06
[STARTED] Implementing `Wheel` across `internal/vterm` (protocol 7), `internal/tmux`, `internal/tui/backend.go`, `internal/tui/preview`, and `internal/tui/app.go`, plus tests and docs.

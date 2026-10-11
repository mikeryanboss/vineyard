### 2026-10-11T16:05
[FINDINGS] Audit of mouse and keyboard handling on `origin/main` (1e2321c); see Context in the issue. Grapes's issues screen handles its own mouse and is left alone. The preview's scrollback does nothing for agents on the alternate screen: with Claude Code 2.1.296 and `"tui": "fullscreen"`, tmux reported `alternate_on=1 history_size=0`.

### 2026-10-11T16:05
[DECISION] Expanded sessions keep their count line, as `▾ N subagents`, so the mouse has a line to collapse them with; it costs one line per expanded session. A tab click acts like the `tab` key and keeps focus. Clicking outside a dialog does not cancel it, so a half-typed new session survives a stray click.

### 2026-10-11T16:05
[STARTED] Tabs, edge clicks, and preview scrollback in `app.go`; the count line in `list`; diff header and summary clicks; dialogs; config screen.

### 2026-10-11T16:16
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.2 s. Manual, development build in a 140×36 tmux window on a throwaway repository, two `claude --model haiku` sessions drawing fullscreen (`alternate_on=1`), clicks sent as SGR mouse sequences: a click on `▸ 3 subagents` selected the session and showed `▾ 3 subagents` and its rows, and a second click hid them; a click on a subagent row selected it; clicks on Diff, Recap, and Preview showed those tabs with the list still focused, and on Diff in a zoomed recap unzoomed it; a click on `▾ M shop.go` folded it in place and on `A shop_test.go` in the summary scrolled to it; the wheel over a Claude preview kept it live and showed "No scrollback…"; a click on `n cancel` closed the kill dialog with both sessions alive; on the config screen, clicks showed Profiles, then General, and two clicks on Auto-yes turned it on (left unsaved: no config.toml was written). PASS. Screenshots in the PR.

### 2026-10-11T16:16
[DONE] `internal/tui/app.go`: `handleMouse` routes by position (dialogs, config screen and picker, list, tab bar, pane content; header and status bar ignored), `tabLabels`/`tabAt`/`showTab`, `centerOffset`, the no-scrollback message; `list`: count line always shown and clickable; `diffview`: header and summary clicks; `preview`: no scroll mode without scrollback; `dialog`: clicks in `Pick`, `Confirm`, `NewSession`, wheel in `Pick`, agent hint; `settings`: clicks and wheel, category change resets the field; tests; `README.md`, `docs/README.md`, `docs/architecture.md`.

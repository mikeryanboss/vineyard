### 2026-10-11T16:05
[FINDINGS] Audit of mouse and keyboard handling on `origin/main` (1e2321c); see Context in the issue. Grapes's issues screen handles its own mouse and is left alone. The preview's scrollback does nothing for agents on the alternate screen: with Claude Code 2.1.296 and `"tui": "fullscreen"`, tmux reported `alternate_on=1 history_size=0`.

### 2026-10-11T16:05
[DECISION] Expanded sessions keep their count line, as `▾ N subagents`, so the mouse has a line to collapse them with; it costs one line per expanded session. A tab click acts like the `tab` key and keeps focus. Clicking outside a dialog does not cancel it, so a half-typed new session survives a stray click.

### 2026-10-11T16:05
[STARTED] Tabs, edge clicks, and preview scrollback in `app.go`; the count line in `list`; diff header and summary clicks; dialogs; config screen.

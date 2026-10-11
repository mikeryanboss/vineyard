### 2026-10-11T15:33
[FINDINGS] Review of #31 asked for subagents collapsed by default, with a count, and → to expand. → (`l`) currently gives the pane focus for scrolling. Claude Code 2.1.296 has no per-subagent status beyond being in its list: a finished subagent leaves it, except the one whose view is open, which stays with its timer stopped and is drawn like a running one (capture: `.grapes/32/tmp/finished-view.ansi`).

### 2026-10-11T15:33
[DECISION] The count takes its own line under the branch line, so a collapsed entry grows by one line when subagents start, not one per subagent; on the title or branch line it would truncate them at the list's 26-column minimum. → expands when there is something to expand and otherwise keeps giving the pane focus, and ← collapses: → goes in (rows, then the pane) and ← comes back out, as `h`/`←` already leave the pane. Expansion is kept per session until collapsed, so a redraw that briefly shows no subagents cannot collapse a session the user opened.

### 2026-10-11T15:33
[STARTED] Collapse subagents in `internal/tui/list`, bind ← and →, update the status bar and docs.

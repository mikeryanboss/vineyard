### 2026-10-11T15:33
[FINDINGS] Review of #31 asked for subagents collapsed by default, with a count, and → to expand. → (`l`) currently gives the pane focus for scrolling. Claude Code 2.1.296 has no per-subagent status beyond being in its list: a finished subagent leaves it, except the one whose view is open, which stays with its timer stopped and is drawn like a running one (capture: `.grapes/32/tmp/finished-view.ansi`).

### 2026-10-11T15:33
[DECISION] The count takes its own line under the branch line, so a collapsed entry grows by one line when subagents start, not one per subagent; on the title or branch line it would truncate them at the list's 26-column minimum. → expands when there is something to expand and otherwise keeps giving the pane focus, and ← collapses: → goes in (rows, then the pane) and ← comes back out, as `h`/`←` already leave the pane. Expansion is kept per session until collapsed, so a redraw that briefly shows no subagents cannot collapse a session the user opened.

### 2026-10-11T15:33
[STARTED] Collapse subagents in `internal/tui/list`, bind ← and →, update the status bar and docs.

### 2026-10-11T15:44
[DECISION] The count line says `▸ 4 subagents`, not "running": in the manual run, a finished subagent whose view was open stayed in Claude Code's list, drawn like the running ones.

### 2026-10-11T15:44
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 5.9 s. Manual, development build in a 140×36 tmux window on a throwaway repository, `claude --model haiku`, four background subagents waiting on files: the session showed `▸ 4 subagents` and the status bar `→ show subagents`; → showed four rows with the session still selected; Enter on `error messages` attached to its view (`Message @general-purpose…`); after detaching, ← collapsed the rows and selected the session; `l` and `h` did the same; → on an expanded session and on a session without subagents gave the pane focus; finishing three subagents left `▸ 1 subagent` (the one whose view was open). PASS. Screenshots in the PR.

### 2026-10-11T15:44
[DONE] `internal/tui/list`: subagents collapsed by default, `Expand`, `Collapse`, count line; `internal/tui/common/keys.go`: `ListKeys.Collapse` (h, ←); `internal/tui/app.go`: → expands before giving the pane focus, ← collapses, status bar hints; tests; `README.md`, `docs/README.md`, `docs/architecture.md`.

### 2026-10-11T13:30
[FINDINGS] Subagents run inside the parent `claude` process; Claude Code's subagent view is part of its own screen, reached with ↓, the agent list, and Enter. Agent teams can split tmux panes, but the user does not use them. Captures from Claude Code 2.1.296 are in `.grapes/31/tmp/` (`six-*.ansi`, `focus-*.ansi`, `inline-*.ansi`).

### 2026-10-11T13:30
[DECISION] Switching Claude's view whenever a row is selected was rejected: it would type into working agents while the user browses. Vineyard types only when the user presses Enter or `o` on a subagent row, right before attaching.

### 2026-10-11T13:30
[STARTED] Parse Claude's agent list from the status poll's screens, add subagent rows to the list, and open a subagent through Claude's keys before attaching.

### 2026-10-11T13:46
[DECISION] Rows Claude Code scrolls out of sight get labelled rows by place (`subagent 6`) instead of a `+N more` line, so every subagent can be opened. The list keeps the selected place through a poll with fewer rows: right after a detach, Claude Code redraws and one poll saw a single row, which moved the selection to the first subagent. Rows use tree glyphs (├ └) rather than ↳, which common terminal fonts lack.

### 2026-10-11T13:46
[FINDINGS] Testing in a real session found nested subagents: a subagent's own subagents are tree rows under it while its view is shown (`  ├ ◯ general-purpose  helper one`), and only counted otherwise (`◯ general-purpose (+2)  lead`). `Parse` now reads both; captures are in `internal/subagent/testdata/nested*.ansi`. Separately, Claude Code 2.1.296's workspace trust prompt preselects "No, exit", so Vineyard's auto-yes Enter on a trust prompt may now exit Claude; not changed here.

### 2026-10-11T13:46
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.2 s. Manual, development build in a 140×36 tmux window on a throwaway repository, `claude --model haiku`: six background subagents showed as five labelled rows and `subagent 6`; selecting rows sent no keys; Enter on the third and on `subagent 6` attached to `worker three` and `worker six` (Claude scrolled its list) with `Message @general-purpose…` focused; detaching kept the selection; opening `worker two` from `worker six`'s view kept a two-line draft; a coordinator's helpers appeared under it once its view was open, and Enter on `helper two` attached to its view; rows went when the subagents finished. PASS. Screenshots in the PR.

### 2026-10-11T13:46
[DONE] New `internal/subagent` (parse Claude Code's agent list, open a subagent through its keys); `tmux.Client.SendKeys`; `Backend.OpenSubagent`; subagent rows and selection in `internal/tui/list`; status poll and Enter/`o` handling in `internal/tui/app.go`; docs in `README.md`, `docs/README.md`, `docs/architecture.md`.

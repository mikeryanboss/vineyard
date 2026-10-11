### 2026-10-11T13:30
[FINDINGS] Subagents run inside the parent `claude` process; Claude Code's subagent view is part of its own screen, reached with ↓, the agent list, and Enter. Agent teams can split tmux panes, but the user does not use them. Captures from Claude Code 2.1.296 are in `.grapes/31/tmp/` (`six-*.ansi`, `focus-*.ansi`, `inline-*.ansi`).

### 2026-10-11T13:30
[DECISION] Switching Claude's view whenever a row is selected was rejected: it would type into working agents while the user browses. Vineyard types only when the user presses Enter or `o` on a subagent row, right before attaching.

### 2026-10-11T13:30
[STARTED] Parse Claude's agent list from the status poll's screens, add subagent rows to the list, and open a subagent through Claude's keys before attaching.

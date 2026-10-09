# Vineyard Project Guide

Short orientation for contributors and coding agents. Read this first; follow
links only as far as the task requires.

## What Vineyard Is

A Go terminal application that runs coding agents in parallel. A session is one
agent program in a detached tmux session, working in its own git worktree and
branch. The TUI lists sessions, mirrors the selected agent's screen, shows its
diff, and hands the terminal to tmux when the user attaches. Its issues screen is
the [grapes](https://github.com/Mibokess/grapes) issue tracker, embedded, and
links each issue to the sessions working on it.

## Runtime Mental Model

```text
main.go
  -> find the repository, load .vineyard/config.toml, lock .vineyard/
  -> load sessions.json and reconcile it with live tmux sessions
  -> load grapes for the repository's .grapes directory, if any
  -> run the root TUI model (internal/tui/app.go)
       polls tmux: selected screen (150ms), all screens (1s) -> running/ready
       polls git: selected diff, others' line counts (2s)
       runs lifecycle operations in commands -> result messages -> state
  -> sessions keep running in tmux after exit
```

## Where to Look

| Task | Start here | Then read |
| --- | --- | --- |
| CLI, flags, startup | `main.go` | `internal/config/config.go` |
| Session lifecycle (create, pause, resume, kill, push) | `internal/session/manager.go` | `internal/git/git.go` |
| Session fields, persistence, locking | `internal/session/session.go`, `store.go` | `lock_unix.go` |
| Running/ready detection, prompts | `internal/session/screen.go` | `applyStatus` in `internal/tui/app.go` |
| tmux commands | `internal/tmux/tmux.go` | |
| Diff collection | `internal/git/diff.go` | |
| Diff parsing, changed-word ranges | `internal/diff/diff.go` | |
| Diff rendering | `internal/tui/diffview/` | `spans.go` for highlighting and wrapping |
| Root model, keys, polling, layout | `internal/tui/app.go` | `internal/tui/backend.go` |
| Session list | `internal/tui/list/` | |
| Live preview and scrollback | `internal/tui/preview/` | |
| Dialogs | `internal/tui/dialog/` | |
| Config screen | `internal/tui/settings/` | `SaveConfigMsg` in `internal/tui/app.go` |
| Theme, key bindings, shared messages | `internal/tui/common/` | |
| Issues screen, sessions linked to issues | `internal/tui/app.go` | Grapes in `architecture.md` |

## Repository Map

```text
main.go                    CLI and TUI bootstrap
internal/config/           config schema, defaults, data directory layout
internal/git/              git CLI wrapper: worktrees, commits, pushes, diffs
internal/tmux/             tmux client on Vineyard's private socket
internal/session/          session model, lifecycle manager, store, screen detection
internal/diff/             unified diff parser
internal/tui/app.go        root Bubble Tea model
internal/tui/backend.go    the root model's seam to tmux, git, and storage
internal/tui/list/         session list view
internal/tui/preview/      terminal mirror view
internal/tui/diffview/     diff view
internal/tui/dialog/       new-session and confirmation dialogs
internal/tui/settings/     config screen
internal/tui/common/       theme, keys, messages
internal/tui/testutil/     golden-file helpers and fixtures
.grapes/                   this repository's own issues
```

## Invariants Worth Knowing

- Nothing blocks `Update`. Every tmux, git, or file operation runs in a command
  and reports back with a message. Building an `*exec.Cmd` for attach is the
  only exception, and it runs nothing.
- Session operations return updated `Session` values instead of mutating
  shared state; the root model applies them when their message arrives.
- Results are tagged with a session ID. A capture or diff for a session that is
  no longer the one shown must be dropped, never displayed.
- A screen counts as "still" only after two equal fingerprints. Saved statuses
  can be stale, so pending prompts and auto-yes depend on that, not on `Status`.
- Diffs never touch the worktree's index; the agent is working in it.
- Killing a session keeps its branch whenever the branch has commits.
- Every message the root model does not handle goes to the embedded grapes
  model, shown or not; its file watching and reloads depend on it.
- Vineyard uses its own tmux socket (`vineyard`, or `VINEYARD_TMUX_SOCKET`), so
  its key bindings never reach the user's own tmux server.

## Verify a Change

```sh
go test ./...
go vet ./...
```

For runtime design, read [architecture.md](architecture.md). For testing and
change recipes, read [development.md](development.md).

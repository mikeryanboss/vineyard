# Vineyard Project Guide

Short orientation for contributors and coding agents. Read this first; follow
links only as far as the task requires.

## What Vineyard Is

A Go terminal application that runs coding agents in parallel. A session is one
agent program in a detached tmux session, working in its own git worktree and
branch. The TUI lists sessions, mirrors the selected agent's screen, shows its
diff, its issue, and Claude Code's recaps of it, and hands the terminal to tmux when the user attaches. Its issues screen is
the [grapes](https://github.com/Mibokess/grapes) issue tracker, embedded, and
links each issue to the sessions working on it.

## Runtime Mental Model

```text
main.go
  -> find the repository, load .vineyard/config.toml
  -> reconcile sessions.json with git worktrees and live tmux sessions, under its lock
  -> load grapes for the repository's .grapes directory, if any
  -> run the root TUI model (internal/tui/app.go)
       polls tmux: selected screen (150ms), all screens (1s) -> running/ready
       reloads sessions.json (1s): other Vineyards on the repository change it
       polls git: selected diff, others' line counts (2s)
       runs lifecycle operations in commands -> result messages -> state
  -> sessions keep running in tmux after exit
```

## Where to Look

| Task | Start here | Then read |
| --- | --- | --- |
| CLI, flags, startup | `main.go` | `internal/config/config.go` |
| Session lifecycle (create, pause, resume, kill, push) | `internal/session/manager.go` | `internal/git/git.go` |
| Session fields, persistence, locking | `internal/session/session.go`, `store.go` | `leader.go`, `lock_unix.go` |
| Several Vineyards on one repository | Several Vineyards in `architecture.md` | `adopt` in `internal/tui/app.go` |
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
| Issue tab | `refreshIssue` in `internal/tui/app.go` | `internal/tui/textview/` |
| Prompt of a session started from an issue | `internal/prompt/`, `promptData` in `internal/tui/app.go` | templates in `internal/config/templates.go`, `ForIssue` in `internal/tui/dialog/` |
| Recap tab | `refreshRecap` in `internal/tui/app.go` | `internal/recap/`, Recaps in `architecture.md` |
| Release a version | `var version` in `main.go` | Releases in `development.md` |
| Re-record the README demo | `doc/vhs/demo.tape` | README Demo in `development.md` |

## Repository Map

```text
main.go                    CLI and TUI bootstrap
internal/config/           config schema, defaults, data directory layout
internal/git/              git CLI wrapper: worktrees, commits, pushes, diffs
internal/tmux/             tmux client on Vineyard's private socket
internal/session/          session model, lifecycle manager, store, screen detection
internal/diff/             unified diff parser
internal/recap/            Claude Code transcripts: a session's recaps, title, last prompt
internal/prompt/           rendering an issue session's prompt from a template
internal/tui/app.go        root Bubble Tea model
internal/tui/backend.go    the root model's seam to tmux, git, and storage
internal/tui/list/         session list view
internal/tui/preview/      terminal mirror view
internal/tui/diffview/     diff view
internal/tui/textview/     scrollable text pane of the issue and recap tabs
internal/tui/dialog/       new-session and confirmation dialogs
internal/tui/settings/     config screen
internal/tui/common/       theme, keys, messages
internal/tui/testutil/     golden-file helpers and fixtures
.grapes/                   this repository's own issues
doc/                       README demo GIF and its VHS tape
.github/workflows/         tag on version bump, release with GoReleaser
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
- Git owns worktree paths and live branches. Never save a path in
  `sessions.json`, and read a session's branch from its worktree before acting
  on it.
- Every message the root model does not handle goes to the embedded grapes
  model, shown or not; its file watching and reloads depend on it.
- Several Vineyards may share a repository. Change saved sessions one at a
  time through `Backend.Add`, `Replace`, or `Remove`, never by writing the
  whole list, and type into agents only when this process leads.
- Vineyard uses its own tmux socket (`vineyard`, or `VINEYARD_TMUX_SOCKET`), so
  its key bindings never reach the user's own tmux server.

## Verify a Change

```sh
go test ./...
go vet ./...
```

For runtime design, read [architecture.md](architecture.md). For testing and
change recipes, read [development.md](development.md).

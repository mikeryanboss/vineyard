# Vineyard Project Guide

Short orientation for contributors and coding agents. Read this first; follow
links only as far as the task requires.

## What Vineyard Is

A Go terminal application that runs coding agents in parallel. A session is one
agent program in a detached terminal of a multiplexer, tmux or Vineyard's own
server, working in its own git worktree and branch. The TUI lists sessions,
mirrors the selected agent's screen, shows its diff, its issue, and its
conversation with Claude Code's recaps, lists Claude Code's subagents under it
with their status, and hands the terminal to the multiplexer when the user
attaches. Its issues screen is the [grapes](https://github.com/Mibokess/grapes)
issue tracker, embedded, and links each issue to the sessions working on it.

## Runtime Mental Model

```text
main.go
  -> find the repository, load .vineyard/config.toml
  -> pick the multiplexer: tmux or Vineyard's server (multiplexer in config.toml)
  -> reconcile sessions.json with git worktrees and live terminals, under its lock
  -> load grapes for the repository's .grapes directory, if any
  -> run the root TUI model (internal/tui/app.go)
       polls the multiplexer: selected screen (150ms), all screens (1s) -> running/ready
       reloads sessions.json (1s): other Vineyards on the repository change it
       polls git: selected diff, others' line counts (2s)
       runs lifecycle operations in commands -> result messages -> state
  -> sessions keep running in the multiplexer after exit
```

## Where to Look

| Task | Start here | Then read |
| --- | --- | --- |
| CLI, flags, startup | `main.go` | `internal/config/config.go` |
| Session lifecycle (create, pause, resume, kill, push) | `internal/session/manager.go` | `internal/git/git.go` |
| Session fields, persistence, locking | `internal/session/session.go`, `store.go` | `leader.go`, `lock_unix.go` |
| Several Vineyards on one repository | Several Vineyards in `architecture.md` | `adopt` in `internal/tui/app.go` |
| Running/ready detection, prompts | `internal/session/screen.go` | `applyStatus` in `internal/tui/app.go` |
| tmux commands | `internal/tmux/tmux.go` | `Terminal` in `internal/tui/backend.go` |
| Choosing tmux or Vineyard's server | `multiplexers` in `main.go` | Choosing a Multiplexer in `architecture.md` |
| Vineyard's server | `internal/vterm/daemon.go`, `client.go` | `vterm.go`, `attach.go`; Vineyard's Server in `architecture.md` |
| Diff collection | `internal/git/diff.go` | |
| Diff parsing, changed-word ranges | `internal/diff/diff.go` | |
| Diff rendering | `internal/tui/diffview/` | `spans.go` for highlighting and wrapping |
| Root model, keys, polling, layout | `internal/tui/app.go` | `internal/tui/backend.go` |
| Mouse | `handleMouse` in `internal/tui/app.go` | TUI Message Flow in `architecture.md` |
| Session list | `internal/tui/list/` | |
| Live preview and scrollback | `internal/tui/preview/` | |
| Dialogs | `internal/tui/dialog/` | |
| Config screen | `internal/tui/settings/` | `SaveConfigMsg` in `internal/tui/app.go` |
| Theme, key bindings, shared messages | `internal/tui/common/` | `internal/config/keys.go`, Key Bindings in `architecture.md` |
| Issues screen, sessions linked to issues | `internal/tui/app.go` | Grapes in `architecture.md` |
| Issue tab | `refreshIssue` in `internal/tui/app.go` | `internal/tui/textview/` |
| Prompt of a session started from an issue | `internal/prompt/`, `promptData` in `internal/tui/app.go` | templates in `internal/config/templates.go`, `ForIssue` in `internal/tui/dialog/` |
| Terminal tab | `shellCmd` in `internal/tui/app.go` | Terminal Tab in `architecture.md` |
| Recap tab | `refreshRecap` in `internal/tui/app.go` | `internal/recap/`, Recaps in `architecture.md` |
| Subagent rows, their status, opening a subagent | `internal/subagent/` (`transcript.go` reads the transcripts, `subagent.go` the screen) | `openSubagent` in `internal/tui/app.go`, Subagents in `architecture.md` |
| A selected subagent's tabs | `refreshShown`, `refreshTranscript`, `shownIssues`, `renderAgentRecap` in `internal/tui/app.go` | `TreeDiff` in `internal/tui/backend.go`, Subagents in `architecture.md` |
| Releases | `.github/workflows/auto-tag.yml`, automatic on merge | Releases in `development.md` |
| Re-record the README demo | `doc/vhs/demo.tape` | README Demo in `development.md` |

## Repository Map

```text
main.go                    CLI and TUI bootstrap
internal/config/           config schema, defaults, data directory layout
internal/git/              git CLI wrapper: worktrees, commits, pushes, diffs
internal/tmux/             tmux client on Vineyard's private socket
internal/vterm/            Vineyard's own multiplexer: background server and its client
internal/session/          session model, lifecycle manager, store, screen detection
internal/diff/             unified diff parser
internal/recap/            Claude Code transcripts: a session's title, messages, and recaps
internal/subagent/         Claude Code's subagents: read from its transcripts and screen, opened by its keys
internal/prompt/           rendering an issue session's prompt from a template
internal/tui/app.go        root Bubble Tea model
internal/tui/backend.go    the root model's seam to the multiplexer, git, and storage
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
.github/workflows/         tag each program change, release with GoReleaser
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
  whole list. Type into agents on Vineyard's own initiative only when this
  process leads; typing the user asks for, such as opening a subagent, works
  in every Vineyard.
- Never press Esc in Claude Code unless the screen shows its agent list
  focused: anywhere else Esc interrupts the agent.
- Vineyard uses its own tmux socket (`vineyard`, or `VINEYARD_SOCKET`), so
  its key bindings never reach the user's own tmux server. Vineyard's server
  uses the same name.
- Change `protocol` in `internal/vterm/daemon.go` with any change to the
  server's calls: a running server outlives the Vineyard that started it, so
  a newer Vineyard may meet an older server.

## Verify a Change

```sh
go test ./...
go vet ./...
```

For runtime design, read [architecture.md](architecture.md). For testing and
change recipes, read [development.md](development.md).

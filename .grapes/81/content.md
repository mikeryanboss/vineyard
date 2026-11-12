## Goal
Agents in different harnesses (such as Claude Code and Antigravity `agy`) can
discover each other's program and exchange plain-text peer messages modeled on
Claude Code's cross-session messaging, while Claude-to-Claude sessions keep
using Claude Code's own `SendMessage`.

## Description
Two additions:

1. **`program` in `vineyard sessions`.** Each JSON entry includes `"program"`
   (`session.ProgramName(s.Program)`), alongside `"agents"` (live Claude Code
   registry names).
2. **`vineyard send <session> <message>`.** Sends a plain-text `<peer_message>`
   to another running Vineyard session in the repository, delivering at once
   when the recipient is idle at its prompt or queuing in `.vineyard/inbox/`
   and draining in the background once its current turn finishes.

## Context
Verified against the codebase:
- `sessions.go` (`listedSession`, `runSessions`, `claudeAgents`) prints `id`,
  `title`, `branch`, `status`, `issues`, `worktree`, and `agents`.
- `internal/session/manager.go`: `launchCommand` names worktree Claude Code
  sessions after `s.Branch` (`shellQuote`); `Manager.running` locates a
  session's live terminal across configured multiplexers and older Vineyard
  protocols (`ErrOlderMultiplexer`).
- `internal/session/screen.go`: `ProgramName`, `AwaitingPermission`,
  `AwaitingTrust`, and `ScreenHash`.
- `internal/subagent/subagent.go`: `Parse(screen)` reports `(list, ok)` for
  Claude Code's subagent list; when `ok`, `list.Shown == 0 && list.Focus < 0`
  means the main conversation is shown and the list is unfocused.
- `internal/tui/app.go`: `stillFor` (600 ms) proves an agent is waiting;
  `promptCmd` delivers text via `Paste`, a 150 ms pause, and `SendEnter`. While
  a user is attached (`tea.Exec`), the TUI's `Update` loop does not run status
  ticks, and sessions also outlive an exited TUI (`docs/README.md`).
- `internal/session/store.go` and `internal/session/lock_unix.go`: `Store` and
  `lock(path, wait)` synchronize `.vineyard/` state across processes.

Planned changes:
- `sessions.go`: add `Program string` (`json:"program"`) to `listedSession`.
- `internal/session/manager.go`: export `ShellQuote` (from `shellQuote`),
  `Session.Address()` (`Branch` for worktree sessions when non-empty, else
  `ID`), and `Manager.Running(s Session) (Session, bool, error)` (from
  `running`).
- `internal/session/inbox.go` (planned): per-session FIFO message queue in
  `.vineyard/inbox/<id>.json` guarded by `<id>.lock` and `<id>.deliver.lock`
  using a race-free two-lock handoff so no message is stranded or delivered
  twice; cleaned up on `Store.Remove`.
- `send.go` (planned) and `main.go`:
  - `vineyard send <session> <message>` resolves `<session>` by exact `ID` or
    unique `Branch` (failing explicitly on missing or ambiguous branch), and
    identifies the sender from `repo.Checkout` (`sender.Address()`, or `"main"`
    with no `reply` attribute when no single session owns the main checkout).
  - Refuses empty messages, messages over 1 MiB, self-messages, paused or
    stopped targets, and pairs where both checkouts have a live Claude Code
    agent in `claudeAgents()` (directing the caller to `SendMessage` with the
    target's Claude Code agent name).
  - Wraps the body in `<peer_message from="..." reply="vineyard send '...' '...'">...</peer_message>`.
  - Delivers immediately when the target's screen is non-blank, unchanged across
    600 ms, not awaiting trust or permission, has empty `PendingPrompt`, and (for
    `claude`) is on the main conversation with its subagent list unfocused;
    otherwise enqueues and spawns a detached `vineyard drain <id>` (`Setsid:
    true`) that polls every 1 s until ready or the session stops.
- `README.md`, `AGENTS.md`, `docs/README.md`, `docs/architecture.md`, and
  `writeHelp` in `main.go`: document `program` in `vineyard sessions` and
  `vineyard send` for cross-harness messaging.

## Acceptance Criteria
- [x] `vineyard sessions` includes `"program"` (`session.ProgramName`) on every listed session.
- [x] `vineyard send <session> <message>` resolves `<session>` by `id` or unique `branch`, and fails explicitly when `<session>` is missing, ambiguous, not running, or the sender itself, or when `<message>` is empty or exceeds 1 MiB.
- [x] When both sender and recipient have live Claude Code agents, `vineyard send` refuses with an error naming the recipient's Claude Code agent for `SendMessage`.
- [x] When the recipient is idle and ready for input, `vineyard send` delivers a `<peer_message>` block with `from` and `reply` attributes via bracketed `Paste` and `SendEnter`.
- [x] When the recipient is busy (screen changing, awaiting trust/permission, holding an unsent `PendingPrompt`, or viewing/focusing a Claude Code subagent), `vineyard send` queues the message without blocking and delivers it in FIFO order once the recipient becomes ready, even with no Vineyard TUI running or attached.
- [x] `help`, `README.md`, `AGENTS.md`, and `docs/` describe `vineyard send` and when to use Claude Code's `SendMessage` vs. `vineyard send`.

## Verify
Run from `/home/mboss/dev/vineyard/.vineyard/worktrees/coms-4190f5`:
```bash
gofmt -l . && go vet ./... && go test ./...
go build -o .grapes/81/tmp/vineyard . && .grapes/81/tmp/vineyard sessions | jq .
```
Also exercise `vineyard send` in a throwaway repository on a dedicated
`VINEYARD_SOCKET` (`docs/development.md`) against both an idle and a busy
terminal program in `vterm`.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under
10 seconds; `vineyard sessions` prints `"program"` for every session; idle and
busy recipients receive `<peer_message>` blocks in order; Claude-to-Claude calls
are refused with the `SendMessage` target name.

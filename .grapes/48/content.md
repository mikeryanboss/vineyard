## Goal
Vineyard works without tmux. When tmux is not on PATH, Vineyard runs each agent
on a pseudo-terminal in its own process and mirrors it with a built-in terminal
emulator, instead of refusing to start. Agents then stop when Vineyard exits;
resuming continues their Claude Code conversations (#47).

## Description
Today `runTUI` in `main.go` exits with "vineyard needs tmux; install it and try
again". Most of Vineyard is useful without a multiplexer: the list, preview,
diff, issue and recap tabs, attaching, shells, and subagents only need a
program on a terminal Vineyard can read and type into.

Operations that need a multiplexer fail with a message saying so instead of
the whole program failing. The one such operation is running several
Vineyards on a repository: only the process that started an agent can see it.

## Context
- tmux operations live in `internal/tmux/tmux.go` (`Client`): `Start`,
  `Exists`, `Created`, `Capture`, `CaptureHistory`, `Resize`, `SendEnter`,
  `SendKeys`, `Paste`, `Kill`, `AttachCommand`.
- `session.Terminal` (`internal/session/manager.go`) covers `Start`, `Exists`,
  `Kill`; `tui.LiveBackend` (`internal/tui/backend.go`) calls the rest on a
  `*tmux.Client` directly.
- Attaching runs `tea.ExecProcess` on an `*exec.Cmd` in `internal/tui/app.go`.
- `subagent.Open` presses `Down`, `Up`, `Enter`, `Escape`, named as tmux names
  them.
- Several Vineyards share sessions through the tmux server and lead through
  `session.Leader` (`internal/session/leader.go`).

Planned:
- `internal/vterm` (new): agents on PTYs (`github.com/creack/pty`), mirrored by
  `github.com/charmbracelet/x/vt`. Attaching passes the terminal through to the
  agent until ctrl-q, replaying the terminal modes the agent enabled.
- A `tui.Terminal` interface that both `*tmux.Client` and `*vterm.Server`
  implement; attaching returns a `tea.ExecCommand`.
- Without tmux, Vineyard claims the lead at startup and refuses to start when
  another Vineyard already runs on the repository, saying that needs tmux.

## Acceptance Criteria
- [x] With tmux on PATH, behaviour is unchanged.
- [x] Without tmux on PATH, Vineyard starts, and a new session's agent runs,
  shows in the preview with colours, has scrollback, receives prompts, and
  can be attached to and left with ctrl-q.
- [x] The shell key opens a shell in the session's worktree without tmux.
- [x] Without tmux, a second Vineyard on the same repository exits with an
  error that says several Vineyards need tmux.
- [x] Without tmux, quitting Vineyard ends its agents; on the next start they
  show as stopped and resume.
- [x] Tests cover the built-in terminal: start, capture, keys, paste, exit, kill.

## Verify
Run from the worktree:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual, in a real terminal with tmux hidden from PATH: start Vineyard, create a
session, watch the preview, attach and detach, open a shell, start a second
Vineyard, quit and restart and resume.

## Pass Criteria
Commands pass. Each manual step behaves as described in the criteria; PR has
screenshots of the preview and attached agent without tmux.

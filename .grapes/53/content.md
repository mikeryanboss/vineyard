## Goal
Without tmux, agents keep running after Vineyard quits, and several Vineyards
can share them, as with tmux. Vineyard runs its own background terminal
server in place of tmux. The multiplexer stays a choice: tmux remains
available and is selected in the configuration.

## Description
Today, without tmux, `internal/vterm` runs agents inside the Vineyard process
(#48). Quitting hangs them up, and only one Vineyard per repository may run.

Move `vterm.Server` into a background process, following tmux's design:

- The first Vineyard that starts a terminal starts `vineyard server <socket>`
  in its own session (`setsid`). The server takes a lock beside its socket
  (`<socket>.lock`, flock), as tmux does, so racing Vineyards start one server.
- The socket lives in `$TMPDIR/vineyard-<uid>/<name>`, a 0700 directory owned
  by the user, as tmux's lives in `/tmp/tmux-<uid>/<name>`. `<name>` is
  `VINEYARD_SOCKET`, default `vineyard`, which also names the tmux socket;
  it replaces `VINEYARD_TMUX_SOCKET`.
- Control calls (start, exists, capture, keys, paste, resize, kill) use
  `net/rpc` over the Unix socket.
- Attaching passes the user's terminal file descriptors to the server
  (`SCM_RIGHTS`), as tmux's client does. The server draws on them with the
  existing attach code; the client forwards SIGWINCH and waits for the detach.
  If the client's connection drops, the server stops the attachment.
- Agents get the environment of the Vineyard that started them.
- The server exits when it has neither terminals nor connections.
- A protocol version check fails explicitly when a server of another protocol
  version holds the socket, naming its process ID.

Configuration: `multiplexer = "tmux" | "vineyard"` in `.vineyard/config.toml`,
also on the config screen. Unset means tmux when it is installed, otherwise
Vineyard's server. Choosing tmux without tmux installed is an error. Startup
refuses to run when a saved session's terminal is still alive in the
multiplexer not chosen, because resuming it would run a second agent in the
same worktree.

## Context
Verified on main (851c4ea):
- `tui.Terminal` (`internal/tui/backend.go`) embeds `session.Terminal`
  (`internal/session/manager.go`); implemented by `*tmux.Client` and
  `*vterm.Server`.
- `main.go` `runTUI` picks `vterm.New()` when `exec.LookPath("tmux")` fails,
  claims the lead first, refuses a second Vineyard, and sets `Options.Notice`
  (`internal/tui/app.go`), which nothing else sets.
- `vterm.Server.Start` gives programs `os.Environ()`; `terminal.changed` has
  one reader, so only one attachment is woken per output.
- `vterm.attachment.Run` takes the user's terminal from `tea.ExecCommand`'s
  stdin and stdout and handles SIGWINCH itself.
- `tmuxSocket` in `main.go` reads `VINEYARD_TMUX_SOCKET`; docs and
  `doc/vhs/demo.tape` use it.

Planned:
- `internal/vterm/daemon.go`: `Serve(socket)`, the server process.
- `internal/vterm/client.go`: `Client`, a `tui.Terminal` over the socket.
- `config.Config.Multiplexer`; `settings` field "Multiplexer".
- Removed: the in-process mode (`Server.Close`, the lead claim in `main`,
  `Options.Notice`).

## Acceptance Criteria
- [x] With `multiplexer = "vineyard"`, an agent started by one Vineyard keeps
      running after it quits, and the next Vineyard shows it running.
- [x] Two Vineyards on one repository both run with Vineyard's server.
- [x] Attaching through the server draws the agent, passes keys, resizes with
      the user's terminal, and detaches on ctrl-q.
- [x] Two racing clients start a single server; a stale socket is replaced.
- [x] The server exits once it has no terminals and no connections.
- [x] A server of another protocol version fails with an error naming its PID.
- [x] Unset `multiplexer` picks tmux when installed, otherwise Vineyard's
      server; `multiplexer = "tmux"` without tmux fails explicitly.
- [x] Startup refuses to run when a saved session is alive in the other
      multiplexer.
- [x] `VINEYARD_SOCKET` names both sockets; docs describe the server.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual, in a real terminal driven through a private tmux socket (see
`docs/development.md`): set `multiplexer = "vineyard"` in a throwaway
repository, start a `bash` session, quit, relaunch, attach, resize, ctrl-q.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass in under 10 seconds. The
manual run shows the session running after relaunch and the attach behaving
as above, captured in screenshots for the PR.

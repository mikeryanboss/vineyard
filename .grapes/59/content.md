## Goal
Let an agent or a human find out why Vineyard or one of its sessions failed,
after the fact, with one bounded command, instead of reconstructing state from
`ps`, `ls`, `jq`, and screen scraping.

## Description
After #56 merged, the user saw the Vineyard multiplexer "crash immediately".
By the time an agent looked, it worked again, and nothing recorded what had
failed: the server's log was empty, the TUI keeps no log, and `vineyard debug`
prints only paths. Diagnosing it took `pgrep`, `ps`, `/proc`, the sessions
file, and driving a development build through tmux by hand. This issue adds
the records and the summary that would have answered it.

## Context
Verified current behavior:
- `runDebug` in `main.go` prints version, sockets, and data paths. It says
  nothing about whether Vineyard's server runs, its process, its protocol, the
  terminals on it, or whether each session's terminal is alive.
- `vterm.Client.startServer` (`internal/vterm/client.go`) sends the server's
  output to `<socket>.log`. The server writes only connection errors there,
  without timestamps (`internal/vterm/daemon.go`).
- When a terminal's program exits, `vterm.Server.Start`'s goroutine
  (`internal/vterm/vterm.go`) deletes the terminal. Its exit status and last
  screen are lost, so an agent that exits at once with an error leaves no trace.
- The TUI shows errors for a few seconds through `setError`
  (`internal/tui/app.go`) and records them nowhere. Bubble Tea prints a
  panic's stack to the terminal only.
- `docs/development.md` explains how to run a development build and capture
  screenshots, but not how to debug, and not the traps of driving Vineyard
  in an outer tmux: the outer server must not use the `VINEYARD_SOCKET` name,
  which also names Vineyard's tmux socket, and the pane must outlive Vineyard
  so its exit can be read.

## Acceptance Criteria
- [x] `vineyard debug` reports, without starting a server: whether Vineyard's
      server runs, its process, whether its protocol matches this build, and
      the terminals on it; the server log's path and last lines.
- [x] `vineyard debug` lists each session with its multiplexer, recorded
      status, and whether its terminal is alive.
- [x] The server log has timestamped lines for server start and stop, each
      terminal start (name, program), and each terminal exit (name, exit
      status, lifetime, last lines of its screen).
- [x] The TUI appends to `.vineyard/vineyard.log`: its start, every error it
      shows (consecutive repeats once), its exit error, and the stack of a
      panic in `Update` or `View`. `vineyard debug` prints its path and last lines.
- [x] `docs/development.md` has a Debugging section with the order of steps,
      and a committed helper drives a development build in tmux with captures
      filtered to non-blank lines.
- [x] Tests cover the debug report against a real server, the exit record, and
      the TUI log; the suite stays under 10 seconds.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then, with the helper, start a dev build with `multiplexer = "vineyard"`,
create a session whose program is `sh -c 'echo boom; exit 3'`, and run
`vineyard debug` in the demo repository.

## Pass Criteria
Checks pass. `vineyard debug` shows the server running with a matching
protocol, the session's terminal not alive, and the server log's last lines
include the exit with status 3 and the line `boom`.

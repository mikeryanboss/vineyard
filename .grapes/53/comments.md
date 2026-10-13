### 2026-10-13T10:55
[STARTED] Move `vterm.Server` into a background `vineyard server` process in tmux's design: Unix socket, flock beside it, net/rpc calls, attach by passing the user's terminal file descriptors (SCM_RIGHTS). Keep tmux selectable through `multiplexer` in config.toml.

### 2026-10-13T11:05
[DECISION] The server stops when it has neither terminals nor connections, but only once armed: after its first terminal starts or a 10s grace. Without arming, a server racing to start probed it and the probe's disconnect stopped it before its client connected (found by TestRacingVineyardsShareOneServer). Stopping removes the socket under the lock and only if it is still its own file (os.SameFile), so it never removes a successor's socket. `Serve` waits for the stop to finish before returning, or the process exited before removing the socket.

### 2026-10-13T11:05
[DECISION] Removed the in-process mode with its lead claim and `Options.Notice`: the server covers it. Startup refuses to run when a saved session is alive in the multiplexer not chosen, since resuming it would start a second agent in its worktree. `VINEYARD_SOCKET` replaces `VINEYARD_TMUX_SOCKET` and names both sockets.

### 2026-10-13T11:10
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test -count=1 ./...` PASS in ~7s wall time. `go test -race ./internal/vterm/` PASS five runs in a row. Manual run with the built binary in a throwaway repo, `multiplexer = "vineyard"`, driven through a private outer tmux socket: started a bash session, attached, typed, resized 110x30 to 90x24 (`stty size` reported 24 90), ctrl-q, quit Vineyard; `vineyard server` still ran; relaunch showed the session ready with its screen; a second Vineyard on the repo started; with `multiplexer = "tmux"` startup failed with "sessions survive quit still run in vineyard ..."; after kill and quit the server exited and removed its socket. PASS. Screenshots: .grapes/53/tmp/{attach,relaunch,config}.png.

### 2026-10-13T11:10
[DONE] New `internal/vterm/daemon.go` (Serve, Service, SocketPath) and `client.go` (Client, attach over the socket); `vterm.go` and `attach.go` adapted (per-start environment, several watchers per terminal, attach on passed files). `main.go`: `multiplexers`, `runningElsewhere`, `server` subcommand, `VINEYARD_SOCKET`. `config.Multiplexer`, config screen field. Docs: README, docs/README.md, architecture.md (Choosing a Multiplexer, Vineyard's Server), development.md, demo tape.

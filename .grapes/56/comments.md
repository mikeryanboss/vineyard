### 2026-10-13T11:45
[STARTED] Record the multiplexer on each session and route every terminal operation through it; make every available multiplexer at startup instead of choosing one; drop `runningElsewhere`. The user reported that refusal while this was planned: Vineyard would not open with a tmux session alive and `vineyard` selected.

### 2026-10-13T11:50
[DECISION] A session picks its multiplexer from its profile's `multiplexer`, else the configuration's, which defaults to `vineyard`. Profiles carry it because they already describe how a session runs (`main_checkout`); `m` on a profile row cycles it. An empty `multiplexer`, written by #53's "auto", loads as `vineyard`. A session in an unavailable multiplexer shows stopped, cannot start, and can be killed. Adopting a tmux session Vineyard did not start stays out of scope.

### 2026-10-13T11:55
[DECISION] Fixed a flaky #53 test while here: `TestRacingVineyardsShareOneServer` failed once in the full suite because `t.TempDir` cleanup ran while a server that lost the start race still wrote in the directory. `serverSocket` now waits for the server to stop and retries removing its directory.

### 2026-10-13T11:58
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test -count=1 ./...` PASS; `./internal/vterm` PASS 15 runs in a row. Manual run in a real terminal through a private outer tmux socket: #53's build with `multiplexer = "tmux"` created "old tmux session" (saved without a multiplexer); this build with `multiplexer = "vineyard"` started without error, recorded it as tmux, and created "new server session" on Vineyard's server; both listed, both attached and took input (tmux's status bar shows on the tmux one), both killed; then tmux had no server and Vineyard's server stopped. PASS. Screenshots: .grapes/56/tmp/{list-after,tmux-attach,config}.png.

### 2026-10-13T11:58
[DONE] `session.Session.Multiplexer`, `Manager.Terminals`/`Default`, `Restore` finds each session's multiplexer, `session.Unavailable`; `tui.LiveBackend.Terminals` with an `unavailable` fallback; `config.Profile.Multiplexer`, `Config.MultiplexerFor`, `vineyard` default, validation on load; config screen and new session dialog show multiplexers; `main.go` makes every available multiplexer and drops `runningElsewhere`. Docs: README, docs/README.md, architecture.md (Multiplexers per Session), development.md.

### 2026-10-13T12:05
[DECISION] Renumbered from #54: another agent's #54 (Enter on the issue tab) merged first. `grapes issue` saw only this checkout's issues because `.grapes/config.toml` lists no worktrees.

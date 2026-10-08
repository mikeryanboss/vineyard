### 2026-10-08T20:40
[DECISION] Rebuild in Go on grapes' stack instead of extending the TypeScript runtime. claude-squad's code is AGPL-3.0 and on Bubble Tea v1, so its behaviour is reimplemented rather than copied. Agent sessions use a private tmux socket (`vineyard`), and trust prompts are only auto-accepted with auto-yes.

### 2026-10-08T20:40
[VERIFY] `go vet ./...` clean; `go test -count=1 ./...` all ok, 2.0 s wall. End-to-end run of the real binary in a tmux terminal with a fake agent script: create with prompt (prompt delivered when idle), live preview, diff with syntax and add/remove colours confirmed in captured escape codes, attach and ctrl-q detach, shell, checkout and resume (work committed, worktree removed and restored), push failure reported, kill keeping a branch with commits, recovery after killing the agents' tmux server, scrollback. PASS.

### 2026-10-08T20:40
[FINDINGS] Attaching failed with `open terminal failed: can't use /dev/tty` while the TUI ran on descriptors from `tea.OpenTTY()`: tmux refuses a descriptor opened through /dev/tty. Vineyard now uses stdin and stdout when they are terminals. grapes' `main.go` uses the same `tea.OpenTTY()` plus `tea.ExecProcess` tmux attach, so it likely has the same bug (not reproduced there).

### 2026-10-08T20:40
[DONE] Replaced the TypeScript runtime with the Go TUI: `main.go`, `internal/{config,git,tmux,session,diff,tui}`, `docs/`. Not yet included: an auto-yes daemon after exit, a branch picker, a help overlay, diffs for paused sessions, and Grapes integration.

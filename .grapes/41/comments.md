### 2026-10-11T23:45
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.3 s. `TestLoad_SubagentsOfAnExitedClaudeCodeStopped` fails without the rule. Real terminal (dev build, haiku): a subagent showed `◑ second sleeper`; after `c`, `y`, then `r` at once, the row read `▾ 1 subagent stopped` / `× second sleeper`; a new prompt cleared it. A first try resumed within the same second as the pause: the killed subagent's last write landed in the second tmux reports as the new session's start, so the backend now counts the second after `session_created` as the start. Trace: `.grapes/41/tmp/pause-trace.log`. PASS.

### 2026-10-11T23:45
[DONE] `subagent.Reader.Load(dir, started)`: running subagents of a conversation not written since `started` are stopped. `Failed` renamed `Stopped`, shown as "stopped". `tmux.Client.Created`; `LiveBackend.Subagents` passes it plus a second. Docs: architecture Subagents.

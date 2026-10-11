### 2026-10-11T22:11
[DECISION] Hold a shorter list for one poll rather than detect a half-drawn screen: the frame Claude Code draws midway has everything but the agent list, so nothing on it tells a redraw from finished subagents. Cost: a finished subagent leaves the count one poll later.

### 2026-10-11T22:11
[DONE] `internal/tui/app.go`: `seeSubagents` and the `fewer` map; `applyStatus` uses it; pause, resume, and `adopt` forget a session's subagents. `internal/tui/app_test.go`: `TestApp_SubagentCountSurvivesAPollCatchingARedraw`. `docs/architecture.md`: Subagents.

### 2026-10-11T22:11
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.5 s. The new test fails with `seeSubagents` believing every poll ("one poll without the list should keep the count"), and with the resume reset removed ("a resumed session should show no subagents"). Real terminal (140×36 tmux, `claude --model haiku`, three background subagents), five attach/detach cycles each, sampled every 20 ms: `main` dropped the count in 5 of 5 cycles, from 0.06 s to about 1.03 s after the detach; the branch in 0 of 5. Stopping one subagent: Claude Code dropped it at 31.84 s, Vineyard at 33.48 s (1.64 s later). Logs: .grapes/38/tmp/trace-before.log, trace-after.log, trace-removal.log. PASS

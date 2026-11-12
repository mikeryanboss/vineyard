### 2026-11-12T16:58
[STARTED] Implementing cross-harness peer messaging (`vineyard send`, `internal/session/inbox.go`, and `"program"` in `vineyard sessions`) modeled on Claude Code's `<peer_message>` communications while keeping Claude-to-Claude on `SendMessage`.

### 2026-11-12T17:13
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` PASS in 7.24s. New tests: `TestInbox_HandoffAndStoreRemove`, `TestInbox_ConcurrentHandoffNeverStrandsMessages`, `TestRun_SendDeliversAcrossHarnessesWhenIdle`, `TestRun_SendRefusesInvalidCallsAndClaudeToClaude`, `TestRun_SendQueuesWhileBusyAndDeliversInOrderOnceReady`, plus `program` assertions in `TestRun_SessionsPrintsIssuesAndAgents` and `send` usage checks in `TestRun_CommandsAndUsageErrors`. End-to-end check with `.grapes/81/tmp/vineyard` via `scripts/drive` on `vineyard-drive-dev-coms-4190f5`: `vineyard sessions` printed `"program"` (`claude` / `agy`); `vineyard send mboss/beta "hello while busy"` printed `Queued for mboss/beta` and drained via detached `vineyard drain` once `beta` settled on `ready> `; `vineyard send mboss/beta "hello when idle"` printed `Delivered to mboss/beta` immediately, with both `<peer_message>` blocks recorded in order. PASS.

### 2026-11-12T17:14
[DONE] Added `send.go`, `send_test.go`, `internal/session/inbox.go`, `internal/session/inbox_test.go`; exported `Session.Address`, `ShellQuote`, and `Manager.Running`; added `program` to `sessions.go`; wired `send` and `drain` in `main.go`; updated `README.md`, `AGENTS.md`, `docs/README.md`, and `docs/architecture.md`.

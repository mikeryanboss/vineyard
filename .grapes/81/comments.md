### 2026-11-12T16:58
[STARTED] Implementing cross-harness peer messaging (`vineyard send`, `internal/session/inbox.go`, and `"program"` in `vineyard sessions`) modeled on Claude Code's `<peer_message>` communications while keeping Claude-to-Claude on `SendMessage`.

### 2026-11-12T17:13
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` PASS in 7.24s. New tests: `TestInbox_HandoffAndStoreRemove`, `TestInbox_ConcurrentHandoffNeverStrandsMessages`, `TestRun_SendDeliversAcrossHarnessesWhenIdle`, `TestRun_SendRefusesInvalidCallsAndClaudeToClaude`, `TestRun_SendQueuesWhileBusyAndDeliversInOrderOnceReady`, plus `program` assertions in `TestRun_SessionsPrintsIssuesAndAgents` and `send` usage checks in `TestRun_CommandsAndUsageErrors`. End-to-end check with `.grapes/81/tmp/vineyard` via `scripts/drive` on `vineyard-drive-dev-coms-4190f5`: `vineyard sessions` printed `"program"` (`claude` / `agy`); `vineyard send mboss/beta "hello while busy"` printed `Queued for mboss/beta` and drained via detached `vineyard drain` once `beta` settled on `ready> `; `vineyard send mboss/beta "hello when idle"` printed `Delivered to mboss/beta` immediately, with both `<peer_message>` blocks recorded in order. PASS.

### 2026-11-12T17:14
[DONE] Added `send.go`, `send_test.go`, `internal/session/inbox.go`, `internal/session/inbox_test.go`; exported `Session.Address`, `ShellQuote`, and `Manager.Running`; added `program` to `sessions.go`; wired `send` and `drain` in `main.go`; updated `README.md`, `AGENTS.md`, `docs/README.md`, and `docs/architecture.md`.

### 2026-11-12T17:54
[RESUMED] Adding `peer_messages` repository configuration toggle (`.vineyard/config.toml` and `C` config screen) that disables `vineyard send` and marks `send` disabled in `vineyard help` when off.

### 2026-11-12T18:00
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` PASS in 7.49s (`TestSettings_PeerMessagesToggles`, `TestRun_PeerMessagesDisabledMarksHelpAndStopsSendAndDrain`, updated `TestSettingsView_General.golden` and `TestAppView_Config.golden`). Exercised TUI Config screen (`C`) via `scripts/drive` on `.grapes/81/tmp/demo`: toggled `Peer messages` to `off` and saved with `ctrl+s` (`peer_messages = false` written to `.vineyard/config.toml`); `vineyard help` printed `Send a peer message to another running session (disabled in config.toml)` and `vineyard send` failed with `Error: peer messages are disabled in .vineyard/config.toml`. PASS.

### 2026-11-12T18:01
[DONE] Added `PeerMessages` (`peer_messages`, default `true`) to `internal/config/config.go` and the `C` config screen (`internal/tui/settings/settings.go`); refused `vineyard send` and discarded queued messages on `drain` when disabled (`send.go`); marked `send` disabled in `writeHelp` (`main.go`); updated `README.md` and `docs/architecture.md`.

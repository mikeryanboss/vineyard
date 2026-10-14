### 2026-10-14T15:20
[STARTED] Research in this session found peers see only derived names; four orphaned agents from killed sessions on older servers cluttered the list (the user killed them; reproducing that is separate work). Plan: `--name <branch>` at launch, and a `vineyard sessions` JSON command.

### 2026-10-14T15:45
[DECISION] `agents` is a list, not one name: nothing stops a second Claude Code process in the same worktree. Registry entries whose process is gone are skipped (`kill(pid, 0)`). `issueIDs` moved to `session.IssueIDs`, with `Session.Issues` shared by the TUI's `issuesOf` and the command. `--name` makes Claude Code write no `ai-title`, so the recap tab of such sessions shows no title; accepted, documented in architecture.md.

### 2026-10-14T15:50
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` PASS. New tests: `TestStart_NamesClaudeCodeAfterTheBranch` (quoting round-trips through `sh`), resume table in `TestResume_ClaudeContinuesItsConversation`, `TestRun_SessionsPrintsIssuesAndAgents` (recorded, committed, and uncommitted issues; dead and foreign registry entries ignored). Manual: dev build via `scripts/drive` with Claude Code's env vars unset (inherited `CLAUDE_CODE_CHILD_SESSION` turns off registration); new session "peer demo" showed `mboss/peer-demo` on Claude Code's prompt border, `ListAgents` listed `mboss/peer-demo [34f855]`, and `vineyard sessions` printed `"agents": ["mboss/peer-demo"]`. In this repository it prints this session with `"issues": [74]`, `"agents": ["othersessions-16282e-3d"]`. Earlier: `SendMessage` to `73/peer-name-test` delivered. PASS.

### 2026-10-14T15:52
[DONE] `launchCommand` and `shellQuote` in `internal/session/manager.go`; `sessions.go` and the `sessions` command in `main.go`; `Session.Issues`, `IssueIDs`; `recap.ConfigDir`; README, docs, AGENTS.md.

### 2026-10-14T15:25
[DECISION] Renumbered from #73, which another agent's merged PR #63 already took (created in a worktree `grapes issue` could no longer see). The experiment name `73/peer-name-test` is kept as run.

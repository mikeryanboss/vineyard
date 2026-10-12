### 2026-10-12T22:58
[STARTED] Save the conversation ID the status poll already reads, and resume it with `claude --resume <id>` when its transcript exists. Chose this over `--continue` (picks whatever is newest at restart) and over assigning `--session-id` at launch (goes stale after `/clear`).

### 2026-10-12T23:10
[FINDINGS] `claude --resume <unknown id>` prints "No conversation found with session ID: …" and exits 1 (Claude Code 2.1.296), so `Resume` checks the transcript exists first. `claude --resume <id>` touches the transcript at startup (mtime 23:01:38 → 23:01:46, size unchanged at 173550 bytes), which made the old all-or-nothing `exited` rule in `Reader.Load` report a subagent killed with the tmux server as running.

### 2026-10-12T23:10
[DECISION] Judge each subagent by its own transcript's mtime against Claude Code's start. A subagent the current process runs was started after it, so its transcript is newer; one resumed by a message appends to its transcript. Only the leading Vineyard saves the conversation ID, like prompt delivery.

### 2026-10-12T23:12
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test ./...` PASS in 6.9s. New tests fail on the old code: dropping the `conversation` JSON field fails `TestResume_ClaudeContinuesItsConversation`; letting followers save fails `TestApp_LeaderSavesTheConversationToResume`; the old subagent rule fails the resume case of `TestLoad_SubagentsOfAnExitedClaudeCodeStopped`. PASS

### 2026-10-12T23:12
[VERIFY] Real terminal, dev build, throwaway repo, `-p claude`: told the agent the codeword PINEAPPLE-47, had it start a subagent running `sleep 900`, quit Vineyard, ran `tmux -L vy-47 kill-server`, reopened: session stopped. r started `claude --resume 092be8a5-71ee-40fc-8b83-d3a24a520d12` (`pane_start_command`), the preview showed the earlier prompts, the row read "1 subagent stopped / × long sleep" (the old rule read "◑ long sleep" running on the same state), and asked for the codeword the agent answered PINEAPPLE-47. Screenshots: `.grapes/47/tmp/{stopped,resumed,answered}.png`, published in the PR. PASS

### 2026-10-12T23:12
[DONE] `internal/session/{session,manager}.go`: `Session.Conversation`, `resumeCommand`. `internal/subagent/transcript.go`: `Conversation.ID`, per-subagent exit rule. `internal/tui/app.go`: the leader saves a changed conversation ID. Tests in each package; `docs/architecture.md` and the README key table updated.

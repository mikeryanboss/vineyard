## Goal
Resume Antigravity CLI (`agy`) conversations when `Manager.Resume` restarts a paused or stopped session, matching how Claude Code sessions continue their conversation across pauses and devcontainer rebuilds while staying pure Go (`CGO_ENABLED=0`, no SQLite dependency).

## Description
Previously `resumeCommand` in `internal/session/manager.go` returned `launchCommand(s)` whenever `ProgramName(s.Program) != "claude" || s.Conversation == ""`, and `Session.Conversation` was only populated from Claude Code subagent transcript polling in `internal/tui/app.go`. When a devcontainer rebuild stops the terminal server, or when a user pauses and resumes a session, an `agy` profile such as `agy --dangerously-skip-permissions` restarted with a blank conversation.

Resolve the active `agy` conversation by scanning `<gemini-dir>/antigravity-cli/history.jsonl` (where `<gemini-dir>` is `$ANTIGRAVITY_GEMINI_DIR` when non-empty, otherwise `~/.gemini`) for the latest entry whose `workspace` matches `s.WorktreePath` exactly and has a non-empty `conversationId`. Never pass `agy --continue` or `-c`, which fall back to parent and child directories and would cross-contaminate main-checkout and worktree sessions.

Record `s.Conversation` during leader status polling via `Backend.Conversation` in `internal/tui/app.go` and also resolve from `history.jsonl` inside `resumeCommand` (falling back to `s.Conversation` if `history.jsonl` has no matching entry) so conversations survive even if the container stopped before the next status poll. When `<gemini-dir>/antigravity-cli/conversations/<id>.db` exists on disk, relaunch with `s.Program + " --conversation " + ShellQuote(id)`; otherwise fall back to `launchCommand(s)`.

## Context
- `internal/session/session.go`: `Session.Conversation` (`conversation` JSON field).
- `internal/session/manager.go`: `Manager.Resume`, `resumeCommand`, `launchCommand`, `ShellQuote`; add `antigravityDir`, `AgyConversation`, `agyConversation`.
- `internal/session/screen.go`: `ProgramName`.
- `internal/tui/backend.go`: `Backend` interface and `LiveBackend`; add `Conversation(s session.Session) (string, error)`.
- `internal/tui/app.go`: `statusCmd` and `applyStatus` populate and save `s.Conversation` on the leader.
- `internal/session/session_test.go` and `internal/tui/app_test.go`: unit tests using `ANTIGRAVITY_GEMINI_DIR` and `fakeBackend`.
- `docs/architecture.md` and `README.md`: document `agy` conversation resume behavior.

## Acceptance Criteria
- [x] `ANTIGRAVITY_GEMINI_DIR` overrides `~/.gemini` when resolving `<gemini-dir>/antigravity-cli/`.
- [x] Scanning `history.jsonl` matches `workspace == s.WorktreePath` exactly, skips lines without a non-empty `conversationId`, handles large prompt lines (>64 KB), and ignores conversations from parent `<repo>` and sibling worktrees.
- [x] `Manager.Resume` for `ProgramName(s.Program) == "agy"` relaunches with `s.Program + " --conversation " + ShellQuote(id)` when `<gemini-dir>/antigravity-cli/conversations/<id>.db` exists on disk, even if `s.Conversation` was not yet populated before stopping, and records the resumed ID on `s.Conversation`.
- [x] `Manager.Resume` falls back to `launchCommand(s)` when no matching conversation or `.db` file exists on disk, and never uses `agy --continue` or `-c`.
- [x] Leader status polling in `internal/tui/app.go` records `s.Conversation` for `agy` sessions via `Backend.Conversation` without rendering Claude subagents.
- [x] Unit tests in `internal/session/session_test.go` and `internal/tui/app_test.go` cover these behaviors, and `docs/architecture.md` and `README.md` are updated.

## Verify
Run from `/var/home/mboss/dev/vineyard/.vineyard/worktrees/agy-resume-c4dd5e`:
```bash
gofmt -l .
go vet ./...
CGO_ENABLED=0 go build -o .grapes/85/tmp/vineyard .
CGO_ENABLED=0 go test -v ./internal/session -run TestResume_Agy
CGO_ENABLED=0 go test -v ./internal/tui -run TestApp_StatusPollSavesAgyConversation
time CGO_ENABLED=0 go test -count=1 ./...
```

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` exits 0; `CGO_ENABLED=0 go build` succeeds; `TestResume_AgyContinuesItsConversation` and `TestApp_StatusPollSavesAgyConversation` pass; `CGO_ENABLED=0 go test -count=1 ./...` passes all packages in under 10 seconds.

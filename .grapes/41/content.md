## Goal

After a session's Claude Code exits, as a pause, resume, or crash ends it, its subagents no longer show as running.

## Description

Subagents run inside Claude Code's process, so they end with it. Claude Code queues no task notification for them then, so the transcripts (#39) still read as running. Pausing kills the session's tmux session; resuming starts a new one running a fresh Claude Code, which writes no transcript until the first prompt. Until then the newest transcript in the worktree's directory is the old conversation's, and its subagents show as running.

Verified on 2.1.296 in a scratch repository (`.grapes/39/tmp/kill/demo`): a subagent running `sleep 8` was listed; killing the tmux session left the conversation transcript last written at the kill (1791753934) and the subagent's a second before, with no notification. A new Claude Code in a new tmux session (`session_created` 1791754006) wrote no transcript; `Reader.Load` reported the subagent running.

## Context

- `internal/subagent/transcript.go`: `Reader.Load`, `Status` (`Running`, `Done`, `Failed`).
- `internal/tmux/tmux.go`: the session runs the agent program directly (`new-session … program`), so tmux's `session_created` is when that Claude Code started.
- `internal/tui/backend.go`: `LiveBackend.Subagents`.
- `internal/tui/list/list.go`, `internal/tui/app.go`: status words.

## Acceptance Criteria

- [x] A conversation whose transcripts were all last written before the session's Claude Code started has no running subagents: those still running count as stopped, ended at their last message.
- [x] A conversation written to since then is read as before.
- [x] A subagent that stopped without completing reads "stopped" (×): failed, stopped by the user, or ended with its Claude Code.
- [x] Docs describe it.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual: in tmux with a development build, a session running a subagent is paused (`c`) and resumed (`r`); before any new prompt its row shows the subagent stopped, and a new prompt clears it.

## Pass Criteria

gofmt prints nothing; vet and tests pass in under 10 s; the manual run shows ◑ before the pause and × after the resume.

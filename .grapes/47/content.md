## Goal
Pressing r on a stopped or paused Claude Code session reopens the conversation
its agent was in, so the agent remembers its task instead of starting empty.

## Description
A session stops when its tmux session ends: the agent exited, or the tmux
server died, as when the WSL VM shuts down. `Resume` then runs the session's
program again, plain `claude`, which starts a new, empty conversation. The
user has to explain the task again or find the old conversation with
`/resume`.

Vineyard already reads each Claude session's current conversation on every
status poll, for the subagent rows. It will save that conversation's ID in the
session, and `Resume` will start `claude --resume <id>` while the
conversation's transcript is still on disk.

## Context
- `internal/session/session.go`: `Session` is what `sessions.json` saves.
  Planned: a `Conversation` field, the ID of the Claude Code conversation the
  agent was last seen in.
- `internal/session/manager.go`: `Resume` starts `s.Program` when the tmux
  session is missing. Planned: it appends `--resume <id>` when the program is
  `claude` (`ProgramName`), an ID is saved, and
  `recap.Dir(projects, worktree)/<id>.jsonl` exists.
- `internal/subagent/transcript.go`: `Reader.Load` reads the newest
  `*.jsonl` in the session's transcript directory, the conversation Claude
  Code is writing. Planned: `Conversation.ID` is that file's name without
  `.jsonl`.
- `internal/tui/app.go`: `applyStatus` stores each poll's conversation.
  Planned: the leading Vineyard saves a changed conversation ID with
  `replaceCmd`. Only the leader writes, so several Vineyards on one
  repository do not race; the others adopt the saved value on reload.
- Verified with Claude Code 2.1.296: `claude --resume <unknown id>` prints
  "No conversation found with session ID: …" and exits 1, which would end the
  tmux session at once. Hence the transcript check: without it, a session
  whose transcript Claude Code cleaned up could never restart.
- `--resume` reuses the conversation's ID unless `--fork-session` is passed
  (`claude --help`). If it ever wrote a new transcript, the next poll would
  save the new ID.
- `/clear` starts a new conversation; it becomes the newest transcript, so the
  poll follows it.
- Found while verifying: `claude --resume` touches the conversation's
  transcript as it starts. `Reader.Load` counted a subagent as stopped only
  when neither the conversation nor any subagent transcript had been written
  since Claude Code started, so subagents that died with the previous Claude
  Code showed as running after a resume. Each subagent is now judged by its
  own transcript: one not written to since Claude Code started ran in an
  earlier process.

## Acceptance Criteria
- [x] The leading Vineyard saves a Claude session's current conversation ID in
      `sessions.json` and updates it when the conversation changes.
- [x] `Resume` of a Claude session with a saved conversation whose transcript
      exists starts `<program> --resume <id>`, keeping the program's flags.
- [x] `Resume` starts the plain program when no ID is saved, when the
      transcript is gone, or when the program is not `claude`.
- [x] In a real terminal, a Claude session whose tmux session was killed
      reopens its earlier conversation on r.
- [x] Subagents that ended with the previous Claude Code show as stopped after
      their conversation is resumed.
- [x] `docs/architecture.md` describes the saved conversation, the resume
      command, and the subagent rule.

## Verify
Run from the worktree root:
```bash
gofmt -l .
go vet ./...
go test ./...
```
Manually: run a development build in a throwaway repository with
`claude` as the program, send the agent a prompt, kill its tmux session on the
development socket, press r, and look at the preview.

## Pass Criteria
`gofmt -l .` prints nothing, `go vet` and `go test` pass. After r, the
preview shows the earlier prompt and answer, and `tmux list-panes -F
'#{pane_start_command}'` shows `claude --resume <id>`.

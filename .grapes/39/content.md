## Goal

Each subagent row says whether the subagent is working, done, or failed, and a finished subagent stays listed, marked done, until the user sends the conversation a new prompt.

## Description

Vineyard reads subagents off Claude Code's screen (`internal/subagent.Parse`). That list holds only running subagents: Claude Code drops a finished one at once, except the one whose view is open. The screen shows no per-subagent status, and a finished subagent's row vanishes with nothing left to look at.

Claude Code also keeps the transcripts (verified on 2.1.296):
- The conversation: `<projects>/<dirName(worktree)>/<conversation>.jsonl`, the most recently written one being current, as `recap.Load` assumes. A prompt the user sent is a `user` line with `origin.kind = "human"`. When a subagent stops, a `queue-operation` `enqueue` line, and later a `user` line with `origin.kind = "task-notification"`, carry `<task-notification><task-id>ID</task-id>…<status>completed</status>`.
- Each subagent: `<projects>/<dir>/<conversation>/subagents/agent-<ID>.jsonl`, plus `agent-<ID>.meta.json` with `agentType`, `description`, `toolUseId`, and `spawnDepth`. Its first line is the prompt it was given. A resumed subagent (the parent's SendMessage) appends to the same file.
- A subagent another one started is launched by an `Agent` tool use, whose ID is the child's `toolUseId`, in the parent's transcript.

User decisions (2026-10-11): keep finished subagents until the next prompt, marked done.

## Context

- `internal/subagent/subagent.go`: `Parse`, `List`, `Open`. Opening needs a subagent's position in Claude Code's on-screen list.
- `internal/tui/list/list.go`: `Item.Subagents subagent.List`, `renderSubagent`, the count line.
- `internal/tui/app.go`: `statusCmd` parses each Claude session's screen; `applyStatus` stores `m.subagents`; `openSubagentCmd`.
- `internal/recap/recap.go`: `ProjectsDir`, `dirName`.

## Acceptance Criteria

- [ ] A session's subagent rows come from its current conversation's transcripts: each subagent followed by those it started, in launch order, labelled with its description.
- [ ] A subagent is done when a notification for it is newer than its transcript's last line; failed when that notification's status is not `completed`; running otherwise, also again after a resume.
- [ ] Rows show the status with an icon, and the count line counts by status (`▸ 3 subagents: 2 running, 1 done`).
- [ ] A finished subagent stays listed until a prompt from the user newer than its end; a running one always shows.
- [ ] Transcripts are read incrementally: a poll reads only the lines appended since the last one.
- [ ] `enter`/`o` on a running subagent opens it at its position in Claude Code's list, also when Claude Code keeps a finished subagent's view open in that list; when the list cannot be matched, or the subagent has finished, Vineyard says so instead of pressing keys.
- [ ] Docs (`docs/architecture.md` Subagents, README keys/mouse text if affected) describe the transcript source.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in tmux with a development build and `claude --model haiku`: launch background subagents of different lengths; rows show running, then done as each finishes; done rows stay; a new prompt clears them; `o` opens a running subagent whose neighbour finished.

## Pass Criteria

gofmt prints nothing; vet and tests pass in under 10 s; the manual run shows the statuses, the kept rows, their removal on a new prompt, and the right subagent opening.

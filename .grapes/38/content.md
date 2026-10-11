## Goal

A session's subagent count stays on screen while Claude Code redraws, instead of disappearing for a second and popping back, as the user sees after every detach.

## Description

The user reported that the subagents "always take 1-2 seconds to pop up". Traces in a real terminal (140×36 tmux, `claude --model haiku`, three background subagents) show where:

- On spawn, Vineyard shows the count about 0.3 s after Claude Code lists the subagents; on startup, 0.03 s after the session's row. Both are within one status poll (1 s).
- After a detach (`o`, then ctrl-q), the count goes `▸ 3 subagents` → none → (1.0 s later) `▸ 3 subagents`. Sampling Claude Code's pane every ~3 ms right after a detach found 3 of 528 captures, 15–21 ms after it, where Claude Code had redrawn everything except its agent list, which it draws last. The status poll that runs right after the detach catches such a frame, `subagent.Parse` finds no list, and `applyStatus` replaces the session's subagents with none until the next poll.

Fix: a poll that shows fewer subagents than are shown (including none) is believed only when the next poll agrees; until then the previous list stays. More subagents, or as many, apply at once. A finished subagent then leaves the list about one poll (1 s) later.

## Context

- `internal/tui/app.go`: `statusCmd` captures each active session and runs `subagent.Parse`; `applyStatus` stores the result in `m.subagents[id]`; `syncList` passes it to the list.
- `internal/tui/list/list.go`: `SetItems` keeps the selected subagent's place through a poll that sees fewer rows (`TestList_SubagentSelectionSurvivesARedraw`).
- `docs/architecture.md`, Subagents: notes that Claude Code draws fewer rows for a moment while it redraws, as after a detach.
- Pause and resume (`lifecycleMsg`, and `adopt` for another process's) restart Claude Code but keep `m.subagents`, so a resumed session would show its old count until polls replace it.

## Acceptance Criteria

- [x] One poll that shows fewer subagents, or none, between polls showing them does not change the count.
- [x] Two such polls in a row do change it.
- [x] More subagents show at the first poll that sees them.
- [x] Pausing or resuming a session forgets its subagents at once.
- [x] In a real terminal, the count does not drop across several detaches; before the fix, it did.
- [x] `docs/architecture.md` describes the rule.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual: build `main` and the branch into `.grapes/38/tmp/`, run each in a 140×36 tmux window on a demo repository with a `claude --model haiku` session running three background subagents, and trace Vineyard's count and Claude Code's listed rows every ~20 ms across five attach/detach cycles.

## Pass Criteria

`gofmt` prints nothing; vet and tests pass, with a test that fails without the fix. The trace of `main` shows the count dropping after a detach; the branch's trace shows it staying at the number Claude Code lists.

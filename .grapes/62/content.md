## Goal
After the user detaches from a session, the list shows every session's
running/ready status and its subagents within well under a second, instead of
the 1–2 s it takes now.

## Description
`tea.Exec` blocks bubbletea's event loop while the user is attached
(bubbletea v2.0.8 `tea.go`, `case execMsg: // NB: this blocks.`), so no status
poll runs during the attachment and the list shows the statuses from before it.
After the return:

1. `attachDoneMsg` (`internal/tui/app.go`) refreshes the preview but not the
   statuses; they wait for the next `statusTickMsg`, up to `statusInterval`
   (1 s) away.
2. `applyStatus` calls a session ready only when two consecutive polls, a
   `statusInterval` apart, see the same screen. The first poll after the
   return compares with a screen from before the attachment, which has changed
   (if only because the agent's window was resized to the attached terminal
   and back), so it says running, and ready takes one or two more seconds.

Change:

1. On `attachDoneMsg`, poll every status at once.
2. Follow it with quick polls (`quickInterval`, 100 ms, for `quickChecks`
   polls, one self-paced chain like the other ticks), and decide ready by time rather than by poll count: a screen is
   still once it has stayed unchanged for `stillFor` (600 ms) since the poll
   that first saw it. Polls a second apart decide as before; polls closer
   together can no longer call a busy agent ready.

`stillFor` comes from measurement: sampling three running Claude Code agents
every 10 ms for 60 s, their screens never stayed unchanged longer than 454 ms
(p50 110 ms), so 600 ms leaves a margin while halving the 1 s window.

## Context
- `internal/tui/app.go`: `statusInterval`, `statusTickMsg`, `statusCmd`,
  `statusMsg`, `applyStatus`, `attachDoneMsg`, `Model.hashes`.
- `internal/tui/app_test.go`: status tests send `statusMsg` directly; they
  must carry sample times now.
- `docs/architecture.md` Status Detection describes the rule.
- Out of scope: a brief "running" for the session just detached from, when
  its window's resize back to preview size redraws the agent after a poll.

## Acceptance Criteria
- [x] Detaching starts a status poll immediately, without waiting for a tick.
- [x] Quick polls follow for about a second after detaching.
- [x] A session is ready only after its screen stayed unchanged for `stillFor`;
      two equal polls closer together leave its status alone.
- [x] A changed screen still marks a session running at once.
- [x] Polls a `statusInterval` apart behave as before (existing tests pass).
- [x] Docs describe the rule.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manually: run a development build with two Claude sessions, one working and one
idle, attach to one, wait several seconds, detach, and watch the list.

## Pass Criteria
Checks print nothing from `gofmt -l`, and vet and tests pass. In the manual
run, statuses and subagents update right on return, and an idle agent shows
ready within about 0.7 s of the return.

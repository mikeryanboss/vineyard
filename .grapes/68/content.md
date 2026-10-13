## Goal
The status bar of an attached terminal (Vineyard's server) names the session
attached to first, so the user can always tell which session they are in.

## Description
The bar lists only the other sessions. The attaching Vineyard now also sends
the attached session's title, and the bar draws it in bold at the left, then a
separator, then the other sessions (or their counts, or "no other sessions").

## Context
- `internal/session/manager.go`: `Attachment` gains `Title` (JSON `title`;
  only add fields, older executables ignore unknown ones).
- `internal/tui/app.go`: `attachment` sets `Title` to the session's title, for
  the agent and its shell alike.
- `internal/vterm/daemon.go`: `AttachRequest` gains `Title`; `protocol` rises
  from 4 to 5, since attaching changed. Sessions on a protocol 4 server attach
  through its own executable and keep the old bar.
- `internal/vterm/client.go`: sends `Title`.
- `internal/vterm/bar.go`: `newBar`, `bar.text`.
- tmux's status line is out of scope: it already shows the tmux session name.

## Acceptance Criteria
- [x] The bar starts with the attached session's title, bold, followed by `│` and the other sessions.
- [x] When the other sessions' titles do not fit, the title stays and the counts follow it.
- [x] With no other sessions, the bar shows the title, then "no other sessions".
- [x] The TUI sends the session's title when attaching to its agent or its shell.
- [x] `protocol` raised to 5; docs updated.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then attach to a session in the real app and capture the bar.

## Pass Criteria
`gofmt` prints nothing, vet and tests pass; the attached terminal's last row
starts with the session's title.

## Goal
`ctrl-q` returns to the session list from every screen, pane, and dialog, as
it already does from an attached agent or shell, so one key always gets the
user back.

## Description
Today the key that leaves a view depends on the view:

- Attached agent, subagent view, or shell (tmux, or `internal/vterm` without
  tmux): only `ctrl-q` returns; every other key goes to the program.
- Issues screen (embedded grapes): `q` returns; `ctrl-q` does nothing and
  `esc` only goes back inside grapes.
- Focused or zoomed pane: `esc`, `h`, `←`, or `q`; not `ctrl-q`.
- Config screen: `esc` (one level at a time); not `q` or `ctrl-q`.
- Dialogs: `esc`; `q` in the confirm and pick dialogs; not `ctrl-q`.

Change: `handleKey` in `internal/tui/app.go` handles `ctrl+q` before routing
keys anywhere else. It closes any dialog, the config screen, and the issues
screen, leaves the pane, and unzooms it, leaving the user on the session list.
On the session list `ctrl-q` does nothing; it never quits, so a second press
after detaching is harmless. Other keys keep their meaning.

## Context
- `internal/tui/app.go`: `handleKey` routes keys to the dialog, settings,
  grapes, the focused pane, then the list; `ctrl+c` is already handled first.
  `setFocus(focusList)` exits pane scrolling; `setZoom(false)` unzooms.
  Closing a dialog is `m.dialog = nil` (`DialogCancelledMsg`); closing config
  discards unsaved edits, as `esc` does.
- `internal/tmux/tmux.go`: `DetachKey = "C-q"`; `internal/vterm/attach.go`:
  `detachKey = 0x11`. Unchanged.
- `internal/tui/testutil/testutil.go`: `Key` needs a `ctrl+q` entry.
- Docs: `README.md` key table, `docs/architecture.md` grapes key routing.

## Acceptance Criteria
- [x] `ctrl-q` returns to the session list from the issues screen, also while
      grapes' search takes text input.
- [x] `ctrl-q` returns to the session list from a dialog, including a dialog
      over the issues screen.
- [x] `ctrl-q` returns to the session list from the config screen, also from
      its field pane.
- [x] `ctrl-q` returns to the session list from a focused pane and a zoomed
      pane.
- [x] `ctrl-q` on the session list does not quit.
- [x] README and architecture docs describe `ctrl-q`.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
go test ./internal/tui -run CtrlQ -v
```
Manual: development build in a throwaway repo (`docs/development.md`), press
`ctrl-q` in the issues screen, config screen, new-session dialog, focused and
zoomed pane, and on the list; attach with `o` and detach with `ctrl-q`.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass. The `CtrlQ` test passes for
every case above. Manually, each `ctrl-q` lands on the session list and the
one on the list leaves Vineyard running.

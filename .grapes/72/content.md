## Goal
The user can change a session's title after creating it, with `R` on the
session list, so the list keeps saying what each session is for once its work
changes or its random title ("amber riesling") no longer helps.

## Description
`R` opens a dialog with one text field holding the selected session's title.
Enter saves the new title; esc cancels. The new title shows in the list, the
attached terminal's status bar on the next attach, and every message that
names the session, and is saved to `sessions.json`, so other Vineyards on the
repository pick it up on their next reload.

Only the title changes. The session's ID, worktree directory, branch, and
terminal name were derived from the title at creation and stay as they are:
git and the multiplexer own them, and the agent may have switched branches.

## Context
- `Session.Title` in `internal/session/session.go` is display-only after
  creation: `newID` and the branch use it only in `Manager.Create`. Nothing
  looks a session up by title.
- `internal/config/keys.go`: `DefaultKeys` list mode; `R` is unbound there.
  Planned action: `rename`.
- `internal/tui/common/keys.go`: `ListKeys`, `NewKeyMap`.
- `internal/tui/dialog/dialog.go`: planned `Rename` dialog beside
  `NewSession`, `Confirm`, `Pick`; planned `common.RenameMsg`.
- `internal/tui/app.go`: the list key switch (`keys.AutoYes` saves a field
  through `m.replace` and `m.replaceCmd`), `ready`, status bar hints.
- A lifecycle operation in flight returns the session as it started
  (`lifecycleMsg` replaces it), so a rename during it would be lost. `R`
  goes through `ready`, which refuses busy sessions. The rename applies to the
  session as it is when the dialog submits, not as it was when it opened.
- `README.md` lists the default keys; `TestREADMEListsTheDefaultKeys` checks.

## Acceptance Criteria
- [x] `R` on a session opens a dialog prefilled with its title.
- [x] Enter with a non-empty title renames the session in the list and saves
      it; the branch, ID, and terminal name are unchanged.
- [x] Enter with an empty title keeps the dialog open and renames nothing;
      esc cancels.
- [x] `R` on a busy session reports it busy and opens no dialog.
- [x] `keys.list.rename` rebinds or unbinds it; the README lists it.
- [x] The status bar on the list hints `R rename` after auto-yes, when the
      terminal is wide enough, as it does the other list keys.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manually: run a development build with `scripts/drive`, press `R` on a
session, type a new title, press enter, and check the list; quit and restart
to see it kept.

## Pass Criteria
`gofmt -l .` prints nothing, `go vet` and `go test` pass in under 10 s, and
the driven build shows the new title in the list before and after a restart.

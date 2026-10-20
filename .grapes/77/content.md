## Goal
The user can put the sessions in the list in the order they want, and every
Vineyard on the repository shows that order.

## Description
A move mode: `m` in the list starts moving the selected session, `j`/`k` and
`↓`/`↑` move it one place, `enter` keeps the new order and saves it, and `esc`
puts the session back where it was. While moving, the status bar shows only
these keys, and other keys and the mouse do nothing.

Drag and drop with the mouse is out of scope; it can reuse `Store.Arrange`
later.

## Context
- The list shows the sessions in the order `sessions.json` saves them:
  `adopt` in `internal/tui/app.go` keeps the saved order, then appends
  sessions not saved yet; `syncList` builds the list from `m.sessions` in
  order. `Store.Add` appends new sessions. Nothing sorts them.
- Several Vineyards share `sessions.json` and reload it every status tick
  (`statusTickMsg`, `reloadedMsg`). `Store.Add`, `Replace`, and `Remove`
  change one session under `sessions.lock`; the invariant in `docs/README.md`
  forbids writing the whole list.
- A reload starts only with no write pending and is dropped when a write was
  issued while it ran (`m.writes`, `m.pendingWrites`). During a move nothing
  is written until `enter`, so a reload would undo the move in progress.
- A session with status `loading` in `m.sessions` is not saved yet: it is
  added to the store on `startedMsg`, and `adopt` puts unsaved sessions last.
- `ready()` refuses busy and loading sessions with "X is busy.".
- Keys: `config.DefaultKeys` (`internal/config/keys.go`) lists modes and
  actions; `common.NewKeyMap` (`internal/tui/common/keys.go`) binds them and
  panics when they disagree; `Config.ResolvedKeys` rejects a key two actions
  of one mode share. `m` is free in the list mode. README's default keys
  block must equal `DefaultKeys` (`TestREADMEListsTheDefaultKeys`).
- Clicks and the wheel reach `handleMouse`; release and motion go to grapes.
- `list.Model.SetItems` keeps the selection on the same session ID, so the
  selection follows a moved session.

## Plan
- `session.Arrange(sessions, ids)` (planned) sorts sessions into the order of
  `ids`; sessions `ids` does not name come last, in their order.
  `Store.Arrange(ids)` (planned) applies it under the lock, so sessions
  another Vineyard added or removed meanwhile are kept or stay gone.
  `Backend.Arrange` calls it.
- Key `list.move = ["m"]` and a new mode `move`: `up = ["k", "up"]`,
  `down = ["j", "down"]`, `done = ["enter"]`, `cancel = ["esc"]`.
- Root model: `moving` (the session's ID) and `moveStart` (the order, by ID,
  when the move began). `m` on a ready session selects the session (not a
  subagent row) and starts moving it; up/down swap it with its neighbour and
  stop at the ends and at a session still loading; `enter` saves the order
  through `Arrange` if it changed; `esc` and `global.back` restore
  `moveStart`. While moving, other keys and the mouse do nothing, reloads do
  not start, and a reload result is dropped. Removing the moved session ends
  the move.
- The list marks the session being moved, and the status bar shows the move
  keys.

## Acceptance Criteria
- [ ] `m` then `j`/`k` moves the selected session down/up one place, and the
      selection stays on it.
- [ ] The session stops at the top and bottom, and does not pass a session
      that is still starting.
- [ ] `enter` saves the new order to `sessions.json`; another Vineyard's
      reload shows it. A session another Vineyard added during the move is
      kept.
- [ ] `esc` and `ctrl+q` restore the order the move began with and save
      nothing.
- [ ] While moving, a reload does not undo the move, and other keys and
      clicks do nothing.
- [ ] `m` on a busy or starting session says it is busy and starts no move.
- [ ] The moved session is marked in the list, and the status bar shows the
      move keys; the list's hints include `m move`.
- [ ] README key table and default keys block, `docs/architecture.md` (Key
      Bindings, Several Vineyards), and the `docs/README.md` invariant on
      writes describe the move and `Arrange`.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual, with `scripts/drive` on a throwaway repository with three `-p bash`
sessions: `m`, `j`, `enter` moves the first session to second place; quit
and restart, and the order is kept. `m`, `j`, `esc` leaves the order as it
was. Screenshot the list while moving.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass. In the running app the
moved order survives a restart, and `esc` leaves the order unchanged.

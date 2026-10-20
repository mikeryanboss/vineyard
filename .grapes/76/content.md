## Goal
Moving the session list's selection past either end wraps around, so the
first session is one key away from the last.

## Description
Today `j`/`↓` on the last row and `k`/`↑` on the first session do nothing.
Down from the last row should select the first session; up from the first
session should select the last row.

## Context
- `move` in `internal/tui/list/list.go` steps through sessions and their
  shown subagent rows and stops at both ends. `Update` calls it for
  `common.Keys.List.Up`/`Down` and for the mouse wheel.
- The last row is the last session's last subagent row when that session is
  expanded, else the session itself.
- The wheel keeps stopping at the ends: one flick sends a burst of wheel
  events, which would carry the selection past the end and around again.
- `TestList_NavigationClamps` in `internal/tui/list/list_test.go` asserts the
  current clamping and changes with this issue.
- The README's key table does not list the navigation keys.

## Acceptance Criteria
- [x] Down on the last row selects the first session, and the list scrolls
      it into view.
- [x] Up on the first session selects the last row: the last subagent of an
      expanded last session, else the last session.
- [x] The mouse wheel still stops at both ends.
- [x] The README's key table lists the navigation keys and says they wrap.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual: run Vineyard in a terminal with several sessions, press `j` past the
last session and `k` past the first, and wheel past both ends.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass. In the running app, `j` on
the last session selects the first and `k` on the first selects the last,
while the wheel stops at the ends.

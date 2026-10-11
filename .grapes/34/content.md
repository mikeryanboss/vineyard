## Goal

Make a double click do what `enter` does where it lands: attach to a session, open a subagent's view, open the shown tab. A single click only selects.

## Description

#33 made Vineyard's screens clickable, but a click on a session or subagent only selects it; acting on it still needs the keyboard. Bubble Tea reports single clicks only, so the root model tells a double click from two clicks on the same cell within half a second.

Where a double click lands on the main screen:

- A session row: what `enter` does there (attach on Preview, the diff tool on Diff, the issue on Issue, the zoomed recap on Recap).
- A subagent row: open its view and attach, as `o` does, on every tab.
- A tab: show it, as one click does, then `enter`.
- The pane's content on Preview, Issue, or Recap: `enter` for that tab. On Diff, a click already folds a file or jumps to one, so the second click is ignored.
- A session's `▸`/`▾ N subagents` line: the first click already expanded or collapsed it, so the second click is ignored instead of undoing it.

## Context

Verified on `origin/main` at 2c54598:

- `handleMouse` in `internal/tui/app.go` routes clicks by position; dialogs, the issues screen, and the config screen get theirs first. List clicks go to `list.Update`, whose `click` selects a row and toggles a session's subagents on the count line. Pane clicks give the pane focus and go to its content; a click on the tab bar calls `showTab`.
- `handleKey` does what `enter` (`keys.Open`) and `o` (`keys.Attach`) do from the list: attach, open a subagent (`openSubagentCmd`), open the diff tool, show issues, zoom the recap. With the pane focused, keys go to the pane instead.
- Timers are a package variable (`tick`) so tests control them.

Planned:

- `Model.lastClick` (time and cell of the last click on the list or pane) and a `now` variable for tests; a click counts as double when it lands within one column on the same line within 500 ms; a third click starts over.
- Clicks that dialogs, the issues screen, or the config screen take clear `lastClick`, so the click that closes a dialog cannot pair with one on the screen beneath.
- `list.Model.CountLine(y)` tells the count line apart.
- A double click runs `enter`, or `o` on a subagent row, through `handleKey` with the list focused.

## Acceptance Criteria

- [ ] A double click on a session attaches on Preview and zooms the recap on Recap; on a subagent row it opens the subagent's view and attaches, on any tab.
- [ ] A double click on a tab shows it and does `enter` for it; on Preview, Issue, or Recap content it does `enter` for the tab.
- [ ] A double click on the count line expands or collapses once; on Diff content it folds a file once and opens nothing.
- [ ] Two clicks further apart than 500 ms, or on different cells, are two single clicks; a click that closes a dialog never makes a double click with the next one.
- [ ] Tests cover each of the above.
- [ ] `README.md` and `docs/architecture.md` describe double clicks.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in a real terminal: on a throwaway repository with Claude Code sessions and background subagents, double-click a session, a subagent, a tab, the preview, the count line, and a diff header.

## Pass Criteria

`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under 10 seconds. In the manual run each double click does what the criteria say.

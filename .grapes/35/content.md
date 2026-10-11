## Goal

Make a double click in the diff pane do what `enter` does for the Diff tab, open the full diff, as a double click in the preview attaches.

## Description

#34 made double clicks act like `enter`, but dropped every double click in the diff pane, because a single click there already folds a file (on its header) or jumps to it (on its summary line). The user reported that double-clicking in the diff view does not work; everything else did.

A double click on the diff's code, rules, or blank lines now opens the full diff: the diff tool, or the zoomed diff when the tool is missing, as `enter` from the list does. A double click on a file header or summary line still only folds or jumps, once.

Which of the two a double click is depends on its first click: a jump scrolls the view, so the second click lands on a different line.

## Context

Verified on `origin/main` at 0910bc6, in a real terminal with `diff_command = 'hunk diff {base} --watch'`: a double click on a header folded it once, on the Diff tab it opened hunk, and on a code line it did nothing.

- `handleMouse` in `internal/tui/app.go`: `case double && m.tab == tabDiff: return m, nil`.
- `diffview.Model.click` folds on a header (`fileStarts[i]+1`) and jumps on a summary line (`summaryFiles+i`).
- `doubleClick` returns only whether the click made a double; `lastClick` holds the first click's place and time.

Planned:

- `diffview.Model.Clickable(y)`: whether a click on line y of the view folds or jumps.
- `clickAt.acted`: set for a diff click that folds or jumps; `doubleClick` returns the first click too, and a double click on the diff is dropped only when its first click acted.

## Acceptance Criteria

- [ ] A double click on a diff code line opens the full diff, as `enter` from the list does.
- [ ] A double click on a file header folds it once; on a summary line it jumps once; neither opens the full diff.
- [ ] Tests cover both.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in a real terminal with a diff tool configured: double-click a diff code line, a file header, and a summary line.

## Pass Criteria

`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under 10 seconds. In the manual run the code line opens the diff tool, and the header and summary line only fold and jump.

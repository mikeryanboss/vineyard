## Goal

Show a Claude Code session's subagents as one line counting them, and show their rows only when the user expands the session with → (`l`). Rows that appear unasked, a second or more after the subagents start, push the rest of the list down.

## Description

#31 added a row per running subagent under its session, always shown. The status poll reads them once a second, so a session's entry grows by several lines shortly after Claude Code starts its subagents, and shrinks again when they finish.

This issue collapses every session's subagents by default:

- A collapsed session with subagents shows one line under its branch line: `▸ 3 subagents running` (`1 subagent running`). It belongs to the session's row: it is highlighted with it, and clicking it selects the session.
- → or `l` on a session with collapsed subagents expands them: the rows from #31 replace the count. Anywhere else → and `l` keep their job of giving the pane focus for scrolling.
- ← or `h` in the list collapses the selected session's subagents and selects the session.
- A session stays expanded until the user collapses it, also while it has no subagents.
- The status bar names the key that expands or collapses the selected session.

## Context

Verified on the branch's base (`origin/main` at bfcf8f0):

- `internal/tui/list/list.go`: `Item.height()` is `3 + Subagents.Len()`; `row`, `move`, `click`, and `renderItem` use `Subagents.Len()` as the number of subagent rows.
- `internal/tui/common/keys.go`: `ListKeys.Focus` is `l`/`right` ("scroll pane"); `h` and `left` are unbound in the list. In the pane, `PaneKeys.Back` (`esc`, `h`, `left`, `q`) returns to the list.
- `internal/tui/app.go` `handleKey`: `keys.Focus` gives the pane focus (`setFocus(focusPane)`), which highlights the pane's border and sends j/k, ctrl+d/u, g/G to the pane. `renderStatusBar` lists `l scroll`.
- Claude Code 2.1.296 tells running subagents apart from finished ones only by leaving finished ones out of its list. The one whose view is open stays after it finishes, its timer stopped, drawn like a running one (`.grapes/32/tmp/finished-view.ansi`). So the count is the only status Vineyard can read for a collapsed session.

Planned:

- `list.Model.expanded` (session IDs); `Expand() (Model, bool)` and `Collapse()`; the entry height and the subagent rows depend on it.
- `ListKeys.Collapse` (`h`, `left`); `handleKey` expands on `keys.Focus` before falling back to focusing the pane.
- Status bar: `→ show subagents` on a collapsed session with subagents, `← hide subagents` on an expanded one.
- Docs: `README.md` keys and the Subagents section of `docs/architecture.md`.

## Acceptance Criteria

- [ ] A session with subagents shows `▸ N subagents running` under its branch line and no subagent rows, until expanded.
- [ ] → and `l` on a collapsed session with subagents expand it, and the selection stays on the session; on any other row they give the pane focus, as before.
- [ ] ← and `h` collapse the selected session from its session row or any of its subagent rows, and select the session.
- [ ] j/k, the mouse wheel, and clicks skip a collapsed session's subagents; clicking the count line selects the session.
- [ ] Enter and `o` on an expanded session's subagent row still open that subagent's view and attach.
- [ ] The status bar shows `→ show subagents` or `← hide subagents` for the selected session when either applies.
- [ ] Tests cover the collapsed rendering, expanding and collapsing, navigation past collapsed rows, and → falling back to the pane.
- [ ] `README.md` and `docs/architecture.md` describe collapsed subagents and the keys.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in a real terminal: start a development build on a throwaway repository with `claude` as the program, ask the agent to run several background subagents, and check each criterion above. Capture screenshots of the collapsed and expanded list for the PR.

## Pass Criteria

`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under 10 seconds. In the manual run, the session first shows the count line only, → shows the rows, ← hides them, and Enter on a row attaches to that subagent's view.

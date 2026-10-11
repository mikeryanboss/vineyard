## Goal

Let the mouse do on Vineyard's own screens what the keyboard does, starting with the pane's tabs, and fix the mouse and keyboard behaviour an audit of every screen finds wrong.

## Description

The user asked for clickable tabs (Preview, Diff, Issue, Recap) and a check of every mouse and keyboard interaction. Today the mouse can only select sessions, focus the pane, and scroll.

## Context

Audit of `origin/main` at 1e2321c (verified):

- `handleMouse` in `internal/tui/app.go` gets clicks and wheel events. A click left of `listWidth` goes to the list (selects a row and focuses the list); any other click only focuses the pane; the wheel scrolls the list selection or the shown pane. Clicks are dropped while a dialog or the config screen is open. Clicks on the header rows, the status bar, or a too-small window are not filtered: a click on the status bar row can select an entry scrolled out of view.
- Tabs are drawn in `renderPane` and switched only with `tab`/`shift+tab` (`switchTab`).
- The list (`internal/tui/list`): a collapsed session's `▸ N subagents` line selects the session when clicked; nothing expands or collapses subagents by mouse, and an expanded session has no line to click.
- Diff (`internal/tui/diffview`): every file card has a header line with a ▾/▸ chevron; `enter` folds the file at the top of the view. Clicks are ignored, also on the summary's per-file lines.
- Preview (`internal/tui/preview`): wheel up, or k, ctrl+u, g in the focused pane, asks tmux for 5,000 lines of scrollback and freezes the preview on it. An agent that draws on the alternate screen, such as Claude Code with `"tui": "fullscreen"`, has none: tmux reported `alternate_on=1 history_size=0`, and the capture held only the visible rows. The preview then freezes on a copy of the screen that does not scroll.
- Dialogs (`internal/tui/dialog`): `Pick`, `Confirm`, and `NewSession` take only keys. `NewSession.Hints` offers `←/→ agent` whenever there are several agents, even while the title field has focus, where ←/→ move the cursor instead.
- Config screen (`internal/tui/settings`): keys only, including its option picker.
- The issues screen is grapes, which has its own mouse support; Vineyard passes it every mouse event while it is open. Out of scope.

## Changes

- Clicking a tab shows it, as `tab` does: focus stays where it is, and a zoomed pane unzooms.
- Sessions with subagents always show the count line: `▸ N subagents` collapsed, `▾ N subagents` expanded, above the rows. Clicking it expands or collapses the session and selects it. → and ← are unchanged.
- Diff: clicking a file header folds or unfolds that file in place; clicking a file in the summary scrolls to it.
- Clicks on the header rows and the status bar do nothing, and so does every click while the window is too small.
- Preview: when the scrollback holds no more than the screen, Vineyard stays live and says so in the status bar instead of freezing.
- Dialogs: clicking a `Pick` choice chooses it, and the wheel moves through the choices; clicking `y confirm` or `n cancel` in a `Confirm` does that; in `NewSession`, clicking a field focuses it, `‹` and `›` step through templates or agents, and the sub-issues box toggles.
- Config screen: clicking a category or field selects it, and clicking the selected field edits it, as `enter` does; the wheel moves the selection; clicking an option in the picker chooses it. Clicks are ignored while a value is being typed.
- Keyboard: `NewSession` offers `←/→ agent` only while the agent field has focus. On the config screen, moving to another category starts its fields at the first one; before, the old field's place could leave a short screen scrolled down the new category's fields.

## Acceptance Criteria

- [x] Clicking each tab shows it, focus unchanged; in a zoomed pane it also unzooms.
- [x] Clicking a session's subagent count line expands or collapses it and selects the session; expanded sessions show `▾ N subagents` above their rows.
- [x] Clicking a diff file header folds or unfolds that file without moving the view; clicking a summary line scrolls to its file.
- [x] Clicks on the header rows, the status bar, or a too-small window change nothing.
- [x] Scrolling up in the preview of an agent without scrollback keeps the preview live and shows a status message.
- [x] `Pick`, `Confirm`, and `NewSession` respond to clicks as listed; the wheel moves `Pick`'s choice.
- [x] The config screen and its picker respond to clicks and the wheel as listed, and ignore clicks while a value is typed.
- [x] `NewSession` offers `←/→ agent` only on the agent field; a new category on the config screen starts at its first field.
- [x] Tests cover each of the above.
- [x] `README.md` and `docs/` describe the mouse.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in a real terminal: start a development build on a throwaway repository with Claude Code sessions and background subagents, then click each tab, the subagent count line, diff headers and summary lines, a confirm dialog, the new-session dialog's fields, and the config screen and its picker; scroll the preview of a fullscreen Claude Code session.

## Pass Criteria

`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under 10 seconds. In the manual run each click does what the criteria say.

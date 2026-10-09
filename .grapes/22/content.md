## Goal
Let shift+tab cycle the pane tabs backward, so reaching the previous tab takes one key instead of three.

## Description
`tab` cycles the right pane forward through preview, diff, issue, and recap. Nothing cycles backward. Bind `shift+tab` to the previous tab wherever `tab` switches tabs: the session list and the focused pane.

## Context
- `internal/tui/app.go`: `nextTab` computes the next tab; `switchTab` applies it. The list handles `ListKeyMap.Tab` directly; the pane handles `common.SwitchTabMsg`.
- `internal/tui/common/keys.go`: `ListKeys.Tab` and `PaneKeys.Tab` bind `tab`.
- `internal/tui/preview/preview.go`, `diffview/diffview.go`, `textview/textview.go`: send `common.SwitchTabMsg` on `PaneKeyMap.Tab`.
- `internal/tui/testutil/testutil.go`: `Key` has no `shift+tab` entry yet.
- The settings screen and dialogs keep their own `tab` handling; they are out of scope.

## Acceptance Criteria
- [x] From the list, `shift+tab` on the preview tab shows the recap tab, and again shows the issue tab.
- [x] With the pane focused, `shift+tab` switches to the previous tab in each pane (preview, diff, issue/recap text view) and keeps pane focus.
- [x] README documents `shift+tab`.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run vineyard in a terminal and press `shift+tab` from the list and from the focused pane.

## Pass Criteria
`gofmt` prints nothing, vet and tests pass, and `shift+tab` steps backward through the tabs in the running app.

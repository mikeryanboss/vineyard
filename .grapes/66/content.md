## Goal
The default key that hides and shows an attached terminal's status bar can
be typed on a Swiss keyboard.

## Description
The default, `ctrl+\` (#61), needs a backslash, which a Swiss keyboard types
with AltGr, so ctrl+\ is out of reach. The key must be ctrl and a letter or
ctrl+\, since attached terminals find it as one byte in the input. Claude
Code leaves a, k, w, and q free (q is detach). The user chose ctrl+k.

## Context
- `DefaultKeys` in `internal/config/keys.go` (`global.status_bar`).
- README: key table, config example, default keys block (checked by
  `TestREADMEListsTheDefaultKeys`), How it works.
- `docs/architecture.md`: the tmux key table name
  (`vineyard-detach-C-q-bar-C-backslash`).
- `TestApp_AttachListsTheOtherSessionsInTheStatusBar` expects the default
  keys in the attachment.
- Risk: VS Code's terminal may take ctrl+k as the start of its two-key
  shortcuts (`terminal.integrated.allowChords`); the user chose it anyway
  and will try it.
- ctrl+\ stays a valid key to configure.

## Acceptance Criteria
- [x] `global.status_bar` defaults to `ctrl+k`, and the bar's hint says `ctrl+k hide`.
- [x] README and architecture name ctrl+k.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then attach in the real app and press ctrl+k.

## Pass Criteria
Checks pass; ctrl+k hides and shows the bar in the real app.

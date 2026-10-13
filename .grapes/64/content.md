## Goal
The status bar on an attached terminal (#61) looks right on light terminals
too, and says so when there is nothing to list.

## Description
The bar has one fixed palette: background 238 (#444444) with light text. On
a light terminal, such as the user's Solarized light, it reads as a dark grey
stripe ("it's just like a gray bar"). With no other sessions to list, it shows
only the key hints at the far right, so it looks empty.

The server cannot tell the user's background, but the TUI can: Bubble Tea
reports it (`tea.BackgroundColorMsg`, `common.Theme.IsDark`). The attaching
Vineyard sends it, and the bar picks a light or a dark palette.

## Context
- `internal/vterm/bar.go`: `barStyle`, `hintStyle`, `iconStyles`, `bar.text`.
- `internal/vterm/daemon.go`: `AttachRequest`, `newBar` call; `protocol` (3)
  must rise with any change to attaching. Protocols differ only in attaching,
  so `sameCallsSince` stays.
- `internal/session/manager.go`: `Attachment`, filled by `attachment` in
  `internal/tui/app.go`.
- The theme is light until the terminal reports its background
  (`NewModel`), which is the TUI's own assumption too.

## Acceptance Criteria
- [x] On a light terminal the bar has a light background (256-colour 254) with dark text and darker status colours; on a dark terminal it keeps its colours.
- [x] The attaching Vineyard sends whether its terminal is dark (`session.Attachment.Dark`, `AttachRequest.Dark`).
- [x] With no session to list, the bar says "no other sessions".
- [x] `protocol` raised to 4; docs updated.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then capture the attached bar in the real app with a light and a dark
background, and render both.

## Pass Criteria
Checks pass; the screenshots show a light bar with readable dark text on a
light background, and the dark bar on a dark one.

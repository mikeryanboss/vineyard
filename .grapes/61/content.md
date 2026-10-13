## Goal
While attached to an agent full-screen, see which other sessions are running
and which have finished, without detaching to the session list.

## Description
The attached screen gets a one-row status bar at its bottom. On Vineyard's
server it lists the repository's other live sessions on that server: running
(◑), ready (●), and finished (✓, seen running during this attachment and now
ready), with the detach hint on the right. The agent gets one row less.

A config field turns the bar on or off for every attachment, and a key
toggles it while attached. On tmux the same field and key show and hide
tmux's status line, which holds the detach hint.

## Context
- While attached, the TUI's Bubble Tea loop is blocked in `tea.Exec`
  (bubbletea v2.0.8 `tea.go`: `case execMsg: // NB: this blocks.`), so it
  cannot poll statuses. The server draws the attached screen
  (`internal/vterm/attach.go`, `screen.draw`) and owns every terminal, so it
  samples the other terminals itself: a screen fingerprint changed since the
  last sample means running, unchanged means ready, as `applyStatus` in
  `internal/tui/app.go` decides.
- Terminal names (`vineyard-<project>-<id>`) are not readable, so the
  attaching Vineyard sends each peer's terminal name, title, and its last
  known status (`AttachRequest`). That status shows until the server's first
  comparison, a second later.
- The bar is at the bottom so that mouse coordinates the user's terminal sends
  still match the program's rows; a top bar would need every mouse report
  rewritten.
- Changing `AttachRequest` changes the protocol (`protocol` in
  `internal/vterm/daemon.go`).
- Keys: the toggle key is matched in the raw input like the detach keys, so
  it has their constraint: ctrl and a letter, or ctrl+\. Claude Code binds
  most ctrl letters (b c d e f g j l n o p r s t u v x y); ctrl+\ is free in
  Claude Code and in raw mode reaches Vineyard as byte 0x1c. Terminals that
  follow the VT220 convention also send it for ctrl+4.

## Decisions
- Config field `status_bar` (default true), shown on the config screen as
  "Status bar while attached".
- Key `global.status_bar`, default `ctrl+\`; an empty list unbinds it.
- The toggle lasts for one attachment; each attachment starts from the config.
- Peers are the repository's active sessions in the attached session's
  multiplexer, except the attached one; attaching to a session's shell lists
  its agent too.

## Acceptance Criteria
- [x] Attaching on Vineyard's server draws the agent one row shorter with a bar on the last row listing the other live sessions' titles and statuses, and the detach hint.
- [x] A session that runs during the attachment and then sits still shows as finished (✓).
- [x] The toggle key hides and shows the bar while attached; hidden, the agent takes the full height. The key never reaches the program.
- [x] `status_bar = false` starts every attachment without the bar; the config screen toggles it.
- [x] On tmux, `status_bar = false` hides the status line and the toggle key shows and hides it.
- [x] A toggle key other than ctrl and a letter or ctrl+\ is a config error.
- [x] Protocol version bumped; docs (architecture, README key and config tables) updated.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run Vineyard in a real terminal with two Claude sessions, attach to one
while the other works, and screenshot the bar before and after the other
finishes, and with the bar toggled off.

## Pass Criteria
No gofmt output, vet clean, all tests pass in under 10 seconds; the
screenshots show the bar, the ✓ after the other session finishes, and the full
height screen once toggled off.

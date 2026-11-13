## Goal
Make Vineyard's main TUI screen more mouse-friendly by letting the user drag a session vertically to reorder it and drag the pane's left border horizontally to resize the session column, persisting the column width in `.vineyard/config.toml` (`list_width`).

## Description
Previously `internal/tui/app.go` (`handleMouse`) only handled `tea.MouseClickMsg` and `tea.MouseWheelMsg`, ignoring `tea.MouseMotionMsg` and `tea.MouseReleaseMsg`, and `Model.layout()` always computed `listWidth = max(26, min(44, m.width*3/10))`. Moving a session required pressing `m` (`moveKey`), and changing the split width between the session list and right-hand pane was not possible.

Add two drag interactions to `internal/tui/app.go`:
1. **Drag session to reorder**: Pressing the left mouse button on a `list.LineSession` row (neither busy nor `StatusLoading`) and dragging vertically (`MouseMotionMsg` on a different `Y`) enters drag-move mode (`m.moving = id`, `m.dragging = true`), moves the session live using `list.Model.DragDelta` and `Model.moveBy`, and commits the new order on `MouseReleaseMsg` via `Backend.Arrange` (leaving single clicks and double clicks unchanged when there is no vertical drag, and allowing `esc`/`ctrl+q` to cancel and restore `moveStart`).
2. **Drag divider to resize session column**: Pressing the left mouse button on the pane's left border (`X == listWidth` when not zoomed) and dragging horizontally resizes `Config.ListWidth` live (clamped to `[config.MinListWidth, width - minPaneWidth]`, i.e. `[16, width - 24]`), highlights the pane's left border in `ColorAccent` while held, re-renders layout/issue/recap/transcript views immediately and resizes agent/shell terminals on capture, and saves `Config` to `.vineyard/config.toml` quietly on `MouseReleaseMsg` via `Backend.SaveConfig` (unless `ConfigErr != nil`, so a broken config file is never overwritten). Also expose `Session list width` (`auto` when `0`, or integer `>= 16`) on the Config screen (`internal/tui/settings`).

## Context
- `internal/config/config.go`: `Config` struct, `MinListWidth = 16` (planned), `ListWidth int` (`toml:"list_width,omitempty"`, planned), `Load` validation.
- `internal/tui/settings/settings.go`: `fieldID` (`idListWidth`, planned), `fields`, `value`, `setValue`, `updateEditing` validation.
- `internal/tui/list/list.go`: `Offset` (planned), `DragDelta` (planned), `View` narrow-width truncation (`fit`, planned).
- `internal/tui/app.go`: `minPaneWidth = 24` (planned), `layout`, `resizeLayout` (planned), `setZoom`, `Update`, `handleKey`, `finishMove` (planned), `dragSession` (planned), `dragResize` (planned), `finishResize` (planned), `cancelResize` (planned), `handleMouse`, `renderPane`.
- `internal/config/config_test.go`, `internal/tui/settings/settings_test.go`, `internal/tui/list/list_test.go`, `internal/tui/app_test.go`, and `*.golden` files.
- `README.md`, `docs/architecture.md`, `docs/README.md`.

## Acceptance Criteria
- [ ] Dragging a session row vertically moves the session live in the list (stopping at ends and before `StatusLoading` sessions) and persists the order via `Backend.Arrange` on mouse release.
- [ ] Pressing and releasing without vertical drag behaves as a normal click/double-click; `esc` or `ctrl+q` mid-drag restores `moveStart` without saving; busy/starting sessions and subagent/count rows do not start a drag; keyboard `m` still ignores mouse events.
- [ ] Dragging the pane's left border (`X == listWidth`) resizes the session column live within `[config.MinListWidth, width - minPaneWidth]` (`[16, width - 24]`), highlights the left border while held, reflows pane views and resizes agent/shell terminals on capture, and persists `list_width` via `Backend.SaveConfig` on mouse release without overwriting a config that had `ConfigErr != nil`.
- [ ] `list_width` (`0` = auto, or `>= 16`) round-trips through `config.Load` / `config.Save` (rejecting invalid values `< 0` or `1..15`) and is editable as `Session list width` on the Config screen.
- [ ] Unit tests, interaction tests, and updated golden files cover config, settings, list, and root TUI behavior.
- [ ] `README.md`, `docs/architecture.md`, and `docs/README.md` document session drag, column resize, and `list_width`.

## Verify
Run from `/var/home/mboss/dev/vineyard/.vineyard/worktrees/resize-1e7697`:
```bash
gofmt -l .
go vet ./...
time CGO_ENABLED=0 go test -count=1 ./...
```
Plus exercise a development build in a real tmux terminal with `scripts/drive` and SGR mouse drag sequences, verifying `.vineyard/sessions.json` and `.vineyard/config.toml` (`list_width`) persist across restarts and capturing before/after PNGs.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` exits 0; `CGO_ENABLED=0 go test -count=1 ./...` passes all packages in under 10 seconds; manual `scripts/drive` check shows dragged session order and resized column width saved and applied.

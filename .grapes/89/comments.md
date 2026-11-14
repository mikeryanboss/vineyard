### 2026-11-14T19:29
[STARTED] Upgrading `charm.land/bubbletea/v2` (`v2.1.1`) and `github.com/charmbracelet/ultraviolet` (`v0.0.0-20261008173134-6b8d4baf91b4`), restoring Unicode Mode 2027 (`?2027h`) and clearing the screen on `attachDoneMsg` / `diffToolDoneMsg`, calling `resizeLayout()` on `WindowSizeMsg`, and adding regression tests in `internal/tui/app_test.go`.


### 2026-11-14T19:51
[VERIFY] `gofmt -l .` clean; `go vet ./...` clean; `go test -count=1 ./...` passed in 6.99s (`TestApp_RestoreScreenCmd`, `TestApp_DetachRestoresUnicodeCoreAndRedrawsAfterResize`, and `TestIssues_WindowResizeRefreshesIssueTab` also passed under `-race`); verified real binary via `scripts/drive` across attach -> `resize-window 96x28` -> `ctrl+q` detach and inspected rendered screenshot.

### 2026-11-14T19:52
[DONE] Upgraded `bubbletea/v2` (`v2.1.1`) and `ultraviolet` (`v0.0.0-20261008173134-6b8d4baf91b4`), restored DEC Mode 2027 (`?2027h`) before `tea.ClearScreen` on detach/diff-tool return, switched `WindowSizeMsg` to `resizeLayout()`, and added unit + PTY regression tests.

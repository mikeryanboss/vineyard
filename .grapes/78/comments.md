### 2026-11-11T23:39
[FINDINGS] Inspected `agy` 1.5.6 in a PTY and `internal/vterm`:
- `agy` enters `?1049h` during its startup sign-in splash and modals such as `/changelog` (where it sets `?1002h` and `?1006h`), then exits to the primary screen (`?1049l`) and resets `?1002l`/`?1003l`/`?1006l` for its main chat UI, writing conversation history into primary-screen scrollback.
- `internal/vterm/attach.go` enters the alternate screen on the user's outer terminal (`EnterAltScreen`), syncs mouse modes only when the inner program enables them, and draws only `t.emu.CellAt(x, y)` for `0..height-1`, never `t.emu.ScrollbackCellAt`.
- In `syncModes`, `SetMode` precedes `ResetMode`; resetting `?1002` after setting `?1000` disables mouse tracking in xterm-compatible terminals when a program leaves a modal (`?1002l`) back to primary screen.

### 2026-11-11T23:40
[STARTED] Implementing mouse-wheel scrollback in `internal/vterm/attach.go`, alternate-screen scrollback guard in `internal/vterm/vterm.go`, protocol bump to 6 in `internal/vterm/daemon.go`, tests, and architecture docs.

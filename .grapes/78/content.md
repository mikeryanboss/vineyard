## Goal
Let the user scroll up and down through scrollback with the mouse wheel while
attached to a session on Vineyard's server, so harnesses like `agy` and shells
that write to primary-screen scrollback can be scrolled when attached.

## Description
When attached to a terminal on Vineyard's server (`internal/vterm`), the server
puts the user's terminal on the alternate screen so it can draw the emulator's
cells and Vineyard's status bar and restore Vineyard cleanly on detach. That
hides the outer terminal's own scrollback. Previously `attach` only synced
mouse modes when the inner program enabled them and only drew `t.emu.CellAt`
for the visible rows (`0..height-1`).

Claude Code's fullscreen TUI enters the alternate screen (`?1049h`) and enables
mouse reporting (`?1000h`/`?1006h`) to scroll its own viewport, so wheel events
reached it. `agy` (Antigravity CLI) leaves the alternate screen (`?1049l`) for
its main chat UI with mouse reporting off and lets output scroll into the
emulator's 10,000-line scrollback (`t.emu.Scrollback()`). With mouse reporting
off on the user's alternate-screen terminal, the wheel either did nothing or
sent Up/Down arrow keys (`alternateScroll`), cycling prompt history instead of
scrolling.

Enable mouse reporting (`ModeMouseNormal` and `ModeMouseExtSgr`, upgraded when
the program asks for button-motion or any-event tracking) on the user's
terminal throughout an attachment, intercept mouse sequences in `attach`, and
scroll the attached view across `t.emu`'s scrollback on wheel up/down whenever
the emulator is on the primary screen (`!t.emu.IsAltScreen()`).

## Context
- `internal/vterm/attach.go`:
  - `attach` reads raw bytes from `in`, strips `detach` and `toggle` bytes, and
    writes the rest to `t.emu.InputPipe()`.
  - `screen.syncModes` currently mirrors `inputModes` from `t.modes` and writes
    `ansi.SetMode(set...)` before `ansi.ResetMode(reset...)`. In xterm-compatible
    terminals, `?1000`, `?1002`, and `?1003` share one tracking state, so
    resetting `?1002` after setting `?1000` turns tracking off; resets must
    precede sets.
  - `screen.draw` copies `t.emu.CellAt(x, y)` into `s.buf` and positions the
    cursor at `t.emu.CursorPosition()`.
- `internal/vterm/vterm.go`:
  - `Server.Start` configures `t.emu.SetScrollbackSize(historyLimit)` (10,000
    lines). `t.emu.ScrollbackLen()`, `t.emu.ScrollbackCellAt(x, y)`, and
    `t.emu.IsAltScreen()` expose the primary screen's scrollback and active
    screen state under `t.mu`.
  - `Server.CaptureHistory` reads `t.emu.Scrollback()` without checking
    `!t.emu.IsAltScreen()`; in `charmbracelet/x/vt` v0.1.0 `Scrollback()`
    returns `scrs[0]`'s scrollback even while on the alternate screen.
- `internal/vterm/daemon.go`:
  - `protocol` is currently `5` (`sameCallsSince = 1`). Any change to attaching
    increments `protocol` so new sessions start on a server with the new
    `attach` while older servers' sessions remain reachable through
    `NewOlderClient` and `AttachThroughServer`.

Changes (planned):
- `internal/vterm/attach.go`:
  - Always keep mouse tracking (`ModeMouseNormal` unless `t.modes` asks for
    `ModeMouseButtonEvent` or `ModeMouseAnyEvent`) and `ModeMouseExtSgr` set on
    the user's terminal while attached; emit `ResetMode` before `SetMode` in
    `syncModes`, and reset all set modes on detach.
  - Parse SGR (`ESC [ < b ; x ; y M/m`) and legacy X10 (`ESC [ M cb cx cy`)
    mouse sequences out of `typed` input, holding an incomplete mouse sequence
    across read boundaries without delaying bare `ESC` or other keys.
  - On wheel up/down (`scrollStep = 3` lines): when `!t.emu.IsAltScreen()`,
    adjust `screen.scroll` within `[0, t.emu.ScrollbackLen()]` and redraw;
    when `t.emu.IsAltScreen()` and the program enabled mouse tracking, forward
    the wheel sequence to `t.emu.InputPipe()`.
  - For non-wheel mouse sequences: forward to `t.emu.InputPipe()` only when
    `scroll == 0`, the program enabled mouse tracking, and the row is within
    `t.emu.Height()` (not on the status bar); drop them otherwise so clicks do
    not write escape text into shells or `agy`.
  - While `scroll > 0`, anchor the viewed scrollback line when new lines enter
    scrollback, hide the cursor when scrolled off the bottom (or while viewing
    history), return `scroll` to `0` if the program enters the alternate screen
    or clears scrollback, and jump back to `scroll = 0` when the user types
    input to the program.
- `internal/vterm/vterm.go`: guard `Server.CaptureHistory` scrollback inclusion
  with `!t.emu.IsAltScreen()`.
- `internal/vterm/daemon.go`: bump `protocol` from `5` to `6` (`sameCallsSince`
  stays `1`).
- `docs/architecture.md`: document attached scrollback and protocol `6`.

## Acceptance Criteria
- [ ] Turning the mouse wheel up while attached to a primary-screen terminal on
      Vineyard's server scrolls into `t.emu`'s scrollback, and turning it down
      returns to the live screen, even when the program did not enable mouse
      mode (such as `agy` or `bash`).
- [ ] Clicking in an attached terminal whose program did not enable mouse mode
      writes nothing to the program.
- [ ] New lines scrolling off the live screen while the user is scrolled up keep
      the viewed scrollback lines in place, and typing input jumps the view back
      to the live screen and delivers the input.
- [ ] On the alternate screen (`?1049h`), wheel and click events still reach a
      program that enabled mouse tracking, and switching back to the primary
      screen (`?1049l`) leaves mouse wheel scrollback working.
- [ ] `Server.CaptureHistory` omits primary-screen scrollback while the
      terminal is on the alternate screen.
- [ ] `protocol` in `internal/vterm/daemon.go` is `6` with `sameCallsSince = 1`,
      and `docs/architecture.md` reflects the behaviour.

## Verify
Run from `/var/home/mboss/dev/vineyard/.vineyard/worktrees/attached-scroll`:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual verification with `scripts/drive` on a throwaway repository using a
development build: start a session on Vineyard's server whose output exceeds
the terminal height (and an `agy` session), attach with Enter, send SGR wheel-up
and wheel-down events, verify the screen scrolls into earlier lines and returns
to the live bottom, and capture before/after screenshots.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under
10 seconds; attached wheel-up shows scrollback lines above the live screen and
wheel-down returns to the live bottom.

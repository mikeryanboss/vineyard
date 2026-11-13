## Goal
Let the user scroll fullscreen mouse-aware terminals—such as Claude Code—directly
in Vineyard's preview and terminal tabs using the mouse wheel or pane scroll keys.

## Description
Programs that draw on the primary screen, such as `agy` and shells, let output
roll into the multiplexer's scrollback buffer, which `preview.Model` displays in
a local viewport via `CaptureHistory`. Programs that take over rendering on the
alternate screen (`?1049h`) with mouse tracking enabled (`?1000h`–`?1003h`,
`?1006h`), such as Claude Code 2.1.323 (`alternate_on=1`, `mouse_any_flag=1`,
`mouse_sgr_flag=1`, `history_size=0`), keep off-screen lines in their own memory
and manage scrolling themselves. `CaptureHistory` returns only the visible rows,
so scrolling up in the preview leaves it unmoved and prints `"No scrollback…"`.

Forward mouse-wheel notches (`Wheel`) to the session's or shell's terminal
whenever it is on the alternate screen with mouse tracking enabled, keeping the
preview live and refreshing its screen so the program scrolls its own viewport
in both directions. Primary-screen terminals keep using local scrollback via
`CaptureHistory`, and alternate-screen terminals without mouse tracking keep
showing `"No scrollback…"` on scroll-up.

## Context
- `internal/vterm/vterm.go`, `internal/vterm/attach.go`:
  - `terminal.modes` tracks enabled ANSI modes; `t.emu.IsAltScreen()` reports
    alternate-screen state and `t.hasMouse()` reports mouse tracking.
  - `screen.handleMouse` in `attach.go` encodes SGR (`ansi.MouseSgr`) and legacy
    X10 (`ansi.MouseX10`) mouse events for `t.emu.InputPipe()`.
- `internal/vterm/daemon.go`, `internal/vterm/client.go`:
  - `protocol` is currently `6` (`sameCallsSince = 1`). Adding `Terminals.Wheel`
    raises `protocol` to `7`. Keeping `sameCallsSince = 1` and treating a
    missing `Terminals.Wheel` method on older servers (`NewOlderClient`) as
    unhandled (`false, nil`), as `Client.Info` treats missing `Terminals.List`,
    preserves live protocol 1–6 sessions across an upgrade.
- `internal/tmux/tmux.go`:
  - `display-message -p -t =<name>: '#{alternate_on} #{mouse_any_flag} #{mouse_sgr_flag} #{pane_width} #{pane_height}'`
    reports whether the pane is on the alternate screen with mouse tracking, and
    `send-keys -H -t =<name>:` writes raw mouse-wheel bytes without UTF-8
    decoding.
- `internal/tui/backend.go`, `vineyards.go`:
  - `Terminal` and `Backend` mediate terminal calls (`Wheel` and `WheelShell`,
    planned). `olderVineyard` and `unavailable` implement `Terminal`.
- `internal/tui/common/messages.go`, `internal/tui/preview/preview.go`,
  `internal/tui/app.go`:
  - `preview.Model` emits `common.ScrollbackRequestMsg` when `!m.scrolling` only
    on wheel-up and `Up`/`HalfUp`/`Top`, ignoring wheel-down and
    `Down`/`HalfDown`/`Bottom`. A live fullscreen terminal needs both scroll-up
    and scroll-down requests, with cell coordinates `(X, Y)` and notch count `N`
    (`N > 0` up, `N < 0` down).

Changes (planned):
- `internal/vterm/attach.go`, `internal/vterm/vterm.go`: extract
  `(*terminal).encodeMouse` from `screen.handleMouse`; add
  `(*Server).Wheel(name string, x, y, n int) (bool, error)` that sends `n` wheel
  notches on the alternate screen when `t.hasMouse()` is true and waits up to
  one frame (`frame`) on `t.watch()` for the redrawn screen.
- `internal/vterm/daemon.go`, `internal/vterm/client.go`, `vineyards.go`: add
  `WheelArgs` and `(*Service).Wheel`, bump `protocol` to `7` (`sameCallsSince`
  stays `1`), add `(*Client).Wheel` (returning `false, nil` when an older server
  lacks `Terminals.Wheel`), and add `(olderVineyard).Wheel`.
- `internal/tmux/tmux.go`: add `(*Client).Wheel` querying `alternate_on`,
  `mouse_any_flag`, `mouse_sgr_flag`, `pane_width`, and `pane_height`, and
  sending hex-encoded SGR or X10 wheel sequences via `send-keys -H`.
- `internal/tui/backend.go`: add `Wheel` to `Terminal`, `LiveBackend`, and
  `unavailable`, and `Wheel` / `WheelShell` to `Backend` and `LiveBackend`.
- `internal/tui/common/messages.go`, `internal/tui/preview/preview.go`,
  `internal/tui/app.go`: carry `X`, `Y`, and `N` on `ScrollbackRequestMsg`; emit
  it on wheel up/down and `Up`/`Down`/`HalfUp`/`HalfDown`/`Top`/`Bottom` while
  `!m.scrolling`; in `app.go`, try `Wheel`/`WheelShell` first, refresh the live
  screen when handled, fall back to `CaptureHistory` only when `N > 0`, and
  update the no-scrollback status text.
- `README.md`, `docs/architecture.md`: update preview scroll and protocol 7 docs.

## Acceptance Criteria
- [ ] Turning the mouse wheel up or down over the preview (or terminal tab) of
      an alternate-screen terminal with mouse tracking enabled forwards wheel
      events at the pointer's cell and updates the live screen in both
      directions, on both `vineyard` and `tmux` multiplexers.
- [ ] Pressing `k`/`j`, `ctrl+u`/`ctrl+d`, or `g`/`G` in a focused live preview
      of an alternate-screen mouse-enabled terminal scrolls up and down without
      typing characters into the program.
- [ ] Primary-screen terminals (`!IsAltScreen()`, `alternate_on=0`) never
      receive wheel sequences from the preview: wheel-up enters local scrollback
      via `CaptureHistory`, and wheel-down while live at the bottom does nothing
      without showing `"No scrollback…"`.
- [ ] Alternate-screen terminals without mouse tracking keep the preview live
      and show a `"No scrollback…"` status message on scroll-up.
- [ ] `protocol` in `internal/vterm/daemon.go` is `7` with `sameCallsSince = 1`,
      and `(*vterm.Client).Wheel` against an older server lacking
      `Terminals.Wheel` returns `(false, nil)` without breaking other calls.
- [ ] `README.md` and `docs/architecture.md` reflect the new preview scroll
      behaviour and protocol `7`.

## Verify
Run from `/workspaces/vineyard/.vineyard/worktrees/scroll-96e22b`:
```bash
gofmt -l . && go vet ./... && go test -count=1 ./...
```
Manual verification with `scripts/drive` on a throwaway repository using a
development build: start a real `claude` session (or fullscreen mouse-enabled
session) in Vineyard, scroll the preview up and down with SGR mouse wheel events
and focused-pane keys, confirm the preview scrolls into earlier lines and back
to the bottom while staying live, and capture before/after screenshots.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` and `go test -count=1 ./...` pass in
under 10 seconds; preview wheel-up and wheel-down scroll a fullscreen Claude
Code session up and back to the live bottom without attaching.

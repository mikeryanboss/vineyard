### 2026-11-11T23:39
[FINDINGS] Inspected `agy` 1.5.6 in a PTY and `internal/vterm`:
- `agy` enters `?1049h` during its startup sign-in splash and modals such as `/changelog` (where it sets `?1002h` and `?1006h`), then exits to the primary screen (`?1049l`) and resets `?1002l`/`?1003l`/`?1006l` for its main chat UI, writing conversation history into primary-screen scrollback.
- `internal/vterm/attach.go` enters the alternate screen on the user's outer terminal (`EnterAltScreen`), syncs mouse modes only when the inner program enables them, and draws only `t.emu.CellAt(x, y)` for `0..height-1`, never `t.emu.ScrollbackCellAt`.
- In `syncModes`, `SetMode` precedes `ResetMode`; resetting `?1002` after setting `?1000` disables mouse tracking in xterm-compatible terminals when a program leaves a modal (`?1002l`) back to primary screen.

### 2026-11-11T23:40
[STARTED] Implementing mouse-wheel scrollback in `internal/vterm/attach.go`, alternate-screen scrollback guard in `internal/vterm/vterm.go`, protocol bump to 6 in `internal/vterm/daemon.go`, tests, and architecture docs.

### 2026-11-12T00:04
[DECISION] Parse SGR and legacy X10 mouse reports directly out of the raw `typed` stream in `internal/vterm/attach.go` instead of decoding all input through an event decoder, so bare `ESC`, kitty keyboard sequences, and pastes pass straight through with zero timeout delay. Always keep `ModeMouseNormal` (or the program's motion mode) and `ModeMouseExtSgr` enabled on the user's terminal, writing `ResetMode` before `SetMode` so leaving a modal (`?1002l`) restores `?1000h` instead of turning tracking off.

### 2026-11-12T00:04
[VERIFY] PASS. From `/var/home/mboss/dev/vineyard/.vineyard/worktrees/attached-scroll`: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.3 s; `go test -race -count=1 ./internal/vterm/...` passed in 2.9 s. Manual run with `scripts/drive` on `.grapes/78/tmp/demo` (`vineyard-drive-dev-attached-scroll-p6`, protocol 6): attached to a session with 50 lines of primary-screen output (`[turn 01]..[turn 50]`), sent 5 SGR wheel-up ticks (`\x1b[<64;20;10M`) and confirmed the screen scrolled 15 lines up to `[turn 05]..[turn 38]` above the pinned status bar, then sent 5 wheel-down ticks (`\x1b[<65;20;10M`) and confirmed it returned to `[turn 50]` and the prompt; `ctrl+q` detached cleanly. Screenshots: `.grapes/78/tmp/attached-live.png`, `.grapes/78/tmp/attached-scrolled.png` (published on `pr-evidence` at `853da14b55bbda08cc158646afbbd060bba1ae6d`).

### 2026-11-12T00:04
[DONE] `internal/vterm/attach.go`, `internal/vterm/vterm.go`, `internal/vterm/daemon.go`, `internal/vterm/vterm_test.go`, and `docs/architecture.md` updated for mouse-wheel scrollback in attached server terminals (protocol 6).

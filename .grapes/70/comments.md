### 2026-10-13T20:30
[FINDINGS] Reproduced on a development server with `scripts/drive`: after Claude Code answered "hi", its prompt box read `❯  Greeting reply`, and the emulator's own capture held the same text. A `script` log of Claude Code's output showed `ESC]0;✳ Greeting reply BEL` written with the cursor in the prompt box. Fed alone to `vt.NewEmulator`, that sequence prints the title; with an ASCII title it does not. Cause: `0x9C` in `✳` (E2 9C B3) ends the OSC in `charmbracelet/x/ansi`'s transition table.

### 2026-10-13T20:30
[DECISION] Override `parser.Table`'s `0x9C` entries for the OSC and DCS string states at init in `internal/vterm`, instead of forking the module: upstream PRs #946 and #976 are open, not merged, and the latest release (v0.11.9) still has the entry. The table is shared by every parser in the process, which also corrects `ansi.StringWidth` and `ansi.Truncate` on such strings.

### 2026-10-13T20:32
[VERIFY] PASS. `gofmt -l .` prints nothing; `go vet ./...` passes; `go test ./...` passes in 8 s. `TestTitleStaysOffTheScreen` failed before the change (`the title is on the screen: " Greeting replyready…"`) and passes after it. Claude Code's recorded output, replayed into an emulator with the fix, leaves the prompt box as `❯`. In the real app (`scripts/drive`, Claude Code v2.1.296 attached on a development server, prompted "reply with just the word hi"), the prompt box read `❯  Greeting reply` with the build before and `❯` with the fixed build. Screenshots: `.grapes/70/tmp/before.png`, `after.png`, published on `pr-evidence`.

### 2026-10-13T20:32
[DONE] `internal/vterm/vterm.go`: `init` keeps `0x9C` inside OSC and DCS strings in `parser.Table`. `internal/vterm/vterm_test.go`: regression test. `docs/architecture.md`: the workaround, under Vineyard's Server, Terminals.

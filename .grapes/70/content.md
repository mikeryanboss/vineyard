## Goal
Stop Claude Code's terminal title from being printed on the screen of a
session that runs on Vineyard's server, so its prompt box shows only what the
user typed.

## Description
The user reported text "hovering" in Claude Code's prompt box after Claude
answers: text that is not typed and cannot be edited. It is the session's
title. When Claude Code goes idle it sets the terminal title to
`✳ <title>` with `ESC ] 0 ; ✳ <title> BEL`. `✳` is U+2733, UTF-8
`E2 9C B3`, and the parser of `charmbracelet/x/ansi` takes its `0x9C` byte
for the 8-bit String Terminator. The OSC ends after `E2`, and the emulator
prints ` <title>` at the cursor, which Claude Code keeps in its prompt box.
Claude Code never redraws those cells, so the text stays until something
overwrites them.

The emulator's screen holds the text, so it shows in the preview and in the
attached terminal alike. tmux sessions are unaffected.

## Context
- `internal/vterm/vterm.go`: `Server.Start` feeds each program's output to a
  `charmbracelet/x/vt` emulator (`terminal.mirror`).
- `charmbracelet/x/ansi` v0.11.7 to v0.11.9, `parser/transition_table.go`:
  `table.AddOne(0x9C, OscStringState, DispatchAction, GroundState)`, and the
  same for `DcsStringState`. Every parser reads the package variable
  `parser.Table`. Open upstream fixes: charmbracelet/x#946, #976, #1008.
- On a UTF-8 terminal, programs end OSC and DCS strings with BEL or `ESC \`;
  a lone `0x9C` there is a UTF-8 continuation byte.
- Reproduced: `printf '\033[3;3H\033]0;\342\234\263 Greeting reply\007'` into
  `vt.NewEmulator` prints `Greeting reply` on row 3; the ASCII title
  `Greeting reply` prints nothing.

Change (planned): in `internal/vterm`, keep `0x9C` inside OSC and DCS strings
by overriding those two entries of `parser.Table` at init.

## Acceptance Criteria
- [x] A program that sets a title containing `✳` leaves the screen of a
      Vineyard server terminal without the title's text.
- [x] A regression test in `internal/vterm` fails without the change.
- [x] Claude Code attached on a development server shows an empty prompt box
      after it answers.
- [x] `docs/architecture.md` records the workaround.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
go test ./internal/vterm -run TestTitleStaysOffTheScreen
```
Then attach to a Claude Code session with `scripts/drive` (see Debugging in
`docs/development.md`), prompt it, and read the prompt box once it answers.

## Pass Criteria
All commands pass; the regression test fails with the init removed; the
prompt box reads `❯` with nothing after it once Claude Code answers.

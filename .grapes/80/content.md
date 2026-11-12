## Goal
A program that stops reading its input must not freeze its terminal for
viewers and other Vineyards, as a viewer that stopped reading did (#79).

## Description
Found while checking #79. The emulator (`github.com/charmbracelet/x/vt`
v0.1.0) writes its replies to terminal queries (cursor position, device
attributes, mode reports, colours) synchronously into an unbuffered
`io.Pipe` (`e.pw`), from inside `Emulator.Write`. `forward` in
`internal/vterm/vterm.go` copies that pipe to the pseudo-terminal. When the
program stops reading its input in raw mode, the kernel's input buffer fills,
`forward` blocks writing to the pty, and the next reply blocks inside
`t.emu.Write` in `mirror`, which holds `t.mu`. `Capture`, `CaptureHistory`,
`resize`, `SendKeys`, `Paste`, and attachments then wait behind it. `SendKeys`
and `Paste` also write to the pipe while holding `t.mu`.

The comment on `terminal.mu` says holding `mu` while the pipe blocks is safe
because `forward` never takes `mu`; that holds only while the program reads.

Reproduced on the #79 branch with a throwaway test: a program running
`stty raw -echo; while :; do printf '\033[6n'; done` (cursor-position queries,
never reading their replies); after 2 s, `Capture` did not return within 1 s.

## Open question
Unbuffering the pipe means holding input the program has not read. Decide
the bound and what happens past it (drop input, or end the terminal) before
implementing.

## Acceptance Criteria
- [ ] `Capture` returns promptly while the program ignores its input and
      keeps querying the terminal.
- [ ] Memory held for unread input is bounded.
- [ ] The comment on `terminal.mu` states the rule that holds.

## Verify
Run from the worktree root:
```bash
go test ./internal/vterm/ -count=1
```

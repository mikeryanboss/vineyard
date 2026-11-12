## Goal
An attached viewer whose terminal stops reading must not freeze its session
for everyone else.

## Description
Seen on v0.1.41, protocol 6: a Vineyard window ran in a VS Code terminal
(`/dev/pts/0`) whose tab no longer existed. VS Code kept the terminal alive
but stopped reading it. The server's attach loop blocked in `write(2)` to
pts/0, through the file descriptors the window passed over `SCM_RIGHTS`, while
holding the terminal's `mu`. From then on the agent's screen stopped
updating, `Capture` hung (so every other window's preview of the session
hung), and new attaches to the session hung. Killing the dead terminal's shell
made the write fail and everything recovered.

## Context
- `attach` in `internal/vterm/attach.go` calls `scr.open` and `scr.draw` while
  holding `t.mu`; both write to the viewer's terminal through
  `renderer.Flush()` and `io.WriteString(s.out, ...)`.
- `mirror`, `Capture`, `CaptureHistory`, `resize`, `SendKeys`, `Paste`, the
  status bar's `sample`, and other attachments all take `t.mu`
  (`internal/vterm/vterm.go`, `bar.go`).
- The attach loop is stuck inside the draw, so it never sees `stop` either.

## Plan
1. Render each frame into a buffer while holding `t.mu`; write it after
   releasing the lock.
2. Each attachment writes its frames in its own writer goroutine.
3. Render the next frame only once the writer has finished the last one;
   while it is busy, keep `dirty` set. The renderer sends only changes since
   the frame it last drew, so frames are never dropped, reordered, or queued.
4. If one write has not finished within a stall timeout (5 s), end the
   attachment with the error "terminal stopped reading", without writing the
   exit sequences. The stuck writer goroutine leaks until the terminal hangs
   up; it holds no lock.
5. Never set `O_NONBLOCK` on the viewer's file descriptors: the open file is
   shared with the viewer's process, so the flag would carry over to its shell.

Also check: `mirror` holds `t.mu` during `t.emu.Write`. If the emulator writes
its replies synchronously into its input pipe and the program stops reading
its input, the same freeze can come from the agent's side.

## Acceptance Criteria
- [x] No write to a viewer's terminal happens while `t.mu` is held.
- [x] A viewer whose terminal stops reading does not delay `Capture` of the
      same terminal.
- [x] Such an attachment ends with "terminal stopped reading" shortly after
      the stall timeout, without waiting for the stuck write.
- [x] Detaching normally still resets modes and leaves the alternate screen.
- [x] The agent-side hypothesis is checked and the result recorded.

## Verify
Run from the worktree root:
```bash
go test ./internal/vterm/ -run TestAttachStalledViewer -count=1 -v
gofmt -l . && go vet ./... && go test ./...
```

## Pass Criteria
The stalled-viewer test passes: `Capture` returns within its deadline while
the viewer is stalled, and `attach` returns the stall error. The full suite
passes in under 10 seconds.

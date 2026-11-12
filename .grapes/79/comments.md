### 2026-11-12T12:50
[STARTED] Render frames into a buffer under t.mu and write them from a per-attachment writer goroutine with a stall timeout.

### 2026-11-12T13:40
[FINDINGS] Agent-side hypothesis confirmed. vt v0.1.0 writes query replies synchronously into an unbuffered io.Pipe from inside Emulator.Write. A program in raw mode that floods `\e[6n` and never reads its input fills the pty's input buffer; forward then blocks, the next reply blocks mirror's t.emu.Write under t.mu, and Capture did not return within 1 s (throwaway probe, output in `.grapes/79/tmp/agent-probe.txt`). Filed as #80; out of scope here.

### 2026-11-12T14:00
[VERIFY] `go test ./internal/vterm/ -run TestAttachStalledViewer -count=1 -v`: PASS (1.04 s); `-race -count=3`: PASS. The same test against the old attach.go fails with "Capture waited for a viewer that stopped reading". `gofmt -l .`, `go vet ./...` clean; `go test -count=1 ./...` PASS, 8.6 s wall including builds.
[VERIFY] Real app, through the server and SCM_RIGHTS: dev build in scripts/drive, session flooding base64; attached in the drive window, then SIGSTOPped the drive window's tmux server so nothing read the viewer's pty. A second Vineyard on another tmux server kept a live preview for 8 s, attached and detached normally. The server's only remaining pts fd was the viewer's output, with one thread in write(2) on it. After SIGCONT, the first Vineyard showed "attach: terminal stopped reading" and the server closed the fd. PASS.

### 2026-11-12T14:01
[DONE] internal/vterm/attach.go: screen renders into a buffer; a writer goroutine per attachment writes frames one at a time with stallTimeout (5 s); stalled attachments return errStalled without exit sequences. internal/vterm/vterm_test.go: TestAttachStalledViewer. docs/architecture.md: Attach paragraph.

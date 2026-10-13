### 2026-10-13T08:45
[DECISION] The built-in terminal is the fallback when tmux is not on PATH, not a configuration choice (user's direction). Operations a multiplexer is needed for fail with an error saying so. zellij support is left for a later issue.

### 2026-10-13T11:00
[DECISION] Attaching redraws the emulator on the user's terminal (ultraviolet's diffing renderer) instead of passing the program's output through. Passthrough would leave the user's terminal in whatever alternate-screen and keyboard modes the agent set before the attach, and both terminals would answer the agent's queries. Only input modes (cursor keys, mouse, focus, bracketed paste) are mirrored onto the user's terminal, and reset on detach.

### 2026-10-13T11:05
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test ./...` passes (7.8s wall). `internal/vterm` tests cover capture with colours, scrollback, bracketed paste and keys, program exit, kill of the process group, and attach until ctrl-q over a real pty pair. PASS.
Manual, in a 110x30 tmux window with a PATH lacking tmux (`.grapes/48/tmp/path`): startup shows the notice; a bash session previews, attaches at 110x30 with colours, and returns on ctrl-q; `t` opens a shell; preview scrollback shows lines 160-183 of `seq 1 200`; a second Vineyard exits with "another Vineyard is running on this repository; running several at once needs tmux"; Claude Code renders attached and in the preview, takes a prompt from N (answers "pineapple"); quitting leaves no process in the worktrees; after restart the session shows stopped and `r` resumes the conversation. With tmux on PATH, attach and ctrl-q still go through tmux. PASS.

### 2026-10-13T11:06
[DONE] New `internal/vterm` (Server, attach); `tui.Terminal` interface over `*tmux.Client` and `*vterm.Server`; `Backend.AttachCommand` returns a `tea.ExecCommand`; `main.go` falls back to vterm, claims the lead first, and shows a startup notice. Docs: README, docs/README.md, architecture.md (Without tmux), development.md. Known limit: resizing does not reflow lines, unlike tmux.

### 2026-10-13T11:40
[DECISION] User confirmed tmux stays the default and the built-in terminal is only the fallback when tmux is missing (already the behaviour). Stated what needs a multiplexer: the startup notice now names both features, and README's Install section lists them.

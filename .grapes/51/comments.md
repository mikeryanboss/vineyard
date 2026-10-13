### 2026-10-13T12:45
[DECISION] The tab mirrors the existing `t` shell (`Session.ShellName()`) rather than a new terminal, so `t`, enter on the tab, kill, and check out all act on one shell. Showing the tab starts the shell; a shell that exits is started again on the next tick. The terminal tab polls the shell on the preview tick in place of the agent capture.

### 2026-10-13T12:46
[VERIFY] `gofmt -l .` (empty), `go vet ./...`, `go test ./...`: PASS, tui package 5.6s. New test `TestTerminalTab_MirrorsTheSessionsShell` covers order, mirroring, enter attaching to the shell, and the paused placeholder. Manual run in tmux (`-p bash`, throwaway repo): tab ×3 shows Terminal with the shell prompt and starts `<name>-shell`; enter attaches, a command's output stays visible after ctrl-q; `l` then `k` scrolls the shell's scrollback and `G` returns to live. PASS. Screenshot: `.grapes/51/tmp/terminal-tab.png`.

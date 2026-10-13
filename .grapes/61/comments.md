### 2026-10-13T13:45
[STARTED] Status bar drawn by Vineyard's server during attach, sampling the other terminals itself, since the TUI's loop is blocked in tea.Exec. Config field and toggle key, applied to tmux's status line too.

### 2026-10-13T14:40
[DECISION] Bar at the bottom: mouse reports from the user's terminal then still match the program's rows; a top bar would need every report rewritten. Default toggle key ctrl+\: Claude Code binds most ctrl letters (b c d e f g j l n o p r s t u v x y); attached-terminal keys now accept ctrl and a letter or ctrl+\. Bar colours come from the 256-colour palette (bg 238), since themes remap the 16 basic colours: with bright black, freeze rendered a light grey bar with an unreadable hint. The toggle lasts one attachment.

### 2026-10-13T14:41
[VERIFY] gofmt -l . (no output), go vet ./... clean, go test ./... PASS (about 7 s wall; tui package 6.5 s). Real run in tmux 100x26 with `-p bash` sessions on Vineyard's server: bar lists "◑ parser fix ◑ docs pass", then "✓ docs pass" once its loop ended; ctrl+\ gives stty size 26 100, again 25 100. Session in tmux: ctrl+\ hides and shows tmux's status line. Screenshots: .grapes/61/tmp/attached-{running,finished,hidden}.png. PASS

### 2026-10-13T14:42
[DONE] Server-side bar in internal/vterm/bar.go and attach.go; AttachRequest carries keys, ShowBar, and peers (protocol 3). session.Attachment/Peer passed from tui (attachment in app.go) through Terminal.Attach to tmux and vterm. Config status_bar, key global.status_bar, config screen row, README and docs.

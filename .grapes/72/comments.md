### 2026-10-14T12:10
[DECISION] `R` changes only `Session.Title`. ID, branch, worktree directory, and terminal name keep the names derived at creation: git and the multiplexer own them, and the agent may have switched branches. `R` goes through `ready`, because a lifecycle operation in flight saves the session as it started and would undo the rename. An empty title keeps the dialog open.

### 2026-10-14T12:30
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` PASS (tui 6.3 s). New tests `TestApp_RenameIsSaved`, `TestApp_RenameRefusesEmptyTitlesAndBusySessions`; letting busy sessions through `ready` fails the second. Manual run of a dev build in tmux (own socket `vy72-drive`, not `scripts/drive`, whose fixed names collided with another agent's run): `R`, ctrl+u, typed "Fix the login loop", enter; list and status message show the new title, `sessions.json` has it, branch still `mboss/amber-riesling`; after restart the list still shows it; at 200 columns the status bar shows `R rename`. PASS. Screenshots: .grapes/72/tmp/dialog.png, renamed.png.

### 2026-10-14T12:35
[DONE] Added list key `rename` (`R`), `dialog.Rename`, `common.RenameMsg`, its handling in `internal/tui/app.go`, the status bar hint, tests, and README and architecture lines.

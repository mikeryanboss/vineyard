### 2026-11-14T18:53
[DECISION] User confirmed design choices:
1. Session list issue numbers use a three-way setting: `after` (default), `before`, `off`.
2. When `after`, issue numbers clip off the right first so the title gets width priority (`Fix login redirect #12` -> `Fix login redirect…`).
3. Expose six settings under `[list]` in `.vineyard/config.toml` and in the `C` config screen: issue numbers (`issues`), issue picker order (`issue_picker`), subagents (`subagents`), branch line (`branch`), diff line counts (`diff_stats`), and auto-yes tag (`auto_tag`).

### 2026-11-14T18:54

### 2026-11-14T19:13
[STARTED] Implementing `[list]` configuration in `internal/config`, session list options and rendering in `internal/tui/list`, polling/picker updates in `internal/tui`, and the `Session list` category in `internal/tui/settings` on top of `origin/main` (`v0.1.49`).

### 2026-11-14T19:14
[VERIFY] `gofmt -l .`, `go vet ./...`, and `go test -count=1 ./...` passed in 6.83s (`PASS`). Exercised the TUI in tmux via `scripts/drive` and verified rendered screenshots in `.grapes/88/tmp/{before,after,config-session-list,compact}.png` (`PASS`).

### 2026-11-14T19:14
[DONE] Added `[list]` configuration (`issues`, `issue_picker`, `subagents`, `branch`, `diff_stats`, `auto_tag`) in `internal/config/config.go`, updated session list rendering and clipping in `internal/tui/list/list.go`, wired live application, subagent polling bypass (`subagent.ConversationID`), and issue picker ordering in `internal/tui/app.go` and `internal/tui/backend.go`, added the `Session list` category in `internal/tui/settings/settings.go`, and documented `[list]` in `README.md`.

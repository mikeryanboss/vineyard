### 2026-11-15T10:37
[STARTED] Extending `internal/recap` for Claude Code final responses and Antigravity CLI (`agy`) conversations, and building `internal/tui/recapview` with default 4-line previews, click/keyboard expansion, and entry navigation.

### 2026-11-15T10:59
[VERIFY] Ran `gofmt -l .` (clean), `go vet ./...` (exit 0), and `CGO_ENABLED=0 go test -count=1 ./...` (all packages passed in 6.88s, including `internal/recap` in 0.009s, `internal/tui/recapview` in 0.006s, and `internal/tui` in 6.06s). Exercised the built TUI in tmux via `scripts/drive` against `claude` and `agy` sessions and captured before/after screenshots with `freeze`.

### 2026-11-15T10:59
[DONE] Added Claude Code final assistant responses and Antigravity CLI (`agy`) transcript/history/title loading in `internal/recap`, added `internal/tui/recapview` with 4-line collapsible previews, mouse click and keyboard navigation/folding, wired `LiveBackend.Recap` and `app.go`, and updated documentation.

### 2026-11-15T11:47
[VERIFY] Added `[recap]` configuration (`prompts`, `responses`, `recaps`: `preview` | `collapsed` | `expanded` | `off`), Config screen `Recap` category, clickable per-kind and expand/collapse-all header chips in `recapview`, 1-line header collapse state with inline snippets, and `fold_prompts`/`fold_responses`/`fold_recaps` (`1`/`2`/`3`) pane keys. Verified `gofmt -l .` (clean), `go vet ./...` (exit 0), `CGO_ENABLED=0 go test -count=1 ./...` (all packages passed in 6.87s), and real-terminal tmux screenshots via `scripts/drive` and `freeze`.

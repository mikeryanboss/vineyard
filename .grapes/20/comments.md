### 2026-10-09T11:45
[DECISION] Reuse the issue tab's scrollable pane for recaps by renaming `internal/tui/issueview` to `internal/tui/textview`; it held no issue-specific logic, and a copy would duplicate it. Read every transcript in the worktree's directory, so recaps survive `/clear`. A malformed matching line is an error shown in the tab, not skipped.

### 2026-10-09T12:05
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` clean; `go test -count=1 ./...` passes in 5.0 s. PASS.
`recap.Load` on five real transcripts under `~/.claude/projects` (one over 5 MB) returned their titles, last prompts, and recaps in 0.2 s. PASS.
Real TUI in a throwaway repo, two sessions whose program is a script named `claude`, `CLAUDE_CONFIG_DIR` pointed at copies of real transcripts: the Recap tab shows a session's two recaps newest first under its title, and a session without recaps shows "No recap yet" and its last prompt. Screenshots in the PR. PASS.

### 2026-10-09T12:10
[DONE] Added `internal/recap/` (transcript reader), `Backend.Recap`, the Recap tab in `internal/tui/app.go`, and tests in `recap_test.go` and `app_test.go`; renamed `issueview` to `textview`; documented the tab in `docs/` and `README.md`, and the `freeze` colour-reset workaround in `docs/development.md`.

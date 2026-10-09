### 2026-10-09T09:00
[DECISION] Grapes renders the issue (new `embedded.Model.RenderIssue`, Mibokess/grapes#53) instead of vineyard re-implementing grapes' detail layout. Several issues are stacked rather than paged, which keeps the tab's keys the same as the diff's. No new `i` binding inside the focused pane; `i` from the list already opens the issue in grapes.

### 2026-10-09T09:40
[VERIFY] `gofmt -l .` lists nothing; `go vet ./...` and `go test ./...` pass (suite 4.4 s). Removing either `refreshIssue` call (in `updateGrapes` or `refreshShown`) fails the matching test. Real terminal (tmux 120×34, `-p bash`, demo repo with issue #9): the tab shows #9; editing `.grapes/9/meta.toml` and `comments.md` in the session's worktree switched the tab to that worktree's copy (`in_progress`, worktree pill active) within one grapes reload. Golden files changed only in the tab bar line. PASS

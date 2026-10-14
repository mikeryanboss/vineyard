### 2026-10-14T12:10
[DECISION] Filter in `shownIssues` and `syncList`, not `issuesOf`: `showSessions` and `promptData` must still match a session to each sub-issue it touched. Only the direct parent counts; grapes loads parent cycles, so walking up could loop. Clip to three numbers, else two and `+N`, so the row keeps room for the title.

### 2026-10-14T12:10
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test ./...` PASS (7.8s). New tests `TestIssues_SubIssuesHideUnderTheirParent` and `TestIssues_ManyIssuesShowTwoAndACount` fail with the filter disabled. Ran the dev build in a demo repo (`scripts/drive`): parent+3 subs shows `#1`; sub-issue alone shows `#3`; parent+subs+6 independent shows `#1 #10 +5` in the row and `Issues #1 #10 +5` in the tab. PASS; screenshots in the PR.

### 2026-10-14T12:10
[DONE] `internal/tui/app.go` (`mainIssues`, `shownIssues`, `syncList`, `tabLabels`), `internal/tui/list/list.go`, `internal/tui/common/theme.go` (`ClipIssues`), tests in `internal/tui/app_grapes_test.go`, docs in `docs/architecture.md`.

### 2026-10-13T11:09
[FINDINGS] Reproduced with vineyard v0.1.17 in a throwaway repo: Enter on the issue tab of session `linked` (diff touches `.grapes/1/`) opens #1; after `q`, Enter on session `unlinked` opens #1's detail again. `TestIssues_EnterOnTheIssueTabOpensGrapes` asserted that Enter opens grapes for a session without an issue. Enter on Diff and Recap shows the selected session's own state ("No changes yet", "Recaps come from Claude Code; this session runs bash."), so they have no stale-target bug.

### 2026-10-13T11:12
[DECISION] Enter on the issue tab returns without opening grapes when there is no session or `shownIssues` is empty; the tab's placeholder already says what to do. `i` keeps `showIssues` unchanged, so it still opens grapes as it was, which after an opened issue is that issue's detail. Fixing that needs a grapes `embedded` API to leave the detail screens; out of scope here.

### 2026-10-13T11:13
[VERIFY] `gofmt -l .` printed nothing; `go vet ./...` and `go test ./...` passed (`internal/tui` 5.7s). The new `TestIssues_EnterOnTheIssueTabOpensOnlyTheSessionsIssue` failed before the fix (grapes showed #8 for the issue-less session, and grapes opened with no session) and passes after. In tmux, the dev build kept the sessions screen and the "No issue linked" placeholder for `unlinked`. Screenshots `.grapes/54/tmp/before-enter.png`, `after-enter.png`, published to `pr-evidence`. PASS.

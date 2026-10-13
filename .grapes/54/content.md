## Goal
Enter on the issue tab opens only an issue the tab shows. A session without an
issue must not open grapes at whatever issue grapes showed last.

## Description
Enter on the issue tab runs `showIssues`, as `i` does. When the selected
session (or its shown subagent) has no issue, or no session is selected,
`showIssues` sets `issuesOpen` and grapes opens "as it was". Grapes keeps its
screen when the user quits it, so after any issue's detail was open, Enter on
an issue-less session shows that issue's detail, as if it were the session's.

Enter will do nothing on an issue tab that shows no issue; the tab's
placeholder already says what to do. `i` keeps opening grapes as it was. A
menu for linking an issue to the session is a later issue.

## Context
- `internal/tui/app.go`: `handleKey` routes `keys.Issues` and `keys.Open` on
  `tabIssue` to `showIssues`; `showIssues`, case 0, sets `issuesOpen` only.
- `shownIssues` gives the issues the tab shows; `refreshIssue` renders the
  placeholders "No issue linked. Press i to browse issues." and "Press n to
  start a session.".
- Grapes `embedded.Model` (v0.1.14) has `OpenIssue` but no way to leave a
  detail screen, so vineyard cannot reset grapes to its board.
- `internal/tui/app_grapes_test.go`: `TestIssues_EnterOnTheIssueTabOpensGrapes`
  asserts the buggy behavior for a session without an issue.
- Enter on the diff, recap, terminal, and preview tabs acts on the selected
  session's current state (`diffCmd`, `recapShown` keyed by session ID,
  `EnsureShell`, `AttachCommand`), so they have no stale-target bug.

## Acceptance Criteria
- [x] Enter on the issue tab of a session with an issue opens that issue's detail.
- [x] Enter on the issue tab of a session without an issue leaves grapes closed, even after another issue's detail was open.
- [x] Enter on the issue tab with no session leaves grapes closed.
- [x] `i` behaves as before.
- [x] The README's key table says Enter opens the session's issue only when it has one.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then, in a real terminal, run Vineyard with one session on an issue and one
without: open the first's issue with Enter, quit grapes with q, select the
second, and press Enter on the issue tab.

## Pass Criteria
`gofmt` prints nothing, `go vet` and `go test` pass, and in the terminal the
second session's Enter leaves the sessions screen showing its "No issue
linked" placeholder.

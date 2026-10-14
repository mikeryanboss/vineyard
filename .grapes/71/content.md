## Goal
Keep a session's issue numbers short in its list title and the pane's issue
tab label, even when the session created twenty issues.

## Description
A planner agent that writes every issue for a large run touches each issue's
`.grapes/<id>/`, so `issuesOf` links all of them to its session. The list title
and the tab label then print every number, pushing the title off the row.

Two changes:

1. **Main issues only.** Where vineyard shows a session's issues, it drops each
   issue whose parent is also among them: the parent's rendering lists its
   sub-issues. A session or subagent that works on a sub-issue without its
   parent still shows the sub-issue.
2. **At most three numbers.** The list title and the tab label show up to three
   issue numbers; with more, they show the first two and `+N` for the rest.

## Context
- `issuesOf` in `internal/tui/app.go`: the full set, recorded issue plus issues
  the diff touches. `showSessions` and `promptData` match sub-issues against it,
  so it stays unfiltered.
- `shownIssues` in `internal/tui/app.go` feeds the tab label (`tabLabels`), the
  issue tab (`refreshIssue`), and the issue picker (`showIssues`). It returns
  the filtered set (planned).
- `syncList` passes `issuesOf` to `list.Item.Issues`; it passes the filtered set
  (planned). `renderItem` in `internal/tui/list/list.go` prints them.
- Parents come from `embedded.Model.Issue(id).Parent`. Only the direct parent is
  checked: grapes does not reject parent cycles on load, and walking further up
  could loop.

## Acceptance Criteria
- [x] A session linked to a parent and its sub-issues shows only the parent in
  its list title, tab label, issue tab, and issue picker.
- [x] A session linked to a sub-issue but not its parent shows the sub-issue.
- [x] `showSessions` still finds a session for each of its sub-issues.
- [x] With more than three main issues, the list title and tab label show two
  numbers and `+N`.
- [x] `gofmt -l .`, `go vet ./...`, and `go test ./...` pass.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run vineyard with a session linked to many issues and look at its list row
and tab bar.

## Pass Criteria
Commands print nothing but passing package lines; the screenshot shows at most
three issue numbers on the row and tab.

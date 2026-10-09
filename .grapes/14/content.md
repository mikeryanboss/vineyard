## Goal

Show the grapes issue a session works on as a tab of the pane, next to Preview and Diff, so the user can read it without leaving the sessions screen. `i` keeps opening the full grapes screen.

## Description

Add a third pane tab, Issue. It shows the selected session's issues, the same set `i` uses (`issuesOf`: the recorded `Issue` plus the issues its branch changed), as grapes' detail screen renders them, at the pane's width, scrollable like the diff. With several issues, they are stacked, separated by a rule. `tab` cycles Preview, Diff, Issue.

Grapes' `embedded` package could only draw its whole screen. Mibokess/grapes#53 adds `embedded.Model.RenderIssue(id, worktreePath, width)`, which renders one issue and shows the session worktree's copy when that worktree changed it.

## Context

- `internal/tui/app.go`: `tab` constants, `toggleTab` (planned rename: `switchTab`), `renderPane`, `renderStatusBar`, `handleKey`/`handleMouse` pane routing, `refreshShown`, `updateGrapes`, `issuesOf`.
- `internal/tui/preview/`, `internal/tui/diffview/`: existing pane content models. Planned: `internal/tui/issueview/`.
- Rendering markdown on every frame would be slow; vineyard polls the selected screen every 150 ms. Render only while the tab is shown, when the selection, tab, or grapes state changes. Resizes and theme changes reach grapes through `updateGrapes`, so re-rendering there covers them.
- Dependency: grapes branch `53/embedded-render-issue`. `go.mod` pins its commit; re-pin to grapes `main` once that PR merges.

## Acceptance Criteria

- [x] `tab` cycles Preview → Diff → Issue → Preview; the tab bar labels the tab with the session's issue IDs.
- [x] The issue tab shows the selected session's issues and follows the selection.
- [x] A session without issues, or a repository without grapes, gets a placeholder that says why.
- [x] A grapes reload re-renders the tab.
- [x] The focused issue pane scrolls with j/k, ctrl+d/u, g/G, and the mouse wheel; `esc` returns to the list.
- [x] README, `docs/README.md`, `docs/architecture.md`, and `docs/development.md` describe the tab.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Then run the development build in tmux on a demo repository with an issue, start a session from the issue (`i`, `a`, enter, enter), press `tab` twice, and edit the issue in the session's worktree.

## Pass Criteria

`gofmt` lists nothing; vet and tests pass, including `TestIssueTab_ShowsTheSelectedSessionsIssue` and `TestIssueTab_FollowsGrapesReloads`. In the terminal, the issue tab shows the issue, and after the edit it shows the worktree's copy without user action.

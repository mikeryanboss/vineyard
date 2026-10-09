## Goal

Move between an issue and the agent sessions working on it in one keypress, in both directions, and start a session from an issue.

## Description

With grapes shown inside vineyard (#9), the two screens still know nothing of each other. This issue links them:

- **Grapes to sessions.** Grapes' `a` key (Mibokess/grapes#52) emits `embedded.SessionsMsg{IssueID}` for the selected issue. Vineyard finds the sessions using that issue. None: it opens the new-session dialog filled in for the issue. One: it closes grapes and selects that session. Several: a picker lists them, and the chosen one is selected.
- **Session to grapes.** `i` on a session opens grapes at that session's issue. Several issues: a picker chooses one. None: grapes opens on its board, as in #9.
- **Starting from an issue.** The new-session dialog is pre-filled with the issue's title and a prompt naming the issue. The session records the issue, and its branch is `<id>/<slug of title>`, following this repository's branch convention, instead of the configured prefix.
- **The list** shows `#<id>` before the title of each session linked to an issue.

## Context

Verified against `main` at `33f9d60`:

- `session.Session` (`internal/session/session.go`) has no issue field; sessions persist as JSON in `sessions.json`.
- `Manager.New` (`internal/session/manager.go:56`) names the branch `BranchPrefix + Slug(title, id)`.
- `dialog.NewSessionDialog` (`internal/tui/dialog/dialog.go:60`) has a title and an optional prompt field and emits `common.NewSessionMsg{Title, Prompt, Profile}`.
- `app.go:createSession` builds `session.NewOptions` from that message.
- The list renders `list.Item{Session, Stat}` (`internal/tui/list/list.go`).

A session uses an issue when either holds:

1. **Recorded:** `Session.Issue` (new, JSON `issue`, omitted when zero) names it. It is set when the session starts from an issue and survives pause, which removes the worktree.
2. **Touched:** `embedded.Model.TouchedIssues(session.WorktreePath)` lists it, meaning the session's branch changed the issue's files relative to where it branched. This covers sessions started by hand.

The picker is a new dialog in `internal/tui/dialog/` listing labelled choices and emitting the chosen one; both directions use it.

## Acceptance Criteria

- [x] `a` on an issue with no sessions opens the new-session dialog with the issue's title and a prompt naming `.grapes/<id>/`; submitting starts a session on branch `<id>/<slug>` with `Issue` recorded.
- [x] `a` on an issue with one session closes grapes and selects that session.
- [x] `a` on an issue with several sessions shows a picker; choosing one closes grapes and selects it; `esc` returns to grapes.
- [x] A session counts as using an issue when it recorded it or its worktree touched it.
- [x] `i` on a session linked to one issue opens grapes on that issue's detail; with several, a picker chooses; with none, grapes opens as before.
- [x] The session list shows `#<id>` for linked sessions.
- [x] `Session.Issue` round-trips through `sessions.json`, and old files without it still load.

## Verify

Run from the repository root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in a real terminal: in this repository, press `i`, select an issue, press `a`, submit the dialog; the new session appears with `#<id>` on branch `<id>/...`. Press `i` on it: grapes opens on that issue. Press `a`: vineyard returns to the session. Start a second session for the same issue and press `a` again: the picker lists both.

## Pass Criteria

Formatting, vet, and tests pass, and the tests cover each acceptance criterion above. The manual run behaves as described; screenshots of the picker and of grapes opened at a session's issue go in the PR.

## Goal

Selecting a subagent row shows that subagent in every tab, as selecting a session shows the session: its transcript, its diff, its issue, and what it was asked and reported.

## Description

Today a subagent row shows its session's panes. The user asked for "everything like the diff and issue but of the subagent itself", with the transcript as its preview (decided 2026-10-11: a subagent has no terminal of its own, and switching Claude Code's view on selection would type into the session).

- **Preview:** the subagent's transcript rendered as a conversation: its prompt, messages, tool calls with a one-line summary, and the first lines of tool results; newest at the bottom, following new lines unless the user scrolled up.
- **Diff:** the diff of the worktree the subagent works in. That is the git worktree holding the file it last edited (Edit, Write, NotebookEdit), else the directory of its last `cd <absolute path>` in Bash, else its transcript's `cwd`. When that is the session's worktree, the session's diff.
- **Issue:** the grapes issues of that worktree's diff (`.grapes/<id>/`), as for sessions, when the subagent has its own worktree. In the session's worktree, the issues whose files the subagent edited.
- **Recap:** its description, type, status, the prompt it was given, and its final report (the `SubagentHandback` message, else its last text).

## Context

- `internal/tui/app.go`: `shownID`, `refreshShown`, `captureCmd`, `diffCmd`/`diffMsg`, `refreshIssue`, `issuesOf`, `recapCmd`/`refreshRecap`, `renderPane`.
- `internal/tui/textview`: the scrollable text pane the issue and recap tabs use.
- `internal/git/diff.go`: `Base`, `Diff`.
- Data from #39 (`subagent.Agent`).

## Acceptance Criteria

- [x] With a subagent row selected, Preview shows its rendered transcript and follows new lines.
- [x] Diff shows its own worktree's diff when it works in another worktree, else the session's.
- [x] Issue shows its issues as described, or says it has none.
- [x] Recap shows its description, status, prompt, and report.
- [x] Moving between the session row and its subagent rows switches every tab.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual: a demo session whose subagent creates its own worktree with a grapes issue and edits a file there. Selecting its row shows its transcript, that worktree's diff, and that issue; selecting the session shows the session's again.

## Pass Criteria

gofmt prints nothing; vet and tests pass in under 10 s; the manual run shows each tab for the subagent.

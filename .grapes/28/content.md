## Goal
A session keeps showing the issues it changed after its pull request merges,
and its diff shows only its own work, even after the agent rebases onto main.

## Description
`issuesOf` in `internal/tui/app.go` linked a session to its recorded `Issue`
plus grapes' `embedded.Model.TouchedIssues(WorktreePath)`. Grapes computes
those as the issues changed in `merge-base(origin/main, HEAD)..HEAD`, plus
uncommitted ones. Once the branch merges and `origin/main` is fetched, the
merge-base is HEAD itself, so grapes reports nothing. A session not started
from an issue (`Issue == 0`) then shows no issues at all.

Observed 2026-10-09: session `sessions-34eca0` on `27/multiple-vineyards`
completed #27; PR #19 merged into `origin/main`; vineyard showed no issues
for the session.

The diff tab had a related bug: it diffs from `BaseCommit`, the commit the
session started from, so a rebased branch's diff includes everything it took
in from main. The #27 session's diff showed 29 files; its own commits change 16.
Linking issues from that diff would have shown #25 #26 #27.

Fix, decided with the user (vineyard side, plain git, diff tab included):
`git.Base(dir, start)` finds where the branch's own work begins.

- No `origin/HEAD`: `start` (`BaseCommit`), today's behaviour.
- Not merged: `git merge-base origin/HEAD HEAD`.
- Merged: the oldest commit of
  `git rev-list --first-parent --ancestry-path HEAD..origin/HEAD` is the merge
  commit; the base is `git merge-base <merge>^1 HEAD`.
- HEAD at the mainline's tip, or a fast-forward merge: HEAD.

The diff, line counts, diff tool `{base}`, and issue links all use it. Issue
links come from `.grapes/<id>/` paths in the diff (`git.Stat.Paths`).

## Context
- `internal/git/diff.go`: `Base`, `Stat.Paths`, `DiffStat` (`--numstat -z`).
- `internal/tui/backend.go`: `LiveBackend.Diff`, `DiffStat`, `DiffTool`.
- `internal/tui/app.go`: `issuesOf`, the `diffMsg` case, `updateGrapes`.
- `docs/architecture.md` (Grapes, Diffs), `README.md` (`diff_command`).

## Acceptance Criteria
- [x] `git.Stat` carries the paths its diff changes, both sides of a rename,
      and untracked files.
- [x] `git.Base` returns the branch's own fork point before and after a
      rebase and a merge commit, HEAD at the mainline's tip, and `start`
      without `origin/HEAD` (test).
- [x] Diff, line counts, and `{base}` use `git.Base`.
- [x] `issuesOf` returns the recorded issue plus issues under `.grapes/<id>/`
      in the session's diff paths; it no longer uses `TouchedIssues`.
- [x] The issue tab refreshes when the selected session's issues change (test).
- [x] `docs/architecture.md` and `README.md` describe the new base.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then, in a throwaway clone with a remote: start a session, finish issue #1 on
it, merge another branch into main, rebase the session, merge it with a merge
commit, push, fetch. Compare the old and new builds.

## Pass Criteria
No gofmt output, vet clean, all tests pass. The new build lists `#1` and only
the session's own lines; the old build lists no issue and includes the other
branch's lines.

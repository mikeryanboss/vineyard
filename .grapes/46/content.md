## Goal
A subagent working in its own worktree, on a branch created from its
session's branch, shows only its own work in the diff and issue tabs.

## Description
A subagent's diff runs from where its worktree's branch forks from the
mainline (`git.Base`). When the subagent's branch was created from the
session's branch, that diff also holds every commit the session made before
the subagent started. The issue tab reads its issues off that diff, so each
subagent shows the session's issues as its own.

Seen on 2026-10-12 in the saily session "automode" (branch `44/autopilot`).
Its first commit, d734748 "#44: Open the autopilot issues and its
interface", created issues #44 to #49. Five subagents then worked in
worktrees on branches `45/starts` to `49/autopilot-segel`, each created from
`44/autopilot` at or after d734748. Every one of them forks from `origin/HEAD`
at 42c0514, so every one's diff holds d734748 and lists #44 to #49. Diffed
from where they fork from `44/autopilot`, `46/autopilot-anlegen-boje` lists
only #46 and `48/autopilot-box` only #48.

Branches created from `origin/main` are unaffected, which is why it depends
on how the subagent made its worktree.

The session also merged three of the branches (`45/starts`, `47/...`,
`49/...`) into `44/autopilot` with merge commits. Their own work must survive
that merge, as a session's survives its merge into the mainline (`git.Base`).

## Context
- `internal/git/diff.go`, `Base(dir, start)`: forks from `origin/HEAD`, and
  after a merge from the merge's first parent.
- `internal/tui/backend.go`, `treeBase`, `TreeDiff`, `TreeDiffTool`: the
  subagent's worktree diff, from `git.Base(root, HEAD)`.
- `internal/tui/app.go`, `diffCmd` and the diff tool key: call `TreeDiff` and
  `TreeDiffTool` with the shown subagent's worktree (`agentTree`).
- `internal/tui/app.go`, `shownIssues`: a subagent in a worktree of its own
  has the issues of that worktree's diff.

Change: the subagent's work begins where its branch forks from the session's
branch or from the mainline, whichever is later. Against the session's
branch, the same merge rule as `Base` applies.

## Acceptance Criteria
- [x] A branch created from another branch's tip has, as its own work, only
  its own commits and changes, before and after a merge commit takes it into
  that branch.
- [x] A branch created from the mainline, ahead of where the other branch
  forked, still has only its own work.
- [x] A subagent's diff and issue tabs use this base; the diff tool too.
- [x] `docs/architecture.md` describes the new base.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then check the saily "automode" subagents' issue lists with the new base.

## Pass Criteria
Formatting, vet, and tests pass. In saily, `46/autopilot-anlegen-boje` lists
only #46 and `45/starts` only #45, where each listed #44 to #49 before.

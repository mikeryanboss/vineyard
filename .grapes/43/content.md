## Goal
Agents stop when their PR merges. The human pulls `main` and deletes the
vineyard session; agents no longer clean up after a merge.

## Description
`AGENTS.md` (which `CLAUDE.md` links to) tells agents: "After your PR merges,
fast-forward local `main` and remove the merged branch and its worktree." That
rule predates vineyard, where one session keeps one worktree across many PRs
and the agent runs inside it. The agent cannot remove its own worktree, and it
cannot check out `main` there because the main checkout already has it. So it
checks out `origin/main` with no branch and deletes the merged branch.

That detached HEAD then blocks deleting the session: `Manager.Kill`
(`internal/session/manager.go`) reads the branch with `git.CurrentBranch`
(`internal/git/git.go`), which fails with "has a detached HEAD; check out a
branch first". The "subagents" session hit this on 2026-10-12: its reflog
shows seven `checkout: moving from <id>/... to origin/main` entries, the last
after #42 merged.

The human pulls `main` by hand and deletes the session to clean up, so the rule
is removed and replaced with an explicit stop.

## Context
- `AGENTS.md`, "Pull requests" section, the post-merge bullet.
- `.agents/skills/pr/SKILL.md` already says not to append work to a merged
  branch and to start a new branch from `origin/main`; it needs no change.
- Out of scope: `Kill` refusing a detached HEAD, and `Kill` keeping merged
  branches because they have commits beyond the session's base.

## Acceptance Criteria
- [x] `AGENTS.md` no longer tells agents to fast-forward `main` or remove the
      merged branch or worktree.
- [x] `AGENTS.md` says the agent's job ends when its PR merges.

## Verify
Run from the worktree root:
```bash
grep -n -i "fast-forward\|remove the merged" AGENTS.md CLAUDE.md
grep -n "job ends when the PR merges" AGENTS.md
```

## Pass Criteria
The first command prints nothing. The second prints the new bullet in the
"Pull requests" section.

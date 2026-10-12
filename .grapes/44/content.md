## Goal
Deleting a session works when its worktree has a detached HEAD, unless that
would lose commits.

## Description
`Manager.Kill` reads the checked-out branch before removing the worktree, and
`git.CurrentBranch` fails on a detached HEAD with "has a detached HEAD; check
out a branch first". So a session whose agent left the worktree without a
branch cannot be deleted from Vineyard. The "subagents" session hit this on
2026-10-12: after #42 merged, its agent ran `git checkout origin/main` and
deleted its branch.

`Kill` needs the branch only to decide whether to delete it. A detached HEAD
has no branch to delete, so `Kill` should remove the worktree and delete no
branch. Removing the worktree does lose commits that only the detached HEAD
reaches, and `Kill` promises never to throw away committed work, so it still
refuses when such commits exist.

`Pause` and `Push` need a branch to commit to and push, so they keep refusing a
detached HEAD.

## Context
- `internal/session/manager.go`, `Manager.Kill`: calls `git.CurrentBranch`
  and returns its error.
- `internal/git/git.go`, `CurrentBranch`: returns a plain `fmt.Errorf` on a
  detached HEAD (exit code 1 from `git symbolic-ref --quiet`).
- #19 made `Kill` read the branch from git so it never acts on the branch saved
  in `sessions.json`, which goes stale when the agent switches. With a detached
  HEAD, `Kill` must not fall back to that saved branch either.
- Commits count as kept when a local branch, a remote-tracking branch, or a tag
  contains them. Remote-tracking branches matter: the human pulls `main` by
  hand (#43), so an agent's merged work may be only on `origin/main`.
- Planned: `git.ErrDetachedHead`, wrapped by `CurrentBranch` with the same
  message, and `git.UnreferencedCommits(dir)`, which runs
  `git rev-list --count HEAD --not --branches --remotes --tags`.

## Acceptance Criteria
- [x] `Kill` on a detached HEAD that a branch, remote-tracking branch, or tag
      contains removes the worktree, stops the terminals, and deletes no branch,
      including the branch saved in the session.
- [x] `Kill` on a detached HEAD with commits that nothing else contains fails
      with an error naming the detached HEAD, and leaves the worktree and agent
      in place.
- [x] `Pause` and `Push` still refuse a detached HEAD.
- [x] `docs/architecture.md` describes the new `Kill` behavior.

## Verify
Run from the worktree root:
```bash
go test ./internal/session/ -run 'Kill|DetachedHead' -v
gofmt -l . && go vet ./... && go test ./...
```
In the real app, following `docs/development.md` "Running a Development
Build": start a `bash` session, run `git switch --detach` in its worktree,
kill it with `D`, and check that the session, its worktree, and its tmux
session are gone. Then commit on the detached HEAD of a second session and
check that `D` shows the refusal and leaves it in place.

## Pass Criteria
All tests pass. In the real app the first session disappears and
`git worktree list` no longer shows it; the second stays, with the error in the
status line.

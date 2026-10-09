## Goal
`sessions.json` stops holding facts that git already owns: a session's worktree path and, for a session with a worktree, its branch. Moving or copying the repository then keeps sessions working, and lifecycle operations act on the branch the agent actually works on.

## Description
Two stored copies of git state go wrong:

1. **Absolute `worktree_path`.** `Session.WorktreePath` is saved as an absolute path (`internal/session/session.go`). After the repository directory moves, every operation uses the old path. `Resume` of a paused session recreates the worktree at the old location. Git's own worktree links are absolute too, so a moved worktree is also broken for git (`fatal: not a git repository: <old>/.git/worktrees/<id>`).
2. **Stale `branch`.** Agents create branches inside their worktree (CLAUDE.md: `<id>/short-description`). Observed: session `grapes-62ff23` has `"branch": "mboss/grapes"` in `sessions.json` while `git worktree list` shows its worktree on `18/open-at-issue`. `Push` then pushes the stale branch, `Pause` + `Resume` checks out the stale branch, and `Kill` decides deletion on the stale branch.

Changes:

- `Session.WorktreePath` becomes runtime-only (`json:"-"`). `Manager.Restore` fills it from `git worktree list`, matching the entry whose directory name is the session ID (vineyard names worktree directories by ID). Without a live entry it falls back to `<worktree_dir>/<id>`. When that directory exists but git does not know it (links broken by a move), `Restore` runs `git worktree repair --relative-paths <path>`.
- Worktrees are created with `git worktree add --relative-paths` (git >= 2.48), so links survive moving the repository with worktrees inside it, or moving both together.
- A session with a worktree reads its branch from git: `Restore` takes it from `git worktree list`; `Pause`, `Push`, and `Kill` read the checked-out branch first and fail on a detached HEAD instead of acting on a guess. `Pause` therefore records the branch actually checked out, which is what `Resume` restores. The TUI's 2 s git poll refreshes each session's branch for display.
- `Push` returns the updated session; `KillResult` names the kept branch.

## Context
- `internal/session/session.go` — `Session` fields.
- `internal/session/manager.go` — `New`, `Start`, `Pause`, `Resume`, `Kill`, `Push`, `Restore`.
- `internal/git/git.go` — `AddWorktree`, `AddWorktreeForBranch`, `BranchCheckout` (parses `git worktree list --porcelain`).
- `main.go` `runTUI` — calls `manager.Restore` before the TUI starts; resolves nothing about `worktree_dir` yet.
- `internal/config/config.go` `ResolveWorktreeDir` — `worktree_dir` may be relative to the repository, `~/…`, or absolute, so worktrees can live outside the repository.
- `internal/tui/app.go` `diffCmd` / `diffMsg`, `lifecycleMsg`, `killedMsg`, `pushedMsg`; `internal/tui/backend.go`.
- Verified with git 2.53 in a throwaway repository: with `--relative-paths`, a worktree inside the repository survives renaming the repository; a worktree outside survives moving both together; after moving only the repository, `git worktree repair --relative-paths <worktree>` restores it. `repair` without the flag rewrites the links as absolute.
- Out of scope: adopting a branch pushed from another machine as a session; grapes `NextID` not seeing other machines' IDs.

## Acceptance Criteria
- [x] `sessions.json` contains no `worktree_path`.
- [x] New worktrees' `.git` files hold a relative `gitdir`.
- [x] After the repository directory is renamed, `Restore` finds a session's worktree at its new location, for both relative links and legacy absolute links (repaired).
- [x] A paused session restored after a move resumes inside the moved repository.
- [x] A session whose agent switched branches gets the new branch from `Restore`, and `Pause`, `Push`, and `Kill` act on the checked-out branch.
- [x] `Pause`, `Push`, and `Kill` fail with a clear error on a detached HEAD.
- [x] The session list shows a branch change within one git poll.
- [x] Docs describe the new persistence and the git >= 2.48 requirement.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manually: start vineyard in a scratch repository, create a session, quit, rename the repository directory, start vineyard again, and check that the session's diff and shell work; switch branch inside the session and watch the list update.

## Pass Criteria
`gofmt -l .` prints nothing, `go vet` and `go test` pass in under 10 s, and the manual check shows the session working after the rename with its new branch in the list.

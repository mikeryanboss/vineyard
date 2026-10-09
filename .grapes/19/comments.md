### 2026-10-09T11:50
[FINDINGS] sessions.json (gitignored) stores absolute worktree_path; git worktree links are absolute by default; stored branch goes stale when agents switch branches (grapes-62ff23: stored mboss/grapes, actual 18/open-at-issue). Grapes persists no paths; it discovers worktrees with git worktree list.

### 2026-10-09T11:50
[DECISION] Find worktrees through git by session ID, create them with --relative-paths, repair broken links at startup, and read live branches from git. Agreed with the user.

### 2026-10-09T12:05
[VERIFY] `gofmt -l .` clean, `go vet ./...` clean, `go test -count=1 ./...` PASS in 4.95 s. Mutations (absolute links, no repair, Restore ignoring the branch, Pause keeping the stored branch) each fail a new test. Manual run with a dev build in a throwaway repo (.grapes/19/tmp/vy): created alpha and beta, paused beta, switched alpha to 19/real-work (list followed within one poll), quit, killed the tmux server, renamed demo-a to demo-b, restarted: alpha found at the new path on 19/real-work with its diff, both resumed, beta's worktree created inside demo-b. sessions.json had no worktree_path. PASS. Screenshots: .grapes/19/tmp/before-move.png, after-move-diff.png.

### 2026-10-09T12:05
[DONE] internal/git/git.go (relative worktrees, Worktrees, RepairWorktree, CurrentBranch), internal/session (runtime-only WorktreePath, Restore via git with repair, branch read before Pause/Push/Kill), internal/tui (branch poll, Push/Kill results), main.go (Restore gets worktree_dir), README and docs. Known leftover: after an agent switches branches, the session's original branch stays behind when the session is killed.

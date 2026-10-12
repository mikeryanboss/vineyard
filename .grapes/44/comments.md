### 2026-10-12T21:56
[STARTED] Let `Kill` proceed on a detached HEAD when no commits would be lost; keep `Pause` and `Push` refusing.

### 2026-10-12T22:05
[DECISION] The refusal names the session title instead of the worktree path. With the path, the status line cut the message off before "check out a branch first" at 150 columns.

### 2026-10-12T22:05
[VERIFY] From the worktree root: `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` passes. `go test ./internal/session/ -run 'Kill|DetachedHead' -v` passes all six, including the new `TestKill_RemovesDetachedHeadMergedUpstream` and `TestKill_RefusesDetachedHeadWithUnreferencedCommits`. Dropping `--remotes` from `UnreferencedCommits` makes the merged-upstream test fail. Real app (`-p bash`, throwaway repo): a session detached at `main` with its branch deleted was killed ("Killed merged."; worktree, tmux session, and `sessions.json` entry gone). A session with a commit only on its detached HEAD was refused with `session "adrift" has commits only on its detached HEAD; check out a branch first`, and its worktree and tmux session stayed. Screen captures: `.grapes/44/tmp/kill-detached-*.txt`. PASS.

### 2026-10-12T22:05
[DONE] `internal/git/git.go`: `ErrDetachedHead`, wrapped by `CurrentBranch`, and `UnreferencedCommits`. `internal/session/manager.go`: `Kill` proceeds on a detached HEAD without touching any branch unless HEAD has unreferenced commits. `internal/session/session_test.go`: Kill tests split out of the old detached-HEAD lifecycle test. `docs/architecture.md`: lifecycle bullets.

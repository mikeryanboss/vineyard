### 2026-10-12T21:40
[DECISION] The user pulls `main` by hand and wants agents to stop at a successful merge, so the whole rule goes, fast-forward included, rather than keeping the fast-forward part.

### 2026-10-12T21:44
[VERIFY] From the worktree root: `grep -n -i "fast-forward\|remove the merged" AGENTS.md CLAUDE.md` printed nothing (exit 1); `grep -n "job ends when the PR merges" AGENTS.md` printed line 53 in "Pull requests". `gofmt -l .` empty, `go vet ./...` and `go test ./...` pass. PASS.

### 2026-10-12T21:44
[DONE] Replaced the post-merge cleanup bullet in `AGENTS.md` (`CLAUDE.md` is a symlink to it) with: "Your job ends when the PR merges. Leave `main`, the branch, and the worktree alone: the human pulls `main` and deletes the session."

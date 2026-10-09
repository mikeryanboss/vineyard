### 2026-10-09T15:05
[FINDINGS] Grapes' TouchedIssues compares against merge-base(origin/main, HEAD), which is HEAD once the branch merges. Confirmed on this repository: every merged session branch has merge-base == HEAD. A first fix using BaseCommit over-linked the rebased #27 branch (#25 #26 #27) and matched the diff tab's existing over-inclusion (29 files vs 16).

### 2026-10-09T15:10
[DECISION] With the user: fix in vineyard, using plain git, for issues and the diff tab. git.Base finds the fork point, through the merge commit once merged. Tested by hand on every branch here: 27, 26, 6, 7, 3 each map to exactly their own issue.

### 2026-10-09T15:20
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test ./...` PASS. Mutation check: dropping the merged-branch step fails TestBase_FindsTheBranchsOwnWorkThroughRebaseAndMerge; dropping the issue-tab refresh fails TestIssueTab_FollowsTheDiff. Real terminal, throwaway clone with remote, rebase + merge commit + fetch: new build shows `#1 parser +2 -1` and issue #1 done; old build shows `parser +11 -2` and no issue. PASS. Screenshots in the PR.

### 2026-10-09T15:22
[DONE] internal/git/diff.go (Base, Stat.Paths), internal/tui/backend.go, internal/tui/app.go, tests, docs/architecture.md, README.md, config and session comments.

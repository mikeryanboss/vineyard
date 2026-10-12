### 2026-10-12T22:45
[FINDINGS] In saily's "automode" session, all five subagent branches
(`45/starts` to `49/autopilot-segel`) were created from `44/autopilot` at or
after d734748, which opened issues #44 to #49. `git.Base` measured each from
its fork with `origin/HEAD` (42c0514), so each diff held d734748 and every
subagent listed #44 to #49.

### 2026-10-12T22:50
[DECISION] Added `git.StackedBase(dir, onto)`: the later of the fork with the
session's branch and the fork with the mainline. `baseOn` holds `Base`'s merge
rule with the mainline as a parameter, so a merge commit on the session's
branch takes the subagent's branch in. `TreeDiff` and `TreeDiffTool` take the
session and pass its worktree's HEAD. Without origin/HEAD, the work begins at
the fork with the session's branch.

### 2026-10-12T23:05
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass.
`TestStackedBase_FindsTheWorkOfABranchCreatedFromAnother` fails when
`StackedBase` returns the mainline base (shows the session's
`.grapes/1/meta.toml`) and when it ignores the mainline (shows
`mainline.txt`). On saily's worktrees, against automode's HEAD: Base 42c0514
gives #44 to #49 for each; StackedBase d734748 gives #45, #46, #47, #48, #49
respectively, including the three already merged into `44/autopilot`. In a
throwaway repo with a session and two stacked subagents, one merged back, the
main build's issue tab shows "Issues #2 #3 #4" and "Diff +26 -0" for each
subagent; this branch shows "Issue #3" and "Issue #4", "Diff +2 -1".
Screenshots in the PR. PASS.

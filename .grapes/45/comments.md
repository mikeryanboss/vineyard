### 2026-10-12T22:35
[STARTED] The user asked for a menu that explains which commits a kill would lose and still allows the kill. Approach: a typed refusal from `Kill`, and a second confirmation in the TUI that lists the commits.

### 2026-10-12T22:40
[VERIFY] From the worktree root: `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` passes. New or changed tests: `TestKill_DetachedHeadWithUnreferencedCommits` (refusal lists the commit, then `discardCommits` kills), `TestApp_KillLosingCommitsAsksAgain` (second dialog, five of seven commits plus "… and 2 more", `n` keeps, `y` kills), `TestConfirm_LinesAreCutToFit` (golden; a long subject ends in "…", clicks still answer). Real app (`-p bash`, throwaway repo, 110x30): two commits on a detached HEAD; `D` `y` opened the dialog listing both and the keep hint; `n` kept the session, worktree, and tmux session; `D` `y` `y` printed "Killed parser." and removed the worktree, tmux session, and `sessions.json` entry. Screenshots: `.grapes/45/tmp/dialog.png`, `.grapes/45/tmp/killed.png`. PASS.

### 2026-10-12T22:40
[DONE] `internal/git/git.go`: `UnreferencedCommits` lists commits. `internal/session/manager.go`: `LostCommitsError`, `Kill(s, discardCommits)`. `internal/tui`: `ActionKillDiscardingCommits`, the second dialog in the `killedMsg` handler, `Backend.Kill` signature. `internal/tui/dialog/dialog.go`: `NewConfirm` lists optional lines, cut to fit. `docs/architecture.md`: Kill bullet.

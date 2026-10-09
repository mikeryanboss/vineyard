### 2026-10-09T00:25
[DECISION] Issue sessions take branch `<id>/<slug>` rather than the configured prefix, matching this repository's `<id>/short-description` convention. One session jumps straight to it; only several show a picker; none offers a new session.

### 2026-10-09T10:27
[VERIFY] `go test ./...` passes, including `TestStart_IssueSessionBranchesByIssue`, `TestStore_LoadsSessionsWithoutIssue`, and in `app_grapes_test.go`: no session → pre-filled dialog creating a session with `Issue` 7 and a prompt naming `.grapes/7/`; one session → jump; several → picker, `esc` stays in grapes, choice jumps; `i` on a session → its issue's detail; a session that only touched #8 in its worktree is linked (`issuesOf` = [7 8], picker on `i`, found by `a` on #8). Manual run in tmux on a demo repo: `a` on #1 opened "New session for #1", submitting created `#1 Fix the login redirect` on branch `1/fix-the-login-redirect`; `i` on it opened #1; a second session that only edited `.grapes/1/meta.toml` was tagged `#1`, and `a` on #1 listed both and jumped to the chosen one. PASS.

### 2026-10-09T10:28
[DONE] `Session.Issue`, `<id>/<slug>` branches, `dialog.NewSession.ForIssue`, `dialog.Pick`, `issuesOf`/`showSessions`/`showIssues` in the root model, `#<id>` tags in the list, README and docs.

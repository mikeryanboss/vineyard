## Goal
When killing a session would lose commits, Vineyard shows which commits and
lets the user kill it anyway or cancel, instead of only printing an error.

## Description
Since #44, `Manager.Kill` refuses a worktree whose detached HEAD has commits
that no branch, remote-tracking branch, or tag contains, because removing the
worktree loses them. The TUI shows the refusal in the status line and the user
has no way to proceed from Vineyard.

Now the refusal opens a second confirmation dialog. It says the commits exist
only on the detached HEAD and are lost with the worktree, tells how to keep
them (cancel, open the shell with `t`, `git switch -c <branch>`), lists the
commits, and offers `y` to kill anyway and `n` to cancel.

## Context
- `internal/session/manager.go`, `Manager.Kill`: returns a plain error built
  with `fmt.Errorf` when `git.UnreferencedCommits` is above zero.
- `internal/git/git.go`, `UnreferencedCommits`: returns a count from
  `git rev-list --count HEAD --not --branches --remotes --tags`.
- `internal/tui/app.go`: `D` opens `dialog.NewConfirm` with
  `common.ActionKill`; `runConfirmed` calls `backend.Kill`; the `killedMsg`
  handler shows any error with `setError`.
- `internal/tui/dialog/dialog.go`, `Confirm`: a bold message and the
  `y confirm · n cancel` answers; a click on the last content line answers.
- Planned:
  - `git.UnreferencedCommits` returns the commits as `<short hash> <subject>`,
    newest first.
  - `session.LostCommitsError` carries the session title and those commits;
    `Kill` returns it, keeping today's message as its `Error()`.
  - `Manager.Kill(s, discardCommits bool)`: with `discardCommits`, it skips the
    check and kills.
  - `common.ActionKillDiscardingCommits`: the second dialog's action, which
    calls `Kill` with `discardCommits`.
  - `dialog.NewConfirm` takes optional lines shown under the message, each cut
    to the dialog's width. The TUI lists at most five commits and then
    "… and N more".
- The first dialog stays as it is; the second appears only after `Kill`
  refuses, so the commits listed are the ones `Kill` saw.

## Acceptance Criteria
- [x] `Kill` on a detached HEAD with unreferenced commits returns a
      `*session.LostCommitsError` listing them, and changes nothing.
- [x] `Kill` with `discardCommits` removes such a worktree and stops the agent.
- [x] In the TUI, `D` then `y` on such a session opens a dialog that lists the
      commits and says how to keep them; `n` leaves the session; `y` kills it.
- [x] Commit lines longer than the dialog are cut, not wrapped, and more than
      five commits end with "… and N more".
- [x] `docs/architecture.md` describes the flow.

## Verify
Run from the worktree root:
```bash
go test ./internal/session/ ./internal/tui/... -run 'Kill|Confirm' -v
gofmt -l . && go vet ./... && go test ./...
```
In the real app (`docs/development.md`, "Running a Development Build"), start
a `bash` session, commit twice on a detached HEAD in its worktree, press `D`
and `y`. Capture the dialog as a screenshot, press `n` and check the session
stays, then `D`, `y`, `y` and check that the session, its worktree, and its
tmux session are gone.

## Pass Criteria
All tests pass. The screenshot shows both commits and the keep hint; after
`n` the session is listed; after the second `y` the status line says
"Killed <title>." and `git worktree list` no longer shows it.

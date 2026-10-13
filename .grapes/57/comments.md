### 2026-10-13T11:11
[FINDINGS] Leave keys per view, from `internal/tui/common/keys.go`, `internal/tui/dialog/dialog.go`, `internal/tui/settings/settings.go`, grapes v0.1.14 `internal/tui/common/keys.go`, and a run of the real binary: attached agent/shell leave only on ctrl-q (tmux `DetachKey`, `vterm.detachKey`); issues screen on `q` (grapes `GlobalKeyMap.Quit`, `CloseMsg`), while `esc` stays inside grapes; panes on `esc`/`h`/`←`/`q`; config on `esc` only, one level at a time, `q` does nothing; dialogs on `esc`, plus `q` in Confirm and Pick but not NewSession. ctrl-q did nothing outside tmux.

[DECISION] ctrl-q returns to the session list in one press from any screen, pane, or dialog, matching what it does from tmux, and does nothing on the list rather than quitting, so a repeated press after a detach is harmless. Other keys are unchanged.

### 2026-10-13T11:11
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test -count=1 ./...` passes, 7.7 s wall. `TestApp_CtrlQReturnsToTheList` covers issues, issue search holding `q` as text, a dialog over issues, the new-session dialog, the config field pane, a focused pane, a zoomed recap, and no quit on the list; all seven return cases fail with the handler disabled. Manual: `.grapes/53/tmp/drive.sh` drives a dev build with `-p bash` in tmux; every ctrl-q lands on the session list, `q` in the issue search filters instead of closing, `o`/`t` attach and ctrl-q detach still work, and `q` then quits. Log: `.grapes/53/tmp/drive.log`. PASS.

[DONE] `internal/tui/app.go` handles ctrl+q in `handleKey`; test in `internal/tui/app_grapes_test.go`; `testutil.Key` gains ctrl+q; README key table and `docs/architecture.md` updated.

### 2026-10-13T11:31
[DECISION] Renumbered from #53, which `main` already holds for Vineyard's terminal server (PR #44): both were allocated 53 in separate worktrees. The issue now lives in `.grapes/57/`; the PR branch keeps its name `53/ctrl-q-back`, since GitHub cannot change an open PR's head branch. The `.grapes/53/tmp/` paths above were in the deleted worktree.
After rebasing onto `main` at 6dc4977 (with #54 and #55), `internal/tui/app_grapes_test.go` kept both `TestApp_CtrlQReturnsToTheList` and main's `TestIssues_EnterOnTheIssueTabOpensOnlyTheSessionsIssue`.

[VERIFY] After the rebase: `gofmt -l .` empty; `go vet ./...` clean; `go test -count=1 ./...` passes. PASS.

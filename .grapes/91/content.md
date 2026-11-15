## Goal
Allow users opening the new-session dialog with `N` (`NewFromProfile`) to override whether the session runs in a new worktree or in the main checkout via an option below the title field.

## Description
Currently, pressing `N` opens `dialog.NewSession` with `choose = true`, displaying `Profile` (when more than one profile exists) and `Title`. Whether the created session uses a new worktree or the repository's main checkout is determined solely by `Profile.MainCheckout` from configuration, with no way to flip it per session without editing settings.

Add a `Where` picker field (`‹ new worktree ›` / `‹ main checkout ›`) below `Title` in `dialog.NewSession` when `choose` is true and the session is not for a grapes issue (`issue == 0`):
- Default `mainCheckout` to the currently selected profile's `MainCheckout`, and update it to the newly selected profile's `MainCheckout` whenever the user steps `Profile`.
- Allow toggling `Where` with `PrevOption` / `NextOption` (`←`/`h`, `→`/`l`), `Toggle` (`space`), or mouse clicks on `‹` / `›`.
- Keep `Enter` on `Title` (when `withPrompt` is false) submitting the dialog directly, while `Tab`/`Shift+Tab` navigates between `Profile`, `Title`, and `Where`, and `Enter` on `Where` submits.
- Reflect the effective `MainCheckout` value in the dialog's `profileSummary` line and emit `common.NewSessionMsg` with `Profile.MainCheckout` set accordingly.
- Do not show `Where` in the quick `n` dialog (`choose == false`) or in issue session dialogs (`ForIssue`, where sessions must work on their own branch in a worktree).

## Context
- `internal/tui/dialog/dialog.go`: `field`, `NewSession`, `NewSessionDialog`, `ForIssue`, `fields`, `step`, `submit`, `Update`, `blocks`, `Hints`.
- `internal/tui/dialog/dialog_test.go` and `internal/tui/dialog/testdata/`: unit tests and golden views for `NewSession`.
- `internal/tui/app.go`, `internal/tui/app_test.go`, `internal/tui/testdata/`: `createSession` already reads `msg.Profile.MainCheckout`; update golden view and add end-to-end test coverage.

## Acceptance Criteria
- [x] Pressing `N` (`choose == true`, `issue == 0`) shows a `Where` picker below `Title` initialized to the selected profile's `MainCheckout` (`new worktree` or `main checkout`).
- [x] Stepping `Profile` updates `Where` and the summary line to the new profile's `MainCheckout` default.
- [x] Focusing `Where` (via `Tab`/`Shift+Tab` or click) allows switching between `new worktree` and `main checkout` using `←`/`h`, `→`/`l`, `space`, or clicking `‹`/`›`, updating the summary line and `Hints()` (`←/→ where`).
- [x] Submitting applies the chosen `MainCheckout` value to `common.NewSessionMsg.Profile.MainCheckout`, creating a main-checkout or worktree session accordingly.
- [x] `n` (`choose == false`) and issue sessions (`ForIssue`, `issue > 0`) do not show `Where`.

## Verify
Run from `/var/home/mboss/dev/vineyard/.vineyard/worktrees/dialog-where`:
```bash
gofmt -l .
go vet ./...
go test -count=1 ./...
```
Also capture screenshots of the running TUI with `scripts/drive` showing `N` with `Where` toggled between `new worktree` and `main checkout`.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` exits 0; `go test -count=1 ./...` passes in under 10 seconds; driven TUI shows the `Where` option below `Title` and starts sessions in the chosen checkout mode.

### 2026-11-15T19:42
[STARTED] Add `fieldWhere` (`new worktree` / `main checkout` picker) below `Title` in `internal/tui/dialog/dialog.go` for `choose && issue == 0`, wired to `NewSessionMsg.Profile.MainCheckout`.

### 2026-11-15T19:45
[VERIFY] PASS. From `/var/home/mboss/dev/vineyard/.vineyard/worktrees/dialog-where`: `gofmt -l .` printed nothing; `go vet ./...` exited 0; `go test -count=1 ./...` passed across all packages in ~6.7s. Exercised the built TUI in tmux with `scripts/drive` on `.grapes/91/tmp/demo`: pressed `N`, verified `Where: ‹ new worktree ›`, tabbed past `Title`, toggled `Where` to `‹ main checkout ›`, confirmed summary updated to `Runs bash in the main checkout, on Vineyard's server.`, created the session, verified it ran in `main`, and verified `D` showed the main-checkout kill prompt. Screenshots: `.grapes/91/tmp/dialog-worktree.png`, `.grapes/91/tmp/dialog-main.png`.

### 2026-11-15T19:45
[DONE] Changed `internal/tui/dialog/dialog.go`, `internal/tui/dialog/dialog_test.go`, `internal/tui/dialog/testdata/TestNewSessionView_ChoosingAProfile.golden`, `internal/tui/app_test.go`, and `internal/tui/testdata/TestAppView_NewSessionDialog.golden`.

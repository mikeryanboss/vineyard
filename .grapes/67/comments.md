### 2026-10-13T15:20
[STARTED] Replace the empty-title error in NewSession.submit with a random adjective + grape-variety title.

### 2026-10-13T15:25
[DECISION] Random titles are an adjective plus a grape variety ("sunny viognier"), picked in `randomTitle` in `internal/tui/dialog/dialog.go`. Generated in the dialog, so the title flows into the ID, branch, and worktree slug unchanged. Replaced `TestNewSession_RequiresTitle` and its golden file with `TestNewSession_EmptyTitleGetsARandomOne`, and made `TestNewSession_ChoosingSubmitsTitleAndProfile` assert that enter on the profile produces no `NewSessionMsg`, since its old check looked for the removed error text.

### 2026-10-13T15:27
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass. Drove a dev build with `scripts/drive`: `n`, then Enter on the empty title, listed a session "sunny viognier" on branch `mboss/sunny-viognier` (the user config's branch prefix) in worktree `sunny-viognier-7e879`. PASS.

### 2026-10-13T15:28
[DONE] Changed `internal/tui/dialog/dialog.go`, `dialog_test.go`, and the README key table; removed the `TestNewSession_RequiresTitle` golden file.

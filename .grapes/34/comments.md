### 2026-10-11T16:21
[DECISION] A double click on a subagent row opens the subagent on every tab, as `o` does, since opening it is what the row is for; elsewhere a double click is `enter`. Where the first click already acts (the count line, diff headers and summary lines), the second click of a double is dropped so the action happens once. Double clicks are told on the main screen only: a dialog's first click answers it, and the click that closes it must not pair with one on the screen beneath.

### 2026-10-11T16:21
[STARTED] Double-click detection in `handleMouse`, `list.CountLine`, tests, docs.

### 2026-10-11T16:26
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.2 s. `TestApp_ClosingADialogMakesNoDoubleClick` failed with the reset of `lastClick` on dialog clicks removed. Manual, development build in a 140×36 tmux window on a throwaway repository, two `claude --model haiku` sessions, two background subagents, double clicks sent as two SGR clicks: on `▸ 2 subagents` it expanded once; on the `error-messages` row it opened that subagent's view and attached (`Message @general-purpose…`); on Docs pass it attached; on the Recap tab it zoomed the recap; on the preview it attached; on the diff's `▾ M shop.go` header it folded the file once and opened no diff tool. PASS. Screenshots in the PR.

### 2026-10-11T16:26
[DONE] `internal/tui/app.go`: `doubleClick`, `lastClick`, `now`, `enter`, and the double-click cases in `handleMouse`; `internal/tui/list/list.go`: `LineAt` and `at`; tests; `README.md`, `docs/architecture.md`.

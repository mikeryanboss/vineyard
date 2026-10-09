### 2026-10-09T21:55
[DONE] `go.mod` requires grapes v0.1.14; `version` is 0.1.3. `internal/tui/app_grapes_test.go` imports `internal/config` again.

### 2026-10-09T21:55
[VERIFY] From the worktree root: before the import fix, `go vet ./...` failed with `internal/tui/app_grapes_test.go:49:39: undefined: config`. After it, `gofmt -l .` printed nothing, `go vet ./...` passed, and `go test -count=1 ./...` passed. `.grapes/30/tmp/vineyard version` printed `0.1.3`. Manual: in a scratch repository, `vineyard -p cat -i 2` on a `bug` sub-issue showed `vineyard 0.1.3` in the header, preselected `‹ bug ›`, and the prompt included the bug and parent paragraphs. PASS.

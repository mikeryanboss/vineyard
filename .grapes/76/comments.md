### 2026-10-20T16:40
[DECISION] Only the keys wrap. The mouse wheel still stops at the ends: one flick sends a burst of wheel events, which wrapping would carry past the end and around again. `move` takes a `wrap` flag; `Update` passes true for `Keys.List.Up`/`Down` and false for the wheel.

### 2026-10-20T16:40
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` and `go test ./...` passed (`internal/tui` 6.4 s, the longest). With `move` forced to `wrap=false`, `TestList_NavigationWrapsAround` and `TestList_NavigationWrapsThroughSubagentRows` fail, so they guard the behavior. Manual, with `scripts/drive` on a demo repository with three `-p bash` sessions: `j` on `third` selected `first`, `k` on `first` selected `third`; wheel down on `third` kept `third`, and three wheel-ups from `second` stopped at `first`. PASS. The wrap through subagent rows is covered by the unit test only: the demo sessions have no subagents.

### 2026-10-20T16:40
[DONE] `internal/tui/list/list.go` (`move` wraps when asked), `internal/tui/list/list_test.go` (`TestList_NavigationClamps` replaced by two wrap tests and `TestList_WheelStopsAtTheEnds`), `README.md` (navigation keys in the key table; the wheel stops at the ends).

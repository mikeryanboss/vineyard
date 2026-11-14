### 2026-11-14T14:28
[STARTED] Implementing performance, concurrency, caching, and rendering optimizations on branch `87/speed-up-many-sessions` across `internal/session`, `internal/recap`, `internal/vterm`, `internal/git`, `internal/subagent`, `internal/tui`, `main.go`, `sessions.go`, and `docs/architecture.md`.

### 2026-11-14T17:05
[VERIFY] `gofmt -l .`: empty; `go vet ./...`: exit 0; `go test -race -count=1 ./internal/session ./internal/recap ./internal/subagent ./internal/vterm ./internal/git .`: PASS; `go test -race ./internal/tui/...`: PASS; `time CGO_ENABLED=0 go test -count=1 ./...`: PASS in 7.142s real. Benchmarks: `BenchmarkAgyReader_IdlePoll-16` 1524 ns/op (336 B/op, 3 allocs/op), `BenchmarkReader_IdleLoad-16` 6819 ns/op (892 B/op, 17 allocs/op), `BenchmarkCapture_Idle-16` 13.86 ns/op (0 B/op, 0 allocs/op), `BenchmarkDiff_Untracked-16` 2411478 ns/op (20 untracked files, 0 extra git subprocesses), `BenchmarkSetSize-16` 6795032 ns/op (60 files resized). Real-terminal `scripts/drive` check with 12 active bash sessions verified `vineyard sessions` (12 sessions), `vineyard send`, Preview tab, and Diff tab (tracked + untracked highlighted cards).

[DONE] Ready for commit and pull request on `87/speed-up-many-sessions`.

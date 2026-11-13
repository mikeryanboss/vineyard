### 2026-11-13T16:15
[FINDINGS] `vineyard debug` on v0.1.44 showed `/tmp/vineyard-1000/vineyard-p6` running as PID 2579 (`/proc/2579/exe version` -> `0.1.43`, started at `15:53:28`). Because PR #71 kept `protocol = 6`, `vineyard 0.1.44` (`/go/bin/vineyard`) connected to that running `0.1.43` daemon both when creating new sessions and when attaching. Verified `/go/bin/vineyard` (`0.1.44`) directly on an isolated socket via `scripts/drive`: when running its own server, attached sessions render in color.

### 2026-11-13T16:15
[STARTED] Bump `protocol` to `7` in `internal/vterm/daemon.go`, make `TestVineyards` in `main_test.go` protocol-independent (`vterm.Protocol() + 1`), and update `docs/architecture.md`.

### 2026-11-13T16:21
[VERIFY] PASS.
- `gofmt -l .` clean; `go vet ./...` clean; `go test -count=1 . ./internal/vterm/... ./internal/session/...` PASS (0.86s, 1.97s, 2.76s); `go test ./...` PASS.
- Side-by-side upgrade check via `scripts/drive` on `.grapes/84/tmp/demo`: started `old-p6` under `/usr/local/bin/vineyard` (`0.1.43`, protocol 6), quit TUI leaving `p6` daemon running, then launched the protocol-7 build on the same repo and socket and created `new-p7`. `vineyard debug` confirmed `vineyard-p7` started as a separate process alongside `older` `vineyard-p6` (`old-p6` ready and alive), and attaching to `new-p7` rendered full ANSI SGR color sequences (`^[[31mRED_ON_P7^[[39m` and colored prompt).

### 2026-11-13T16:21
[DONE] Raised `protocol` to `7` in `internal/vterm/daemon.go`, updated `TestVineyards` in `main_test.go` to use `vterm.Protocol() + 1`, and updated `docs/architecture.md`.

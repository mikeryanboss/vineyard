### 2026-10-09T00:20
[STARTED] Blocked in practice on Mibokess/grapes#52, which provides the `embedded` package; developing against it with a local `go.work`.

### 2026-10-09T10:20
[DECISION] Grapes starts dark until the terminal reports its background; vineyard starts light (#7). Vineyard now sends grapes a light background at startup so both match until the report. Found in the screenshot run: tmux never reports, and grapes' dark palette was barely readable on the light canvas.

### 2026-10-09T10:25
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass (3.6 s). `grep -n replace go.mod` prints nothing; go.mod pins `github.com/Mibokess/grapes v0.1.11-0.20261009081705-50b5efb3d664` (PR Mibokess/grapes#37's head). `app_grapes_test.go` covers open/close with selection kept, the no-.grapes message, grapes seeing its own write and reload through vineyard's loop (fails when forwarding is removed), and the light start (fails without it). Manual, real terminal (tmux 110x30, demo repo, `-p bash`): `i` shows the board filling the screen, `q` returns; a worktree edit made while grapes was hidden showed up as a `#1` tag, so grapes reloads while hidden. Screenshots: `.grapes/9/tmp/*.png`, published in the PR. PASS.

### 2026-10-09T10:26
[DONE] `main.go` loads `embedded.New(<root>/.grapes)`; the root model forwards unhandled messages to grapes, routes keys and mouse to it while `issuesOpen`, and closes it on `CloseMsg`. `go.work` ignored; docs describe the Grapes screen.

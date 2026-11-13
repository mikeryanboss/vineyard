### 2026-11-13T23:45
[STARTED] Implementing mouse drag reordering of sessions, border drag resizing of the session list column, and persistent `list_width` config + settings support on branch `86/mouse-drag-resize`.

### 2026-11-13T23:55
[VERIFY] Ran `gofmt -l .` (clean), `go vet ./...` (exit 0), and `CGO_ENABLED=0 go test -count=1 ./...` (all packages passed in 7.233s). Verified in a real 120x35 tmux terminal via `scripts/drive` and SGR mouse escape sequences: dragged the first session below the second (persisted in `.vineyard/sessions.json`), dragged the pane's left border from column 36 to column 24 (persisted as `list_width = 24` in `.vineyard/config.toml` and shown as `Session list width 24` on the Config screen), restarted Vineyard to confirm both persisted, and captured `.grapes/86/tmp/before.png` and `.grapes/86/tmp/after.png`.
[DONE] Completed mouse session drag reordering, session column border resize with `list_width` config + Config screen persistence, tests, golden files, docs, and real-terminal verification on `86/mouse-drag-resize`.

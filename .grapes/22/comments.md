### 2026-10-09T10:20
[DONE] Added `PrevTab` (shift+tab) to `ListKeys` and `PaneKeys`; `SwitchTabMsg` gains `Back`; `switchTab(back)` uses the new `prevTab`. Preview, diffview, and textview forward shift+tab. README documents it. Status-bar hints still name only `tab`, to keep the line short.

### 2026-10-09T10:20
[VERIFY] `gofmt -l .` empty; `go vet ./...` and `go test ./...` PASS. New `TestApp_ShiftTabSwitchesToThePreviousTab` walks the tabs backward from the list and the focused pane. In a 110x30 tmux window with a dev build (`-p bash`), shift+tab (tmux `BTab`) cycled Preview -> Recap -> Issue -> Diff from the list, and Diff -> Preview -> Recap -> Issue -> Diff with the pane focused. PASS.

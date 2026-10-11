### 2026-10-11T16:30
[FINDINGS] Reproduced on 0910bc6 with hunk as the diff tool: a double click on a diff code line did nothing, on a file header it folded once, and on the Diff tab it opened hunk. The user reported the diff view as the one thing that does not work yet.

### 2026-10-11T16:30
[DECISION] Code lines open the full diff on a double click, as `enter` does for the tab, matching the preview, where a double click attaches. Headers and summary lines keep folding or jumping once: a double click there opening the diff tool too would act twice for one gesture. The first click decides, since a jump moves the line under the pointer.

### 2026-10-11T16:33
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.1 s. `TestApp_DoubleClickInThePane` fails with the old rule (drop every double click on the diff). The test helper now clicks one cell twice, as a real double click does; before, it looked the text up again for the second click, which after a jump landed on the header and folded it unnoticed. Manual, development build in a 140×36 tmux window, `bash` session with a two-file diff, `diff_command = 'hunk diff {base} --watch'`: a double click on the summary's `M shop.go` jumped to it once; on its header it folded once; on the code line `return n * 100` it opened hunk, and `q` returned to Vineyard. PASS. Screenshot in the PR.

### 2026-10-11T16:33
[DONE] `internal/tui/diffview/diffview.go`: `fileAt`, `Clickable`; `internal/tui/app.go`: `clickAt.acted`, `doubleClick` returns the first click, a diff double click is dropped only after a click that acted; tests; `README.md`, `docs/architecture.md`.

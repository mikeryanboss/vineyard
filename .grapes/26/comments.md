### 2026-10-09T11:50
[DECISION] Enter acts on the shown tab. The diff opens in a configurable command (Hunk by default), falling back to a full-screen zoom of our own diff view. We keep our diff view in the pane: mirroring Hunk there would add a hard dependency, a process per session, resize churn, and key forwarding. Recap zooms too, since the zoom is generic.

### 2026-10-09T13:58
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass. PASS.
Real terminal (tmux 110x30, demo repo, Hunk 0.23.0 linux-x64 release binary, sha256 checked): enter on Diff ran `hunk diff <base> --watch` in the worktree, showing the committed change, the uncommitted edit, and the untracked file; watch picked up a new edit; `q` returned to vineyard; the worktree index's sha256 was unchanged. With `diff_command` set to a missing program, enter zoomed our diff and the agent's window stayed 75x24; esc returned to the split view. Enter on Recap zoomed; enter on Issue behaved as `i`; enter on Preview and `o` on Recap attached. The config screen lists "Diff tool". PASS. Screenshots: .grapes/26/tmp/{split,zoom,hunk}.png.
Found on main too, not fixed here: on the Issue tab without a .grapes directory, the long placeholder overflows the pane and pushes the status bar off screen.

### 2026-10-09T13:59
[DONE] Enter opens the shown tab; `o` attaches. New `diff_command` config (default Hunk) with a zoom fallback. Files: internal/config/config.go, internal/tui/{app.go,backend.go}, internal/tui/common/keys.go, internal/tui/settings/settings.go, tests and goldens, README.md, docs/architecture.md.

### 2026-10-09T13:57
[PROGRESS] Renumbered from #25, which collided with the merged "Credit claude-squad in the README". The allocator could not see that issue: there is no .grapes/config.toml, and its worktree was gone while local main was stale.

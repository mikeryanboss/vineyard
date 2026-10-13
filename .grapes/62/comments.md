### 2026-10-13T14:00
[FINDINGS] Sampled three running Claude Code agents' screens every 10 ms for 60 s through Vineyard's server: the longest unchanged stretch was 454 ms, the median 110 ms. `stillFor` = 600 ms.

### 2026-10-13T14:12
[DECISION] Quick checks run as one self-paced chain (`quickStatusMsg` schedules the next), not one scheduled per result: in a first version, the pending tick's result and the detach's check each scheduled one, so two chains spent the ten checks in half a second.

### 2026-10-13T14:22
[VERIFY] `gofmt -l .` empty; `go vet ./...` and `go test ./...` pass (7.8 s). Manual, dev build with `-p bash` in tmux (110x30): start a loop printing every 100 ms in "busy", attach to "idle", let the loop end during the attachment, detach, capture the list every 50 ms. Time from detach until busy shows ready:

| Build | Attachment | Runs | Ready after |
| --- | --- | --- | --- |
| main (ce1e5c5) | 0.7 s | 6 | 1035–1093 ms |
| this branch | 0.7 s | 6 | 677–766 ms |
| main (ce1e5c5) | 6 s | 3 | 1022–1057 ms |
| this branch | 6 s | 3 | 734–765 ms |

Statuses that need no stillness (running, stopped) and subagents update on the first check, about 65 ms after detaching on both builds in these runs, since the tick pending during the attachment fired at once. In 1 of 13 runs of this branch the session just detached from showed running for about 0.5 s: the window's resize back to preview size changed its screen after a check (out of scope, see Context). PASS.

### 2026-10-13T14:24
[DONE] `internal/tui/app.go`: `stillFor`, `quickChecks`, `quickInterval`, `quickStatusMsg`, `seenScreen`, `statusMsg.at`; `attachDoneMsg` starts the quick checks. Tests: `TestApp_ReadyNeedsTheScreenStillForStillFor`, `TestApp_DetachingChecksStatusesQuickly`; status tests send timed checks via `polled`. Docs: Status Detection in `docs/architecture.md`, runtime model in `docs/README.md`.

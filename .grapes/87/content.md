## Goal
Eliminate the polling, git subprocess, transcript scanning, terminal rendering, and TUI layout bottlenecks that make Vineyard slow and resource-heavy when many sessions are open and actively working, while preserving existing TUI visuals, golden outputs, and protocol compatibility.

## Description
Six compounding bottlenecks degrade responsiveness and CPU/I/O usage with many active sessions:
1. `session.AgyConversation` scans all of `~/.gemini/antigravity-cli/history.jsonl` from byte 0 on every status poll for every active `agy` session (`internal/session/manager.go`).
2. Navigating with `j`/`k` triggers `diffCmd()` across all `N-1` unselected sessions serially; `diffInFlight`, `transcriptInFlight`, and `previewInFlight` are not reference-counted or guarded properly; `git.Diff` spawns up to 200 `git diff --no-index` subprocesses for untracked files; `git.Base` spawns 3–5 git processes every 2s despite commit graph immutability; `git.IsWorktree` shells out when the directory is missing; and `sessions.go` inspects sessions serially (`internal/git/diff.go`, `internal/git/git.go`, `internal/tui/app.go`, `sessions.go`).
3. `recap.Load` scans all `*.jsonl` files from byte 0 with a 64 MiB buffer every 2s on the Recap tab, and unchanged `recapMsg`/`transcriptMsg` re-wrap and re-style unconditionally (`internal/recap/recap.go`, `internal/tui/app.go`).
4. `vterm.Server.Capture`, `CaptureHistory`, and `bar.sample` call `t.emu.Render()` on every 150ms preview tick and 1s status poll even when no PTY output or resize happened (`internal/vterm/vterm.go`, `internal/vterm/bar.go`).
5. `subagent.Reader` holds a single coarse mutex across filesystem I/O for all sessions, and `subagent.Parse` strips ANSI and regex-scans every Claude screen even when unchanged or lacking a subagent list (`internal/subagent/transcript.go`, `internal/subagent/subagent.go`).
6. `diffview.Model` runs Chroma syntax highlighting and word-diffing even when hidden on other tabs and re-runs Chroma on every column resize; `renderPane` renders `m.preview.View()` even when discarded; `preview.Model.View()` re-truncates/pads unchanged screens; `reloadedMsg` calls `syncList()` when sessions did not change; `updateGrapes` runs `refreshIssue()` on cursor blink messages; `shownSubagent`/`mainIssues` do avoidable work; and `list.count` clones the slice three times per frame (`internal/tui/`).

## Context
- `internal/session/manager.go`, `internal/session/session.go`, `internal/session/session_test.go`: `AgyReader` incremental reader, `Session.Equal`, `Manager.Locate` empty fast path.
- `internal/recap/recap.go`, `internal/recap/recap_test.go`: `Reader` incremental reader and `Recap.Equal`.
- `internal/vterm/vterm.go`, `internal/vterm/bar.go`, `internal/vterm/vterm_test.go`: cached rendered screen & hash inside `terminal` under `t.mu` without bumping `protocol` (`7`) or `sameCallsSince` (`1`).
- `internal/git/diff.go`, `internal/git/git.go`, `internal/git/git_test.go`: pure-Go untracked diff formatting, commit-SHA `Base`/`StackedBase` cache, fast `IsWorktree`.
- `internal/subagent/subagent.go`, `internal/subagent/transcript.go`, `internal/subagent/transcript_test.go`: fast-path screen check, per-directory lock & `os.ReadDir`.
- `internal/tui/app.go`, `internal/tui/backend.go`, `internal/tui/app_test.go`, `main.go`, `sessions.go`: polling counters, `selectedDiffCmd` vs bounded-concurrent `diffCmd`, hash-cached status checks, no-op skips, parallel `vineyard sessions`.
- `internal/tui/diffview/diffview.go`, `internal/tui/diffview/spans.go`, `internal/tui/diffview/diffview_test.go`, `internal/tui/preview/preview.go`, `internal/tui/list/list.go`: lazy `SetVisible`, width-independent `preparedFile` cache, `runeWidth` fast path, cached `preview` render, single-pass `list.count`.
- `docs/architecture.md`: document `recap.Reader` incremental reading alongside `subagent.Reader`.

## Acceptance Criteria
- [x] Incremental `session.AgyReader` and `recap.Reader` avoid re-reading unchanged bytes, handle partial EOF lines safely, handle truncation/replacement/deletion cleanly, and eliminate the 64 MiB `recap` scanner allocation.
- [x] `vterm` caches rendered ANSI output and its FNV-64a hash inside `terminal` (invalidated on PTY write and resize) without changing `vterm` protocol or RPC signatures.
- [x] `git.Diff` formats untracked files in pure Go without spawning `git diff --no-index` per file or touching the worktree index; `git.Base` and `git.StackedBase` cache fork commits by SHA pair; `git.IsWorktree` skips nonexistent paths; and `vineyard sessions` runs with bounded concurrency.
- [x] `subagent.Reader` locks per transcript directory and uses `os.ReadDir`; `subagent.Parse` short-circuits when neither `"● main"` nor `"◯ main"` is present.
- [x] Cursor navigation (`selectedDiffCmd`) polls only the selected session while background `diffCmd` polls unselected sessions concurrently; `previewLoads`, `transcriptLoads`, and `diffMsg.poll` prevent overlapping polls; unchanged status screens, reloads, transcripts, recaps, and blink messages skip redundant work; `diffview` defers rendering when hidden and caches Chroma highlighting across resizes; `preview.View()` caches rendered frames; and `list.count` runs in one pass.
- [x] All existing golden/unit tests pass without visual regressions, new regression tests & benchmarks cover all changes, `docs/architecture.md` reflects `recap.Reader`, and `go test -count=1 ./...` completes in under 10 seconds.

## Verify
Run from `/var/home/mboss/dev/vineyard/.vineyard/worktrees/slowness-80bb17`:
```bash
gofmt -l .
go vet ./...
go test -race ./internal/session ./internal/recap ./internal/subagent ./internal/vterm ./internal/git
time CGO_ENABLED=0 go test -count=1 ./...
```
Plus targeted benchmarks and a real-terminal `scripts/drive` smoke/screenshot check with 12+ active sessions on branch `87/speed-up-many-sessions`.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` exits 0; `-race` tests pass with zero data races; `go test -count=1 ./...` passes all packages in under 10 seconds; benchmarks confirm order-of-magnitude speedups on idle polls, diffs, and view updates; and real-terminal `scripts/drive` screenshots render cleanly.

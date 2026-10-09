## Goal
Several Vineyard processes run on one repository at once, for example one per terminal window. Each shows every session, sees the others' changes within a few seconds, and none loses or duplicates another's work.

## Description
Today a second `vineyard` in the same repository exits with `another vineyard is already running for this repository`. The lock exists because a whole-process lock was the simplest way to keep two processes from corrupting each other's state. Three things go wrong without it:

1. **Lost writes.** Every save writes the process's whole in-memory list to `sessions.json`, so one process's save drops the sessions another created and brings back sessions another killed.
2. **Stale views.** A process reads `sessions.json` only at startup, so it never sees another's sessions.
3. **Doubled keystrokes.** Every process polls every agent's screen. Each would press Enter on an auto-yes session's permission prompt and paste a pending prompt, so an agent would get its prompt twice or an Enter on its next prompt.

Changes:

- **Writes are per session and under a lock.** `session.Store` gains `Add`, `Replace`, and `Remove`. Each takes an exclusive `flock` on `.vineyard/sessions.lock`, reads `sessions.json`, changes one session, and writes the file back. `Replace` and `Remove` leave other sessions alone, and `Replace` does nothing for a session that is no longer saved, so a process's status poll cannot bring back a session another process killed. `Save` of the whole list goes away; startup reconciliation (`Manager.Restore`) runs inside the same lock.
- **Each process reloads.** On every status tick (1 s), the model reads `sessions.json` and adopts it: it adds sessions it lacks, drops sessions no longer saved, and takes saved fields. Its own running/ready state wins over the saved one while both are active, because the screens it polls decide those. It keeps sessions it is starting or operating on (`busy`, `StatusLoading`). It drops a reload result whenever one of its own writes was in flight or issued since the reload started, so stale data never undoes its own change, such as a cleared pending prompt.
- **One process leads.** The existing `.vineyard/lock` flock becomes the lead role instead of a gate. Only the leader presses Enter for auto-yes and delivers pending prompts. A process that does not lead tries to take the role on each status poll, so another process takes over within a second of the leader exiting.
- `Manager.Locate` (planned) is split from `Restore`: it fills `WorktreePath` and live branches from `git worktree list`, and the reload uses it. `Restore` keeps the worktree repair and status reconciliation that only startup needs.

Out of scope, recorded as known limits in `docs/architecture.md`: two processes previewing the same session at different terminal sizes each resize its tmux window when their own size changes; and saves of `config.toml` from two config screens are last-write-wins.

## Context
- `internal/session/lock_unix.go` `Lock`: non-blocking `LOCK_EX` on `.vineyard/lock`; `ErrLocked` in `internal/session/store.go`. `lock_other.go` is a no-op for non-Unix builds.
- `main.go` `runTUI`: `session.Lock(dir)`, `store.Load()`, `manager.Restore(...)`, `store.Save(...)`.
- `internal/tui/backend.go` `Backend.Save(sessions)`; `internal/tui/app.go` `saveCmd` snapshots `m.sessions` after start, pause/resume, kill, auto-yes toggle, and status changes (stopped, pending prompt cleared).
- `internal/tui/app.go` `applyStatus`: sends Enter for `r.permission && s.AutoYes`, Enter for the trust prompt with auto-yes, and `promptCmd` for `PendingPrompt`.
- `internal/session/manager.go` `Restore`: repairs worktrees, then sets `WorktreePath`, `Branch`, and `Status`.
- `docs/architecture.md` Storage section and `docs/README.md` describe the lock.

## Acceptance Criteria
- [ ] A second `vineyard` in a repository where one already runs starts instead of failing with `ErrLocked`.
- [ ] A session created in one vineyard appears in the other within 2 s; killing, pausing, or resuming it in one shows in the other within 2 s.
- [ ] Concurrent store writes from two `Store` values keep each other's sessions (unit test).
- [ ] `Replace` of a session no longer saved does not re-add it (unit test).
- [ ] A vineyard that does not lead neither presses Enter for auto-yes nor pastes a pending prompt; the leader does (unit test).
- [ ] A reload result is dropped when a write was issued after it started, so a cleared pending prompt is not restored (unit test).
- [ ] A reload adds sessions saved by another process, drops sessions it removed, and keeps sessions this process is starting (unit test).
- [ ] When the leading vineyard exits, another running vineyard delivers pending prompts (manual check).
- [ ] `docs/architecture.md` and `docs/README.md` describe the store lock, reloads, and the lead role instead of the single-process lock.

## Verify
Run from the repository root:
```bash
gofmt -l .
go vet ./...
go test ./...
```
Manual, in real terminals (see `docs/development.md`): start two vineyards in one repository in two tmux windows. Create a session with a prompt in the window that does not lead; confirm it appears in the other and receives its prompt once. Kill it from the other window; confirm it disappears from both. Quit the first vineyard, create a session with a prompt in the second, and confirm the prompt arrives.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under 10 s. In the manual check, both windows show the same sessions, each prompt is pasted exactly once, and the remaining vineyard takes over prompt delivery after the first exits. Screenshots of both windows are attached to the PR.

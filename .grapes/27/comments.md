### 2026-10-09T12:05
[DECISION] Keep `sessions.json` and lock each write rather than splitting it into one file per session: a reload is needed either way, and keeping the file avoids migrating existing sessions. A lead role, not claims per prompt, stops doubled keystrokes, because auto-yes Enters have nothing to claim.

### 2026-10-09T14:20
[DECISION] Writes run as commands that bump `Model.writes`; a reload starts only with no write pending and is dropped when `writes` moved while it ran. This, not timestamps, keeps a stale reload from restoring a delivered pending prompt.

### 2026-10-09T14:25
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` passes; `go test -count=1 ./...` passes in 5.5 s wall. New tests: `TestStore_ConcurrentWritesKeepEachOthersSessions`, `TestStore_ReplaceDoesNotReviveRemovedSession`, `TestLeader_OneAtATime`, `TestApp_OnlyTheLeaderTypesIntoAgents`, `TestApp_StaleReloadIsDropped`, `TestApp_ReloadAdoptsOtherVineyardsSessions`; each of the three TUI tests fails with its guard disabled. The store and lead tests pass under `-race`. PASS.

### 2026-10-09T14:26
[VERIFY] Manual, two vineyards in 110x30 tmux windows on a throwaway repo, `-p bash` (script `.grapes/27/tmp/scenario.sh`). The `main` build's second vineyard exits with `another vineyard is already running for this repository`. With this branch: a session with a prompt created in the follower appears in the leader and its prompt runs once; a session created in the leader appears in the follower; a kill in the leader removes it from the follower; after the leader quits, the follower delivers a new session's prompt. Screenshots in the PR. PASS.

### 2026-10-09T14:27
[DONE] Store writes are per session under `.vineyard/sessions.lock` (`internal/session/store.go`); `session.Leader` (`internal/session/leader.go`) turns `.vineyard/lock` into the lead role; `Manager.Locate` split from `Restore`; the TUI reloads and adopts saved sessions each status tick and types into agents only when leading (`internal/tui/app.go`, `backend.go`); `main.go` no longer refuses a second vineyard. Docs: `docs/architecture.md` Several Vineyards, `docs/README.md`, `README.md`.

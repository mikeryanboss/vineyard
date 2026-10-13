## Goal
One Vineyard runs sessions in different multiplexers side by side: some on
Vineyard's own server, some in tmux. The multiplexer belongs to the session,
not to the Vineyard process. New sessions default to Vineyard's server.

## Description
#53 made the multiplexer a process-wide choice. `main` picks one terminal for
the whole run, and refuses to start while a saved session is alive in the
other (`runningElsewhere`). The user hit that refusal with a tmux session still
running and `vineyard` selected.

Instead:
- Each session records the multiplexer that runs it (`Session.Multiplexer`,
  saved in `sessions.json`). Every terminal operation on the session (start,
  exists, capture, keys, paste, resize, attach, kill, its shell) goes to that
  multiplexer.
- `multiplexer` in `config.toml` is the default for new sessions: `vineyard`
  (the default) or `tmux`. A profile may set its own `multiplexer`, which
  overrides the default for its sessions.
- At startup, `Restore` looks for each session's terminal in every available
  multiplexer, its recorded one first, and records where it found it. A saved
  session without a multiplexer, from before this change, that runs nowhere
  gets the configured default.
- tmux is available when installed. A session on tmux without tmux installed
  shows as stopped; starting or resuming it fails, saying tmux is not
  installed. Startup no longer fails for either reason.
- Adopting a tmux session Vineyard did not start is out of scope; this change
  makes it possible later.

## Context
Verified on main (29f9800, rebased onto 95867a6):
- `session.Manager` has one `Terminal`; `tui.LiveBackend` has one `Terminal`
  (`internal/tui/backend.go`), both from `multiplexers` in `main.go`.
- `runningElsewhere` in `main.go` refuses startup; `TestRunningElsewhere_*`
  and `TestMultiplexers_WithoutTmux` in `main_test.go` test #53's behaviour.
- `config.Config.Multiplexer` is empty by default, meaning tmux when installed.
  Files saved by the config screen since #53 may hold `multiplexer = ''`.
- The config screen offers `auto`, `tmux`, `vineyard` (`idMultiplexer`); profile
  rows toggle `main_checkout` with `w` (`keys.Where`).
- `profileSummary` in `internal/tui/dialog/dialog.go` describes what a new
  session runs.

Planned:
- `Manager.Terminals map[string]Terminal`, `Manager.Default`; `LiveBackend.Terminals`.
- `config.Profile.Multiplexer`, `Config.MultiplexerFor(profile)`.
- Config screen: General "Multiplexer for new sessions" (`tmux`, `vineyard`);
  `m` on a profile cycles its multiplexer: default, `tmux`, `vineyard`.
- Removed: `runningElsewhere` and its test.

## Acceptance Criteria
- [x] One Vineyard shows, attaches to, and kills a tmux session and a
      Vineyard-server session at once.
- [x] Startup succeeds with sessions alive in a multiplexer other than the
      configured default.
- [x] New sessions run on Vineyard's server unless `multiplexer` or their
      profile says tmux.
- [x] Restore records where it found each live session; a session saved
      without a multiplexer that runs nowhere gets the default.
- [x] Starting a session on tmux without tmux installed fails with an error
      saying so.
- [x] Docs describe per-session multiplexers.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual, in a real terminal through a private tmux socket: with a tmux
session from before still running and the default `vineyard`, start Vineyard,
create a Vineyard-server session, and show, attach to, and kill both.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass in under 10 seconds. The
manual run shows both sessions in one list, each attachable.

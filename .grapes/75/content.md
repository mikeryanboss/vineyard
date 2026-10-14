## Goal
Killing or checking out a session never leaves its agent running. Today, when
a session's record names a Vineyard server other than the one running its
agent, Vineyard shows the session stopped, and Kill deletes its worktree and
record while the agent keeps running: an orphan that other Claude Code
sessions still list as a peer.

## Description
Four such orphans ran on 2026-10-14: `claude` processes in deleted worktrees
(`test-e72254`, `test2-af608f`, `test-b497c5`, `test3-c0a0c8`), absent from
`sessions.json`, on the protocol-1 server (shared socket `vineyard`) and the
protocol-3 server (`vineyard-p3`).

Reproduced (`.grapes/tmp/repro/` in the #74 worktree, scripted with
`scripts/drive`): start a session with v0.1.26 (protocol 3, program
`sleep`), quit, set its `protocol` to 5 in `sessions.json`, open the current
build. The list shows `× stopped`; `D`, `y` reports `Killed h1.`; afterwards
the agent still runs, the session and worktree are gone.

A plain kill works: every combination of starters v0.1.22, v0.1.26, v0.1.27
and killers v0.1.28 to v0.1.36 and main ended the agent, also with real
Claude Code. So the real orphans had wrong records; how they got them is not
established (Oct 13's `vineyard dev` builds created them while the protocol
changed, beside several Vineyard versions running at once). The fix makes
Vineyard robust to a wrong record, whatever wrote it.

Evidence the protocol-3 server never received Kill for them: Kill removes a
terminal from the server's map at once, and the server stops when it holds
no terminals and no connections; it stopped only right after the agents were
SIGTERMed by hand (`server stopping: no terminals or connections` at
2026-10-14 15:14:33).

## Context
- `Manager.running` (`internal/session/manager.go`) looks for a session's
  terminal on the multiplexer its record names, then on the other entries of
  `Manager.Terminals` (this build's Vineyard server and tmux). It skips the
  recorded multiplexer's name, so a session recorded on `vineyard` with the
  wrong protocol is never checked on this build's server, and older servers
  other than the recorded one are never checked.
- `Manager.Restore` marks a session stopped when `running` finds nothing.
- `Manager.stopTerminals` (used by `Kill` and `Pause`) kills on the recorded
  multiplexer only; a server without that terminal answers success
  (`vterm.Client.Kill`: "A missing terminal is not an error").
- `vineyards.protocols` in `vineyards.go` lists the protocols whose sockets
  exist, this build's first; `vineyards.Older` returns a terminal for each.

Change:
- `running` returns where the terminal runs (multiplexer and protocol),
  checking the recorded one, then every entry of `Terminals`, then every older
  Vineyard server (planned field `Manager.Protocols`, set from
  `vineyards.protocols` in `main.go`).
- `Restore` records what `running` found.
- `stopTerminals` stops the agent and shell where `running` finds the agent.
  An older server whose calls differ refuses the kill
  (`olderVineyard.Kill`), so Kill stops instead of orphaning it.

## Acceptance Criteria
- [ ] Restore finds an agent on an older server that its record does not name, marks the session ready, and records that protocol.
- [ ] Restore finds an agent on this build's server when the record names an older protocol.
- [ ] Kill and Pause end an agent that runs on a server other than the recorded one.
- [ ] The reproduction above no longer leaves the agent running.

## Verify
Run from the worktree:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual: repeat the reproduction with a build of this branch as the second Vineyard.

## Pass Criteria
Tests pass. In the reproduction the session shows ready, not stopped, and after `D`, `y` no `sleep` process remains.

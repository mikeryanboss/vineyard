## Goal
A newer Vineyard attaches to sessions on an older Vineyard multiplexer,
whatever protocol that multiplexer speaks, so that updates never strand
sessions again. No old protocol code is kept: the older server's own
executable does the attaching.

## Description
Attaching is the part of the protocol that changes (#58, #61), and a build's
client speaks only its own protocol. Every server process keeps the
executable it started from: on Linux, `/proc/<pid>/exe` runs it even after
`go install` replaced the file. A new hidden command, `vineyard attach
<socket> <terminal> <attachment as JSON>`, attaches with this build's client.
A newer Vineyard attaches to an older multiplexer's session by running that
server's executable with this command, handing it the user's terminal.

The command's arguments are the stable contract between versions: JSON
ignores fields it does not know, so the attachment can grow. Builds before
this command exit with status 2 (usage), which the caller reports as
predating it. Servers started by builds before this change cannot be
attached this way; no workaround for them (the user's decision).

## Context
- `vterm.NewOlderClient` (#63) and `olderVineyard` (`vineyards.go`, #60)
  fail `Attach` today.
- `session.Attachment` and `session.Peer` (`internal/session/manager.go`)
  describe an attachment; the status bar's peers come from `attachment` in
  `internal/tui/app.go`, which filters by multiplexer but not protocol.
- `main.run` handles `server <socket>`, the other command Vineyards run.
- `macOS`: no `/proc`; attaching to an older multiplexer fails there with an
  error saying so.

## Acceptance Criteria
- [x] `vineyard attach <socket> <terminal> <json>` attaches with this build's
      client and exits 1, never 2, on failure.
- [x] Attaching to a session on an older multiplexer, of matching calls or
      not, runs `/proc/<pid>/exe attach …` of that server and returns when the
      user detaches; a build without the command is reported as predating it.
- [x] `session.Attachment` has JSON names documented as the contract.
- [x] The status bar lists only sessions on the same multiplexer and protocol.
- [x] A test attaches through a real server's own executable.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then with `scripts/drive`: a session started by a build of this branch with
`protocol` lowered, then this branch's build: attach, type, detach. And a
session from a build of 73d94db: attach reports that it predates the command.

## Pass Criteria
Checks pass; the real-app steps behave as described.

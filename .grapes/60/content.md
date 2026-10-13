## Goal
A Vineyard that cannot reach the server running this repository's agents
says so and stops, instead of showing those agents, still running, as stopped.

## Description
#58 raised the server protocol from 1 to 2 (`internal/vterm/daemon.go`). A
Vineyard built after it refuses a server started by an earlier build, which
keeps running while it has agents. Found with `vineyard debug` (#59): the
user's server, process 468432, speaks protocol 1, and runs two of this
repository's sessions, while `main` builds speak protocol 2.

Reproduced in a throwaway repository with `multiplexer = "vineyard"`: a
session started by a build of 73d94db (protocol 1), then Vineyard from this
branch (protocol 2):
- the session shows `× stopped` and "The agent is no longer running", while
  its agent runs;
- `r` and `n` fail with `the Vineyard server at … is from another version of
  Vineyard (protocol 1, wa…`, cut off before the part saying what to do.
This is likely the "crash" the user saw after #56 and #58 merged; they did not
see which part failed.

## Context
- `Manager.Restore` (`internal/session/manager.go`) marks every session
  whose terminal no multiplexer reports as `stopped`.
- `vterm.Client.Exists` reports false when `connect` fails on the protocol.
- `runTUI` in `main.go` restores sessions before the TUI starts.

## Acceptance Criteria
- [x] When Vineyard's server speaks another protocol and this repository
      records sessions on it that are neither paused nor stopped, `vineyard`
      exits before the TUI with an error naming the server's process, both
      protocols, the sessions, and what to do.
- [x] Otherwise Vineyard starts as before; a mismatched server with none of
      this repository's running sessions on it fails only Vineyard sessions'
      operations, as now.
- [x] The mismatch error leads with what to do, so the status line shows it.
- [x] A test covers the refusal and the cases that start.

## Verify
Run from the worktree root:
```bash
go test ./... 
```
Then repeat the reproduction above with this branch's build.

## Pass Criteria
Tests pass; the reproduction prints the error and exits before the TUI.

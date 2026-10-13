## Goal
After an update that changes the Vineyard multiplexer's protocol, agents
started by the earlier Vineyard keep running and are shown as running, and
new sessions start on the new multiplexer, without ending the old agents.

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
- Sessions already record their multiplexer (`Session.Multiplexer`, #56).
- The user chose one Vineyard multiplexer per protocol over refusing to
  start (the first version of this fix).

## Acceptance Criteria
- [x] Each protocol's Vineyard multiplexer listens on a socket of its own,
      `<name>-p<protocol>`; protocols 1 and 2 shared `<name>`.
- [x] A session records the protocol it runs on; sessions recorded before
      count as on `<name>`.
- [x] A session on an older multiplexer whose agent still runs is restored as
      ready, keeps its status while the agent runs, shows why its screen is
      missing, and fails attach, kill, and resume with an error saying to use
      the earlier Vineyard or let the agent finish. Once its agent ended, it
      shows as stopped and resumes on this build's multiplexer.
- [x] New sessions start on this build's multiplexer while the older one runs.
- [x] `vineyard debug` lists every Vineyard multiplexer and each session's.
- [x] Tests cover the socket names, the older multiplexer's calls, restore,
      and resume.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then, with `scripts/drive`: start a session with a build of 73d94db
(protocol 1), quit, start this branch's build in the same repository, start a
session, select the old one, attach to it, end its agent, and resume it.

## Pass Criteria
Checks pass. The old session shows as ready with the explanation in its
preview; the new session runs; attaching to the old one fails with that
explanation; after its agent ends it shows stopped, and `r` restarts it on
this build's multiplexer.

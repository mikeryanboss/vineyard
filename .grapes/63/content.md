## Goal
After an update, sessions on an older Vineyard multiplexer show their screen,
status, prompts, and can be killed, as long as that multiplexer's calls match
this build's. Only attaching needs the Vineyard that started them.

## Description
#60 runs a Vineyard multiplexer per protocol and reaches an older one only
through `Version` and `Exists`: its sessions show a message instead of their
screen. The user expected to see them. The calls themselves did not change
from protocol 1 to 3; only attaching did (#58 added `Detach []byte` to
`AttachRequest`, #61 made it `[]string` and added the status bar fields).

Backward compatibility with the servers that exist now costs nothing, so this
build uses their calls. It promises nothing for later protocols: a constant
records the oldest protocol whose calls match, and a change to a call raises
it, after which older servers fall back to #60's message.

## Context
Verified by comparing `internal/vterm/daemon.go` at 73d94db (protocol 1),
7b79d83 (2), and 81fe8ca (3): `Version`, `Start`, `Exists`, `Created`,
`Capture`, `CaptureHistory`, `Resize`, `SendKeys`, `Paste`, and `Kill` have
the same arguments and results; protocol 3 adds `List`.
- `vterm.Client.connect` refuses a server of another protocol.
- `vineyards.Older` (`vineyards.go`) returns `olderVineyard`, which answers
  only `Exists`, unless the server speaks this build's protocol.

## Acceptance Criteria
- [x] `internal/vterm` records the oldest protocol whose calls match this
      build's (1), with a comment saying when to raise it.
- [x] A client of an older multiplexer whose calls match makes every call,
      never starts a server, and refuses to attach with an error naming the
      process and protocol and saying to attach with the Vineyard that
      started it.
- [x] `vineyards.Older` returns that client for such multiplexers, and
      `olderVineyard` only for older protocols.
- [x] `vineyard debug` says which older multiplexers this build can use.
- [x] Tests cover both kinds of older multiplexer.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then with `scripts/drive`: a session started by a build of 73d94db, then this
branch's build: the session's screen shows, typing a prompt reaches it, attach
fails with the message, and `D` kills it.

## Pass Criteria
Checks pass; the real-app steps behave as described.

### 2026-10-13T16:30
[DECISION] Backward compatibility with today's servers, no promise for later ones: `sameCallsSince` (1) records the oldest protocol whose calls match; a change to a call raises it and older servers fall back to #60's message-only `olderVineyard`. The older client never starts a server and fails attaching with `session.ErrOlderMultiplexer`, leading with "use the Vineyard that started it" so the status line shows it. An older client keeps a connection open, so the older server exits once its agents end and the newer Vineyard quits, as for any server.

### 2026-10-13T16:30
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` passes. New or changed: `TestVineyards` (an older multiplexer of matching calls captures and refuses to attach; one of different calls only tells; one of this protocol attaches as far as needing a terminal), `TestOlderClient` (never starts a server; `SameCalls` covers protocols 1 to 3). PASS.

### 2026-10-13T16:30
[VERIFY] Real app via `scripts/drive`: a build of 73d94db started a session printing the time each second; this branch's build showed it `◑ running` with the live screen; attaching showed `attach: use the Vineyard that started it; this agent runs on an older Vineyard multiplexer (process 647394, protocol 1)…`; `D` killed it; after quitting, the protocol-1 server exited. `vineyard debug` on the user's repository shows the protocol-1 server as "shows its agents but cannot attach to them", and its three sessions alive. Screenshot: .grapes/63/tmp/after-attach.png (published with the PR). PASS.

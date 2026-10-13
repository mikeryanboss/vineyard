### 2026-10-13T15:50
[DECISION] Refuse to start, rather than mark the sessions unreachable in the TUI. Vineyard has no status for "running where this build cannot see", and adding one touches the list, the preview, and every operation; refusing fails explicitly with what to do, in a few lines of `main.go`. The refusal applies only while this repository records running or ready sessions on the server: the server is per user, and another repository's old agents should not block this one. Paused and stopped sessions have no agent to lose. Once a mismatched build has marked sessions stopped, the check no longer sees them; with this change no build marks them so.

### 2026-10-13T15:50
[VERIFY] `go test -count=1 ./...` passes, including `TestCheckServer` (refuses for running and ready sessions on a mismatched server; starts with no server, the same protocol, or paused, stopped, or tmux sessions). PASS.

### 2026-10-13T15:50
[VERIFY] Real app via `scripts/drive`: a session started by a build of 73d94db (protocol 1), then this branch's build in the same repository exits with status 1 before the TUI: `Vineyard's server, process 521774, is from another version of Vineyard: it speaks protocol 1, this Vineyard 2. It may still run the agents of "old", which this Vineyard cannot reach. Let them finish under the Vineyard that started the server, or end them all with: kill 521774`. In a second repository without sessions on that server, Vineyard starts, and creating a session shows `starting "bash": end the agents of Vineyard's server, process 521774, or kill it: …` in the status line, with the full error in `.vineyard/vineyard.log`. PASS.

### 2026-10-13T16:20
[DECISION] Superseded the refusal at the user's request: one Vineyard multiplexer per protocol, on `<name>-p<protocol>`, with sessions recording their protocol (`Session.Protocol`). An older multiplexer is reached only through `Version` and `Exists`, which every protocol answers alike (checked against 73d94db's daemon.go) and which must never change; everything else fails with `session.ErrOlderMultiplexer`. A session recorded before (protocol 0) is looked up on `<name>`; if that server speaks this build's protocol, as a protocol-2 server from a build between #58 and this change does, it is used fully. No protocol bump: the call format did not change.

### 2026-10-13T16:20
[FINDINGS] The long error overflowed the preview: `preview.View` placed its placeholder without wrapping, so the terminal wrapped it and pushed the screen down, hiding the status line. Fixed by wrapping placeholders to the pane width (`TestPreviewView_LongPlaceholderFitsPane`); this also affects any long error in the terminal tab.

### 2026-10-13T16:20
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` passes. New: `TestVineyards` (socket names; an older multiplexer's Exists, Capture, Kill, Start; a same-protocol server on another socket used fully), `TestOlderVineyardMultiplexer` (restore, resume refused while the old agent runs, then resumed on protocol 2), `TestApp_AgentOnAnOlderMultiplexerKeepsRunning` (fails with either TUI change reverted). PASS.

### 2026-10-13T16:20
[VERIFY] Real app via `scripts/drive`, with a build of 73d94db: the old session shows `● old` ready with the explanation in its preview; a new session starts on `vineyard-drive-dev-p2`; attaching to the old one shows `attach: this agent runs on an older Vineyard multiplexer … (process 561814, protocol 1)`; `vineyard debug` lists both multiplexers and `old: older vineyard, protocol 1 or 2, ready, terminal alive`. After ending the old agent (`kill -HUP` of its shell), it showed `× old` stopped; `r` restarted it, and `vineyard debug` showed it on this build's multiplexer, and the older multiplexer gone. Screenshots: before.png, after-attach.png in .grapes/60/tmp (published with the PR). PASS.

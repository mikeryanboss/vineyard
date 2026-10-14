### 2026-10-14T16:45
[STARTED] Reproduced in the #74 session (see Description). Plan: make `running` search every multiplexer and older server, and have Restore and stopTerminals act on what it finds.

### 2026-10-14T17:05
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` PASS. New `TestAgentsOnMultiplexersTheirRecordsDoNotName` (Restore finds an agent on an older server and one on this build's server against their records; Kill and Pause end them); with `stopTerminals` ignoring what `running` finds, it fails on both. Manual: `.grapes/75/tmp/repro/wrong-record.sh` (v0.1.26 starts a `sleep` agent on protocol 3, its record is set to 5, the second Vineyard opens and kills it). v0.1.37: `× 1 stopped`, `Killed h1.`, agent still running. This branch: `● 1 ready`, record corrected to protocol 3, `Killed h1.`, no agent left. PASS.

### 2026-10-14T17:06
[DONE] `Manager.running` searches every multiplexer and older Vineyard server (new `Manager.Protocols`, set from `vineyards.protocols`); `Restore` records what it finds; `stopTerminals` (Kill, Pause) stops the agent there. Architecture doc updated.

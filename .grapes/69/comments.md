### 2026-10-13T15:52
[FINDINGS] Diagnosed from a stuck session on another machine (`/worldmodels`, Claude Code pid started Oct 9, never restarted). The two subagents Vineyard showed running had no task notification after their last work. Around each stop the conversation held `system`/`agents_killed` and a queued "Background agent "<description>" was stopped by the user." with no task ID. The subagents' transcripts then got a rejected tool result and `[Request interrupted by user for tool use]`, about 50 ms after the `agents_killed` line. One of them had completed at 07:10:44 and resumed on its own at 07:15 before the stop. Claude Code 2.1.296's binary defines `agents_killed` as `{type,subtype,timestamp,uuid,isMeta}` and draws it "All background agents stopped".

### 2026-10-13T15:52
[DECISION] Use `agents_killed` as a stop for every subagent whose last work is no later than it, and stop counting tool results and interruption markers as work. Matching the queued "was stopped by the user" text by description was rejected: descriptions are not unique. A stop of a single subagent that writes neither line stays undetected; checking Claude Code's on-screen list would cover it, and is left out of this issue.

### 2026-10-13T15:52
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test -count=1 ./...` passes in 7.1 s. The new test fails without the fix (busy1 and resumed2 running, interruption marker shown as a prompt) and passes with it. On this machine's real transcripts, statuses are unchanged: the `TaskStop`ped probe subagent is still stopped, and three completed subagents are still done. PASS. Pending: the reviewer checks a session with subagents stopped by ctrl+x ctrl+k in a build of this branch.

### 2026-10-13T15:52
[DONE] `internal/subagent/transcript.go`: read `agents_killed`; `last` counts only messages written or sent; interruption markers are no messages. `internal/subagent/transcript_test.go`: `TestLoad_SubagentsTheUserStoppedStopped`. `docs/architecture.md`: Subagents status rules.

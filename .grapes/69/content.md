## Goal
A subagent that Claude Code stopped along with all background agents shows as
stopped in Vineyard, instead of running for as long as that Claude Code runs.

## Description
When Claude Code 2.1.296 stops all background agents (ctrl+x ctrl+k, among
others), it writes no task notification for them. The conversation's
transcript gets a `system` line with subtype `agents_killed`, which Claude
Code draws as "All background agents stopped", and a queued plain-text
"Background agent "<description>" was stopped by the user." Neither carries a
task ID. Each stopped subagent's own transcript ends with a rejected tool
result and a `[Request interrupted by user for tool use]` message, written a
few milliseconds after the `agents_killed` line.

Vineyard misses all of this, so it shows those subagents as running until
Claude Code exits. Seen in a session on another machine, where Claude Code had
run for four days: two subagents stopped at 07:17:44 and 08:51:29 UTC stayed
running in Vineyard, while Claude Code no longer listed them. Nobody stopped
them on purpose; what made Claude Code stop them is unknown.

## Context
- `internal/subagent/transcript.go`: `Load` marks a subagent done or stopped
  from task notifications (`transcript.stops`), and stopped when its transcript
  predates the Claude Code that runs it. `transcript.last` is the time of its
  last user or assistant line, tool results and interruption markers included.
  `add` reads a conversation's lines only when they hold a `markers` entry.
- `docs/architecture.md`, section Subagents, lists the status rules.
- Claude Code's binary (2.1.296) defines the line as
  `{type:"system",subtype:"agents_killed",timestamp,uuid,isMeta:false}`.

## Changes
- Read `agents_killed` lines from the conversation's transcript: a subagent
  still running by the other rules, whose last work is no later than the
  latest such line, is stopped then.
- A subagent's last work is its last assistant message or message sent to
  it. Tool results and Claude Code's `[Request interrupted by user…]` markers
  do not count: Claude Code writes them as it stops the subagent, after the
  line that reports the stop.
- An interruption marker is not a message sent to the subagent, so its tab
  does not show it as one.

## Acceptance Criteria
- [x] A subagent working when an `agents_killed` line was written is stopped,
  ended at that line, even though its transcript ends with a rejected tool
  result and an interruption marker written after it.
- [x] A subagent that completed, then resumed and worked on, before an
  `agents_killed` line is stopped at that line.
- [x] A subagent started, or sent a message, after the latest `agents_killed`
  line stays running.
- [x] The interruption marker is not among the subagent's events as a prompt.
- [x] `docs/architecture.md` describes the rule.

## Verify
Run from the worktree root:
```bash
gofmt -l .
go vet ./...
go test ./...
```
Then, in a Vineyard built from this branch, look at a session in which
background subagents were stopped with ctrl+x ctrl+k while they worked.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass, including the new
`TestLoad_SubagentsTheUserStoppedStopped`. The session shows those
subagents stopped (×) and drops them at the next prompt.

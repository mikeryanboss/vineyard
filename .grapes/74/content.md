## Goal
An agent that finds a problem in another issue's work can tell which Claude
Code session works on that issue and message it. Today its peer list
(`ListAgents`) shows only names like `test-e72254-71`, which say nothing about
the issue.

## Description
Two changes:

1. **Peer names.** Vineyard starts Claude Code with `--name <branch>`, so a
   session started for issue 71 appears to its peers as
   `71/fewer-issue-numbers`, the address `SendMessage` takes.
2. **`vineyard sessions`.** A command that prints the repository's sessions as
   JSON: ID, title, branch, status, linked issues, worktree, and the name of the
   Claude Code agent running in the worktree. Agents filter it with `jq`.

## Context
Verified on Claude Code 2.1.299:
- Claude Code registers each running session in
  `$CLAUDE_CONFIG_DIR/sessions/<pid>.json` (default `~/.claude/sessions`), with
  `pid`, `cwd`, `name`, and `nameSource`. Without `--name` the name is derived
  from the working directory's basename plus two hex characters
  (`nameSource = "derived"`); a Vineyard worktree's basename is its session ID.
  The format is undocumented.
- `claude --name "73/peer-name-test"` registers that name (`nameSource =
  "user"`); `ListAgents` lists it and `SendMessage` to it delivers.
- The name is written to the transcript (`custom-title`, `agent-name`) and kept
  by `claude --resume <id>` without `--name`.
- With a custom title, Claude Code writes no `ai-title`, so the recap tab
  (`internal/recap`, `renderRecap` heading in `internal/tui/app.go`) shows no
  title for such sessions.
- Launch: `Manager.Start` and `resumeCommand` in `internal/session/manager.go`
  pass a shell command to the multiplexer (`sh -c` in `internal/vterm/vterm.go`,
  tmux `new-session`), so the name is shell-quoted.
- Linked issues: `issuesOf` and `issueIDs` in `internal/tui/app.go` (recorded
  `Issue` plus `.grapes/<id>/` paths of the diff from `git.Base`).
- Commands: `run` in `main.go`; `debug` is the model of a read-only command.

Decisions:
- The name is the branch at start: unique within the repository
  (`freeBranch`), and for an issue session it begins with the issue number.
  Only worktree sessions get it; main-checkout sessions share a branch.
- A resume that finds the conversation keeps the name from its transcript; a
  fresh relaunch (no transcript) passes `--name` again.
- `vineyard sessions` reads agent names from Claude Code's registry, matching
  `cwd` to the worktree, so it also covers agents started without `--name`.
  Entries of processes that no longer run are skipped.

## Acceptance Criteria
- [x] A new Claude Code session in a worktree is launched with `--name <branch>`, shell-quoted; other programs and main-checkout sessions are launched unchanged.
- [x] A relaunch without a transcript passes `--name`; `--resume` does not.
- [x] `vineyard sessions` prints a JSON array with `id`, `title`, `branch`, `status`, `issues`, `worktree`, and `agent` for each session, and changes nothing.
- [x] `issues` matches what the TUI links: the recorded issue plus issues whose files the diff changes.
- [x] `agent` is the registry name of the live Claude Code process whose `cwd` is the worktree, or empty.
- [x] `help`, README, docs, and this repository's CLAUDE.md describe the command.

## Verify
Run from the worktree:
```bash
gofmt -l . && go vet ./... && go test ./...
go build -o .grapes/74/tmp/vineyard . && .grapes/74/tmp/vineyard sessions | jq .
```
Manual: start a session from a dev build (`scripts/drive`), then check
`ListAgents` from this session and `vineyard sessions`.

## Pass Criteria
Tests pass; `vineyard sessions` lists this session with `"issues": [74]` and
`"agent": "othersessions-16282e-3d"`; a session started from the dev build
appears in `ListAgents` under its branch name.

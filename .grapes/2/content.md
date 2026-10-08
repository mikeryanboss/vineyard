## Goal
Replace the TypeScript orchestration runtime with a terminal app in the style of
claude-squad: several coding agents side by side, each in its own git worktree
and tmux session, managed from one screen. This is the base for a deeper
Grapes integration that claude-squad cannot offer.

## Description
The project changed direction from autonomous Symphony-style orchestration
(issue #1) to an interactive agent manager. The new app reproduces
claude-squad's behaviour, not its code (claude-squad is AGPL-3.0; Vineyard is
MIT), on the stack and conventions of grapes: Go, Bubble Tea v2
(`charm.land/*`), lipgloss v2, TOML configuration, golden-file TUI tests, and
a `docs/` routing guide.

Improvements over claude-squad in this first version:
- A diff view with a file summary, per-file headers, old and new line numbers,
  Chroma syntax highlighting, changed-word highlighting within edited lines,
  word-boundary wrapping, and file-to-file jumps.
- Diffs include untracked files without writing to the agent's index.
- Agent sessions run on a private tmux socket, so Vineyard's key bindings never
  reach the user's own tmux server.
- Workspace trust prompts are never answered automatically unless auto-yes is on.
- Killing a session keeps its branch when the branch has commits.

## Context
- Removed: `src/`, `test/`, `package.json`, `package-lock.json`, `tsconfig.json`,
  `docs/initial-implementation.md` (the TypeScript runtime; kept in git history).
- Kept: `idea.md`, `.grapes/1`, `.agents/skills/`.
- New: `main.go`, `internal/{config,git,tmux,session,diff}`, `internal/tui/...`,
  `docs/{README,architecture,development}.md`.
- References: `reference-projects/claude-squad` (v1.0.20, behaviour) and
  `reference-projects/grapes` (v0.1.10, libraries and conventions), outside this repository.
- Requires git and tmux at runtime and Go 1.25+ to build.

## Acceptance Criteria
- [x] `vineyard` lists sessions with status (running, ready, loading, paused, stopped), branch, and diff size.
- [x] `n`/`N` create a session: a branch and worktree from HEAD and the agent in a detached tmux session; `N` types an initial prompt once the agent's screen is still.
- [x] The preview mirrors the selected agent's screen live, with scrollback.
- [x] `enter` attaches to the agent in full screen and `ctrl-q` returns to Vineyard; `t` does the same for a shell in the worktree.
- [x] The diff tab shows committed, uncommitted, and untracked changes against the session's base commit, with highlighting.
- [x] `c` commits, stops the agent, and removes the worktree, keeping the branch; `r` restores it; `s` commits and pushes; `D` kills, keeping branches with commits.
- [x] Sessions survive restarts; sessions whose tmux session died show as stopped and restart with `r`.
- [x] `go test ./...` and `go vet ./...` pass, and the suite runs in under 10 seconds.

## Verify
Run from the repository root:
```bash
go vet ./...
go test -count=1 ./...
```
Manual: in a scratch repository, run
`VINEYARD_HOME=/tmp/vy VINEYARD_TMUX_SOCKET=vy-dev vineyard -p <agent>` and
exercise each key above.

## Pass Criteria
`go vet` prints nothing; every package reports `ok`. In the manual run, each
acceptance criterion behaves as described and the `vineyard` tmux socket is
not touched.

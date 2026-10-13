## Goal
Add a Terminal tab to the pane, between Issue and Recap, that mirrors the
selected session's shell, so the user can watch a shell in the session's
worktree without attaching to it.

## Description
Today `t` starts the session's shell (`Manager.EnsureShell`, tmux session
`Session.ShellName()`) and attaches to it at once; nothing shows the shell
inside Vineyard. The pane's tabs are Preview, Diff, Issue, Recap
(`tab` constants in `internal/tui/app.go`).

Change:

- The tabs become Preview, Diff, Issue, Terminal, Recap.
- While the Terminal tab is shown, Vineyard starts the selected session's
  shell if it is not running, keeps it at the agent's size, and mirrors it on
  the preview tick, as the Preview tab mirrors the agent.
- `enter` on the Terminal tab attaches to the shell, as `t` does on any tab.
- With the pane focused, the Terminal tab scrolls the shell's scrollback as
  the Preview tab scrolls the agent's.
- A session without a worktree (paused, starting, busy) shows the Preview
  tab's placeholder instead. A stopped agent's worktree still has a shell.
- The shell is the session's, also when a subagent row is selected.

## Context
- `internal/tui/app.go`: `tab`, `tabNames`, `openNames`, `tabLabels`,
  `showTab`, `refreshShown`, `previewTickMsg`, `screenMsg`,
  `ScrollbackRequestMsg`, `scrollbackMsg`, `attachDoneMsg`, `handleKey`,
  `handleMouse`, `renderPane`.
- `internal/tui/backend.go`: `Backend` gains shell capture, history, and
  resize, as `LiveBackend` methods on `Session.ShellName()`.
- `internal/tui/common/keys.go`: tab help text.
- Docs: `README.md` keys, `docs/architecture.md`.

## Acceptance Criteria
- [x] The tab bar reads Preview, Diff, Issue, Terminal, Recap, and tab and
      shift+tab cycle through them in that order.
- [x] Showing the Terminal tab starts the shell and mirrors its screen.
- [x] `enter` on the Terminal tab attaches to the shell.
- [x] A paused session's Terminal tab shows the paused placeholder and starts
      no shell.
- [x] The Terminal tab works in a real terminal (tmux): the shell's output
      appears, and attaching then detaching returns to it.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run Vineyard in a real terminal, select a session, press tab three
times, run a command in the shell via `enter`, detach with ctrl-q, and check
the tab shows the command's output.

## Pass Criteria
Formatting, vet, and tests pass; the manual run shows the shell's output in
the Terminal tab.

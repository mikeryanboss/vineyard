## Goal

Let standalone grapes start or find a vineyard agent session for an issue. Grapes will run `vineyard --issue <id>` in its terminal (grapes issue planned in github.com/Mibokess/grapes); this issue adds the flag on vineyard's side.

## Description

Today only the issues screen inside vineyard can start a session for an issue: `a` sends `embedded.SessionsMsg`, and `showSessions` jumps to the issue's session, picks among several, or opens the new-session dialog filled in for the issue. Standalone grapes has no way in. Grapes deliberately runs no agents itself (grapes #52 removed its tmux sessions), so vineyard must accept the issue on its command line.

`vineyard --issue <id>` (short `-i`) launches the TUI and does what `a` does on that issue, as soon as the first window size arrives, since the dialogs are sized to the window.

## Context

- `main.go` `run` parses `-p` and `-y`; `runTUI` takes the lock, loads sessions and grapes (`loadGrapes`), and builds `tui.Options`.
- `internal/tui/app.go` `showSessions(issueID)` is the `a` behavior; dialogs take `min(70, m.width-4)`, so they need `m.width`.
- `tea.WindowSizeMsg` is handled in `Update` and forwarded to grapes via `updateGrapes`.
- Planned: `tui.Options.Issue`, a `startIssue` field on the model cleared on first use.
- An unknown issue or a repository without `.grapes` fails before the TUI starts. A running vineyard already fails with `session.ErrLocked`.

## Acceptance Criteria

- [x] `vineyard --issue <id>` and `-i <id>` open the new-session dialog for the issue when no session works on it, jump to the session when one does, and show the picker when several do.
- [x] `--issue` with an issue grapes does not know, or in a repository without `.grapes`, exits 1 with an error naming the problem, before the TUI starts.
- [x] Help and README document the flag.
- [x] `version` is bumped so merging releases a vineyard that grapes can call.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
go build -o .grapes/18/tmp/vineyard . && .grapes/18/tmp/vineyard --issue 9999; echo "exit $?"
```

Manual: in a tmux pane, run the built binary with `--issue 18` and capture the screen.

## Pass Criteria

`gofmt` prints nothing; vet and tests pass. The unknown issue prints `Error: no grapes issue #9999` and `exit 1`. The capture shows the new-session dialog titled for #18.

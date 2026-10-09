## Goal

Open grapes, the repository's issue tracker, as a full screen inside vineyard, so issues and agent sessions live in one program.

## Description

Grapes issue `Mibokess/grapes#52` adds a public package, `github.com/Mibokess/grapes/embedded`, whose `Model` is the grapes TUI made to run inside another Bubble Tea program. Vineyard loads it at startup for the repository's `.grapes` directory and shows it when the user presses `i`. Grapes then owns the whole screen, including its own header and status bar, like vineyard's config screen owns the body today. Grapes' quit key (`q`) returns to the sessions screen instead of ending vineyard.

## Context

Verified against `main` at `33f9d60`:

- `main.go:runTUI` finds the repository (`git.Repo.Root` is the main checkout), loads config, and builds `tui.NewModel(backend, sessions, opts)`.
- `internal/tui/app.go:Update` handles its own messages and passes the rest to an open dialog or the config screen (`app.go:706-716`). `ctrl+c` quits from anywhere (`handleKey`).
- `View` sets alt screen and cell-motion mouse mode, which grapes uses too.
- The grapes model runs its own file watcher and 5-second workspace poll as Bubble Tea commands. Their messages arrive in vineyard's `Update`, so vineyard must forward every message it does not handle, whether or not the issues screen is shown, or grapes stops refreshing.
- `i` is unbound in `common.ListKeyMap`.

Design:

- `main.go` calls `embedded.New(<repo.Root>/.grapes)` when that directory exists, and passes the model in `tui.Options`. A repository without `.grapes` has no issues screen; `i` then says so in the status bar. A load error is shown in the status bar the same way.
- Window-size and background-colour messages, and all unhandled messages, go to grapes. Keys and mouse events go to grapes only while its screen is shown. `ctrl+c` still quits vineyard.
- `embedded.CloseMsg` closes the issues screen.
- Vineyard ignores `go.work` and `go.work.sum`, which developers use to build against a local grapes checkout.

## Acceptance Criteria

- [x] `i` on the sessions screen shows the grapes board for the repository, filling the terminal.
- [x] Grapes keys (navigation, open issue, status picker) work while it is shown, and `q` returns to the sessions screen with the previous selection.
- [x] Session polling keeps running while grapes is shown, and grapes keeps refreshing while it is hidden.
- [x] Without `.grapes`, `i` shows a status message and nothing else changes.
- [x] `go.work` and `go.work.sum` are in `.gitignore`.
- [x] `go.mod` requires a published grapes version or commit, never a local `replace`.

## Verify

Run from the repository root:

```bash
gofmt -l . && go vet ./... && go test ./...
grep -n "replace" go.mod
```

Manual, in a real terminal (see `docs/development.md`): run vineyard in this repository, press `i`, open an issue, change its status with the picker, press `q` twice (detail, then board).

## Pass Criteria

Formatting, vet, and tests pass; `grep` prints nothing. The manual run shows the grapes board inside vineyard, the status change lands in `.grapes/<id>/meta.toml`, and the second `q` returns to the sessions screen without quitting vineyard. Screenshots of the board inside vineyard go in the PR.

# Development Guide

## Local Checks

```sh
go test ./...
go vet ./...
go build .
```

Some tests need `git` and `tmux`. The tmux tests use a throwaway socket and
skip themselves when tmux is not installed.

## Running a Development Build

Keep development runs away from your real sessions:

```sh
VINEYARD_HOME=/tmp/vy-home VINEYARD_TMUX_SOCKET=vy-dev go run . -p bash
```

`-p bash` makes every session a plain shell, which is enough to exercise
creation, preview, attach, diff, and lifecycle without starting real agents.

## TUI Testing

Grapes' three test styles are used here:

- Unit tests for data packages: `diff`, `git`, `session`, `config`, `tmux`.
- Interaction tests that send `tea.KeyPressMsg` and mouse messages to a model
  and assert on state or emitted messages. `testutil.Key("enter")` builds key presses.
- Golden tests that render a model and compare it, with ANSI codes stripped, to
  a file in the package's `testdata/`.

Regenerate golden files after an intentional rendering change, then review
every changed file:

```sh
go test ./internal/tui/... -update
go test ./internal/tui/...
```

Root-model tests (`internal/tui/app_test.go`) drive `tui.Model` with a fake
`Backend`. `send` applies a message and feeds the resulting commands' messages
back until things settle, dropping ticks so polling stays under the test's control.

## Common Change Paths

| Change | Production files | Tests |
| --- | --- | --- |
| New key or action | `tui/common/keys.go`, `tui/app.go` (`handleKey`) | `app_test.go` |
| New session operation | `session/manager.go`, `tui/backend.go`, `tui/app.go` | `session_test.go`, `app_test.go` |
| Support another agent's prompts | `session/screen.go` | `TestScreenDetection` |
| Diff appearance | `tui/diffview/`, colours in `tui/common/theme.go` | diffview golden tests |
| List appearance | `tui/list/list.go` | list golden tests |
| Config field | `config/config.go`, README | `config_test.go` |
| Persisted session field | `session/session.go` | `TestStore_RoundTrip`; keep old files loadable |

When a change moves a responsibility or invalidates an invariant in
[README.md](README.md) or [architecture.md](architecture.md), update the
document in the same change.

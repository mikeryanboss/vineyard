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

Vineyard keeps its data in the repository it runs in, so run development
builds in a throwaway repository, never in this checkout, and on their own tmux
socket:

```sh
go build -o /tmp/vy/vineyard .
git init -q /tmp/vy/demo && git -C /tmp/vy/demo commit -q --allow-empty -m init
cd /tmp/vy/demo && VINEYARD_TMUX_SOCKET=vy-dev /tmp/vy/vineyard -p bash
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

## Screenshots for Pull Requests

A reviewer cannot see golden files rendered, so show TUI changes as screenshots
of the running app. Capture them in a throwaway repo with the development
build above, using a tmux window of fixed size:

```sh
GOBIN=$PWD/.grapes/<id>/tmp/bin go install github.com/charmbracelet/freeze@latest
go build -o .grapes/<id>/tmp/vineyard .
tmux new-session -d -s shot -x 110 -y 30 -c <demo-repo> \
  "VINEYARD_TMUX_SOCKET=vy-shot <tmp>/vineyard -p bash"
tmux send-keys -t shot n          # drive the app like a user
tmux capture-pane -e -p -t shot > .grapes/<id>/tmp/shot.ansi
.grapes/<id>/tmp/bin/freeze .grapes/<id>/tmp/shot.ansi --language ansi \
  --theme github --background "#ffffff" \
  --window=false --border.radius 0 -o .grapes/<id>/tmp/shot.png
```

Vineyard is light until the terminal reports a dark background, and tmux does
not, so the capture is light text on `freeze`'s dark default canvas unless you
pass the white `--background`.

Look at each PNG before publishing it. The [pr skill](../.agents/skills/pr/SKILL.md)
publishes the images and embeds them in the PR.

## Releases

`var version` in `main.go` is the release version. A push to `main` that changes
`main.go` runs `.github/workflows/auto-tag.yml`, which creates `v<version>` if it
does not exist. The tag runs `.github/workflows/release.yml`, and GoReleaser
(`.goreleaser.yaml`) publishes static Linux and macOS archives for amd64 and
arm64, with the tag stamped into `main.version`. There is no Windows build:
sessions run in tmux, and WSL uses the Linux archive.

To release, bump `version` in a PR; merging it releases. Check the release
configuration locally without publishing:

```sh
goreleaser release --snapshot --clean --skip=publish   # writes dist/; delete it after
```

## Common Change Paths

| Change | Production files | Tests |
| --- | --- | --- |
| New key or action | `tui/common/keys.go`, `tui/app.go` (`handleKey`) | `app_test.go` |
| New session operation | `session/manager.go`, `tui/backend.go`, `tui/app.go` | `session_test.go`, `app_test.go` |
| Support another agent's prompts | `session/screen.go` | `TestScreenDetection` |
| Diff appearance | `tui/diffview/`, colours in `tui/common/theme.go` | diffview golden tests |
| List appearance | `tui/list/list.go` | list golden tests |
| Config field | `config/config.go`, `tui/settings/settings.go`, README | `config_test.go`, `settings_test.go` |
| Persisted session field | `session/session.go` | `TestStore_RoundTrip`; keep old files loadable |
| Issues screen, issue tab, or session–issue links | `tui/app.go` (`issuesOf`, `showSessions`, `showIssues`, `refreshIssue`) | `app_grapes_test.go` |
| What grapes exposes | grapes' `embedded/` package, then `go get` the release here | grapes' `embedded_test.go`, `app_grapes_test.go` |

### Changing grapes alongside vineyard

Grapes is a separate module, [Mibokess/grapes](https://github.com/Mibokess/grapes).
To build against a local grapes checkout before it is released, put a `go.work`
in the vineyard checkout; git ignores it:

```sh
go work init . /path/to/grapes
```

Commit `go.mod` only with a published grapes version or commit
(`go get github.com/Mibokess/grapes@<tag-or-sha>`), never with a `replace`.
Prefer a release: grapes squash-merges PRs, so a pinned PR branch commit never
lands in grapes' history.

When a change moves a responsibility or invalidates an invariant in
[README.md](README.md) or [architecture.md](architecture.md), update the
document in the same change.

If a row here is stale, fix it in the same change that made it stale.

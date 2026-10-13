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
builds in a throwaway repository, never in this checkout, and on their own
socket (`VINEYARD_SOCKET` names tmux's and Vineyard's server's):

```sh
go build -o /tmp/vy/vineyard .
git init -q /tmp/vy/demo && git -C /tmp/vy/demo commit -q --allow-empty -m init
cd /tmp/vy/demo && VINEYARD_SOCKET=vy-dev /tmp/vy/vineyard -p bash
```

`-p bash` makes every session a plain shell, which is enough to exercise
creation, preview, attach, diff, and lifecycle without starting real agents.

New sessions run on Vineyard's own server. To exercise tmux, set it in the
throwaway repository's configuration, or on one profile:

```sh
printf 'multiplexer = "tmux"\n' >> /tmp/vy/demo/.vineyard/config.toml
```

Vineyard's server outlives the development build, and a rebuilt Vineyard of another
`protocol` refuses to use it. `vineyard debug` prints its socket and process; the server
stops by itself once its agents end and no Vineyard runs, or end it with
`kill <process>`.

## Debugging

Find out what happened before reproducing it, and reproduce it below the TUI
before driving the TUI. Each step costs more context than the one before.

1. **`vineyard debug`**, in the repository, on the user's socket. It starts
   nothing and prints, in about 30 lines: whether Vineyard's server runs, its
   process and protocol, its terminals; each session's multiplexer, recorded
   status, and whether its terminal is alive; and the ends of two logs.
   - `<socket>.log`, Vineyard's server's: its start and stop, and each
     terminal's start and exit, with the exit status and the last lines of
     its screen. An agent that "crashes immediately" leaves its error here.
   - `.vineyard/vineyard.log`, the Vineyards': each start and exit, with the
     exit error, every error the status line showed, and the stack of a panic
     in `Update` or `View`. Each line names its process.
   A server of another `protocol`, left by an earlier build, is the usual
   cause of failures right after an update.
2. **A failing test against the real thing.** The session tests use fake
   multiplexers; the `vterm` tests start real servers (`serverSocket` in
   `client_test.go`). A test that drives the failing path gives a stack trace,
   runs in milliseconds, and stays as the regression test.
3. **The TUI, driven in tmux**, to confirm a fix in the real app or to
   reproduce what needs the TUI. `scripts/drive` runs a build in a detached
   tmux window and prints only the screen's non-blank lines:

   ```sh
   go build -o .grapes/<id>/tmp/vineyard .
   scripts/drive init .grapes/<id>/tmp/demo vineyard   # or tmux
   scripts/drive start .grapes/<id>/tmp/vineyard .grapes/<id>/tmp/demo -p bash
   scripts/drive keys n; scripts/drive type fix; scripts/drive keys Enter
   scripts/drive screen 2        # the screen, 2 seconds on
   scripts/drive exit            # Vineyard's stderr and exit status, once it quit
   (cd .grapes/<id>/tmp/demo && VINEYARD_SOCKET=vineyard-drive-dev ../vineyard debug)
   scripts/drive stop            # the window, and the development multiplexers
   ```

   It keeps the traps out of the way: its tmux server is not on the
   `VINEYARD_SOCKET` name, which names Vineyard's tmux socket too; the window
   outlives Vineyard, so its exit can be read; and `stop` ends the
   development server without `pkill -f` patterns that match the shell
   running them.

To reproduce a failure across versions, build the earlier commit in a
temporary worktree (`git worktree add --detach <dir> <commit>`) and run each
build in turn on the same repository.

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
scripts/drive start .grapes/<id>/tmp/vineyard <demo-repo> -p bash
scripts/drive keys n              # drive the app like a user
scripts/drive screen 1 -e > .grapes/<id>/tmp/shot.ansi
.grapes/<id>/tmp/bin/freeze .grapes/<id>/tmp/shot.ansi --language ansi \
  --theme github --background "#ffffff" \
  --window=false --border.radius 0 -o .grapes/<id>/tmp/shot.png
```

Vineyard is light until the terminal reports a dark background, and tmux does
not, so the capture is light text on `freeze`'s dark default canvas unless you
pass the white `--background`.

`freeze` ignores resets to the default colours (`\e[0m`, `\e[39m`, `\e[49m`):
default-colour text comes out faint and a selected row's background runs across
the pane. Replace the resets with explicit colours before rendering:

```sh
sed -e 's/\x1b\[0m/\x1b[0m\x1b[38;5;235m\x1b[48;5;231m/g' -e 's/\x1b\[39m/\x1b[38;5;235m/g' \
  -e 's/\x1b\[49m/\x1b[48;5;231m/g' -e 's/^/\x1b[38;5;235m\x1b[48;5;231m/' shot.ansi > shot.fixed.ansi
```

If the PNG's borders do not line up, `freeze` fell back to a proportional
font; pass a monospace one by family and file, such as
`--font.family "Adwaita Mono" --font.file /usr/share/fonts/adwaita-mono-fonts/AdwaitaMono-Regular.ttf`.
The file alone was not enough in one run.
tmux keeps the tabs Bubble Tea moves the cursor with, so expand those with
their column, skipping escape sequences, before rendering.

Look at each PNG before publishing it. The [pr skill](../.agents/skills/pr/SKILL.md)
publishes the images and embeds them in the PR.

## README Demo

`doc/demo.gif` is recorded with [VHS](https://github.com/charmbracelet/vhs) from
`doc/vhs/demo.tape`, which runs real Claude Code agents. Off camera, the tape
builds a throwaway repository at `/tmp/vineyard-demo` with
`doc/vhs/demo-repo.sh`, uses the `vineyard-demo` tmux socket, and starts agents
on two of its issues; it starts recording once both have changes. Re-record from
the repository root, with the `vineyard` under test first in `PATH`:

```sh
vhs doc/vhs/demo.tape
```

- Claude Code must trust `/tmp/vineyard-demo` beforehand: run `claude` there
  once and accept. Worktrees under it inherit the trust.
- Do not run the tape from inside a Claude Code session: agents started with its
  `CLAUDE*` environment variables exit at once. Unset them first.
- Vineyard sometimes leaves a prompt in Claude Code's input box unsent, or loses
  it. `doc/vhs/demo-submit.sh` presses Enter on unsent prompts; a lost one makes
  the tape time out, so run it again.
- The recording font must cover `⑂`, `⏵`, and `⎿`; VHS's default does not. If
  they render as boxes, install a fallback font that has them, such as
  JuliaMono.
- Watch the GIF before committing it; Claude Code's status line and summaries
  differ between takes.

## Releases

Every merge to `main` that changes the program is released; nobody bumps a
version. `.github/workflows/auto-tag.yml` runs on pushes to `main` that change Go
files other than tests and `testdata`, `go.mod`, `go.sum`, or the templates
embedded from `internal/config/templates/`. It tags the pushed commit with the
next patch version after the highest `v*` tag, v0.1.4 to v0.1.5, unless the
commit already has a `v*` tag. Then it calls `.github/workflows/release.yml`: a
tag pushed with a workflow's `GITHUB_TOKEN` starts no workflow by itself. Runs
take turns, so two cannot pick the same tag. When several merges wait, GitHub
keeps only the newest waiting run, and its release includes them all.

GoReleaser (`.goreleaser.yaml`) publishes static Linux and macOS archives for
amd64 and arm64, with the tag stamped into `main.version`. There is no Windows
build: sessions run in tmux, and WSL uses the Linux archive. Then the workflow
requests the tag from the Go module mirror, proxy.golang.org, so that
`go install …@latest` installs it within a minute; otherwise the mirror may take
30 minutes to offer it. `go install …@latest` shows the module version Go
stamps into the binary, which is the tag. Builds in
a checkout show `dev`: Go stamps them with the commit of the nearest `.git`
directory, which for a worktree inside the main checkout is the main
checkout's.

For a minor or major version, push its tag by hand, for example
`git tag v0.2.0 origin/main && git push origin v0.2.0`. The tag runs
`release.yml` directly, and the next automatic tag is v0.2.1. Check the release
configuration locally without publishing:

```sh
goreleaser release --snapshot --clean --skip=publish   # writes dist/; delete it after
```

## Common Change Paths

| Change | Production files | Tests |
| --- | --- | --- |
| New key or action | `config/keys.go` (`DefaultKeys`), `tui/common/keys.go` (`NewKeyMap`), `tui/app.go` (`handleKey`), README's key tables | `app_test.go`, `keys_test.go` |
| New session operation | `session/manager.go`, `tui/backend.go`, `tui/app.go` | `session_test.go`, `app_test.go` |
| Support another agent's prompts | `session/screen.go` | `TestScreenDetection` |
| Diff appearance | `tui/diffview/`, colours in `tui/common/theme.go` | diffview golden tests |
| List appearance | `tui/list/list.go` | list golden tests |
| Config field | `config/config.go`, `tui/settings/settings.go`, README | `config_test.go`, `settings_test.go` |
| Persisted session field | `session/session.go` | `TestStore_RoundTrip`; keep old files loadable |
| Recap tab or transcript format | `recap/recap.go`, `tui/app.go` (`recapCmd`, `refreshRecap`) | `recap_test.go`, `app_test.go` |
| Issues screen, issue tab, or session–issue links | `tui/app.go` (`issuesOf`, `showSessions`, `showIssues`, `refreshIssue`) | `app_grapes_test.go` |
| Issue session prompts, example templates | `prompt/prompt.go`, `config/templates/`, `tui/dialog/dialog.go` (`ForIssue`) | `prompt_test.go`, `dialog_test.go`, `app_grapes_test.go` |
| What a failure leaves for diagnosis | `vterm/daemon.go` (server log), `tui/app.go` (`setError`, `logPanic`), `runDebug` in `main.go` | `TestInfoAndLogDescribeTheServer`, `TestApp_LogsErrorsAndPanics`, `TestRun_DebugPrintsPaths` |
| Server calls (`vterm.Service`) | `vterm/daemon.go`, `vterm/client.go`; raise `protocol` | `client_test.go`, `TestCheckServer` |
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

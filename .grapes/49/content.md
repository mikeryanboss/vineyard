## Goal
Make `N` start a session from a chosen profile, and let a profile run in the
repository's main checkout instead of a new worktree, so a plain terminal (for
a dev server, a long build) can be a session of its own beside the agents.

## Description
Today `n` and `N` open the same dialog (`internal/tui/dialog/dialog.go`,
`NewSessionDialog`); `N` only adds a free-text prompt, which lacks the agent's
own input features (file completion, slash commands) and so is rarely worth
using. The agent picker appears in both, and only with more than one profile.

Change:

- `n` asks for a title and runs the default profile. It shows no picker.
- `N` asks for a profile and a title. It has no prompt field. The picker
  comes first and has the focus.
- The issue dialog (`a` on an issue) keeps its prompt and templates, and
  offers only profiles that make a worktree: an issue session works on a
  branch of its own, and a prompt typed into a shell would run as commands.
- `config.Profile` gains `main_checkout`. A profile with it set starts its
  session in the main checkout (`git.Repo.Root`): no worktree, no branch.
- The defaults offer two profiles: `claude`, and `terminal` (`$SHELL`, in the
  main checkout). A config file with its own `[[profiles]]` replaces them.
- `ResolvedProfiles` offers `default_program` as a command of its own when no
  profile has that name, as `Config.DefaultProgram`'s comment already says.
  Otherwise a config naming `default_program = "aider"` without profiles
  would start claude once the defaults hold profiles.
- The config screen shows where each profile runs and toggles it with `w`.

A main-checkout session (`Session.MainCheckout`, saved):

- Start launches the program in the main checkout; nothing to clean up on
  failure.
- Kill stops its tmux sessions (program and `t` shell) and nothing else.
- Check out (`c`) and push (`s`) refuse: they would commit everything in the
  main checkout. The manager refuses too, not only the TUI.
- Resume restarts the program in the main checkout.
- Locate gives it the main checkout's path; branch polling shows the main
  checkout's branch.
- Its diff is the main checkout's against its base, as for any session.

Out of scope, for later: choosing where a session runs in the dialog, and a
profile that attaches to an existing tmux session on the user's own server
(needs a session picker, and nested tmux shares Vineyard's prefix key).

## Context
- `internal/config/config.go`: `Profile`, `Defaults`, `ResolvedProfiles`.
- `internal/session/session.go`: `Session`.
- `internal/session/manager.go`: `NewOptions`, `New`, `Start`, `Pause`,
  `Resume`, `Kill`, `Push`, `Locate`.
- `internal/tui/dialog/dialog.go`: `NewSession`, `NewSessionDialog`, `ForIssue`.
- `internal/tui/app.go`: `createSession`, `handleKey` (`keys.New`,
  `keys.NewPrompt`, `keys.Kill`, `keys.Pause`, `keys.Push`), `templatesMsg`,
  status hints.
- `internal/tui/settings/settings.go`: profile rows.
- `internal/tui/common/keys.go`: `NewPrompt`.
- Docs: `README.md` keys and configuration, `docs/architecture.md` Sessions.

## Acceptance Criteria
- [x] `n` shows title only and starts the default profile.
- [x] `N` shows a profile picker and a title, no prompt, and starts the chosen profile.
- [x] The issue dialog offers only profiles without `main_checkout`.
- [x] Defaults hold `claude` and `terminal` (`$SHELL`, `main_checkout = true`); a config's own profiles replace them.
- [x] An unmatched `default_program` is offered first as its own command.
- [x] A main-checkout session starts in the main checkout without creating a worktree or branch.
- [x] Killing it stops its tmux sessions and leaves the main checkout and its branch alone.
- [x] Check out and push refuse it, in the manager and in the TUI.
- [x] Resume and Locate use the main checkout.
- [x] The config screen toggles `main_checkout` with `w` and shows it; saving writes it.
- [x] README and architecture docs describe the new `n`/`N` and main-checkout profiles.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then in a real terminal on a scratch repository: press `N`, pick `terminal`,
start it, see it in the list on the main checkout's branch, attach, run a
command, detach, kill it, and check `git worktree list` and `git branch`.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass. In the terminal, the terminal
session runs in the main checkout, and killing it leaves no worktree or branch
changes. Screenshots of the `N` dialog and the running terminal session are in
the PR.

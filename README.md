# vineyard

Vineyard is a terminal app for running several coding agents side by side —
Claude Code, Codex, Gemini, Aider, or any CLI — each in its own git worktree
and tmux session, from one screen.

It follows [claude-squad](https://github.com/smtg-ai/claude-squad)'s model, rebuilt on
Bubble Tea v2 with the conventions of [grapes](https://github.com/Mibokess/grapes),
which it will integrate with more deeply over time.

## Requirements

- git
- tmux
- Go 1.25 or newer, to build from source

## Install

```sh
go install github.com/mikeryanboss/vineyard@latest
```

## Use

Run `vineyard` inside a repository. Sessions keep running in tmux when Vineyard
exits, and the next launch picks them up again.

| Key | Action |
| --- | --- |
| `n` / `N` | New session / new session with an initial prompt |
| `enter` | Attach to the agent; `ctrl-q` returns to Vineyard |
| `t` | Open a shell in the session's worktree |
| `tab` | Switch between the live preview and the diff |
| `l` | Focus the pane to scroll it (`esc` returns to the list) |
| `s` | Commit everything and push the branch to origin |
| `c` | Check out: commit, stop the agent, remove the worktree, keep the branch |
| `r` | Resume a checked-out or stopped session |
| `D` | Kill: stop the agent and delete the worktree (branches with commits are kept) |
| `a` | Toggle auto-yes, which accepts the agent's permission prompts |
| `i` | Open the repository's [grapes](https://github.com/Mibokess/grapes) issues, at the selected session's issue |
| `C` | Open the config screen |
| `q` | Quit; sessions keep running |

In the diff pane, `]` and `[` jump between files, `ctrl+d`/`ctrl+u` page, and
`g`/`G` go to the top and bottom.

The issues screen is grapes itself, for repositories with a `.grapes` directory.
On an issue, `a` goes to the session working on it, offers a choice when several
do, and starts one when none does; that session's branch is `<id>/<title>`.
`q` returns to the sessions. A session works on an issue when it was started
from it or its branch changed the issue's files; the list tags it `#<id>`.

Flags: `-p <command>` launches a different agent in new sessions, and `-y` turns
on auto-yes for them. `vineyard debug` prints where everything is stored.

## Configuration

Each repository keeps its configuration in `.vineyard/config.toml` in its main
checkout, like grapes keeps `.grapes/config.toml`; commit it to share it. Press `C`
to edit it in Vineyard: the default agent, the branch prefix, auto-yes for new
sessions, the worktree directory, and the agent profiles. `ctrl+s` saves, and
the next new session uses the new settings; `esc` leaves without saving.

The file can also be edited by hand; saving from the screen drops its comments.

```toml
default_program = "claude"         # profile name, or a command
branch_prefix = "mboss/"           # defaults to your username
auto_yes = false
worktree_dir = ".vineyard/worktrees"  # relative to the repository, absolute, or ~/...

[[profiles]]
name = "claude"
program = "claude"

[[profiles]]
name = "codex"
program = "codex"
```

With more than one profile, the new-session dialog offers a choice of agent.

## How it works

Each session is its own branch and worktree, under `.vineyard/worktrees/` unless
`worktree_dir` says otherwise, with the agent running in a detached tmux session on Vineyard's own tmux server
(`tmux -L vineyard ls`). The preview mirrors the agent's screen with
`tmux capture-pane`; attaching hands the terminal to tmux. An agent whose
screen keeps changing is working; one whose screen is still is waiting for you.

Everything else Vineyard keeps about a repository, its session list and a lock
file, is in `.vineyard/` too. Vineyard writes `.vineyard/.gitignore` so that only
the configuration reaches git.

See [docs/README.md](docs/README.md) for the code map.

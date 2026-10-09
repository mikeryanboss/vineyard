# Vineyard

Run several coding agents in parallel from one terminal. Each agent gets its
own git worktree, branch, and tmux session, so agents never edit the same
checkout. It follows [claude-squad](https://github.com/smtg-ai/claude-squad)'s
model, rebuilt on Bubble Tea v2.

![Vineyard demo](doc/demo.gif)

- **Any agent.** Claude Code, Codex, Gemini, Aider, or any other CLI.
- **Live preview.** See each agent's screen, whether it is working or waiting
  for you, and the diff of its branch.
- **Issues.** Vineyard embeds the [grapes](https://github.com/Mibokess/grapes)
  issue tracker. Start an agent from an issue, and see which session works on
  which issue.
- **Persistent.** Agents keep running in tmux after you quit; the next launch
  picks them up.

## Install

Requires git 2.48 or newer and tmux.

Download a Linux or macOS build from the
[releases](https://github.com/mikeryanboss/vineyard/releases), or build with
Go 1.25 or newer:

```sh
go install github.com/mikeryanboss/vineyard@latest
```

## Usage

Run `vineyard` inside a git repository.

| Key | Action |
| --- | --- |
| `n` / `N` | New session / new session with a prompt |
| `enter` | Open the shown tab: attach to the agent from Preview, open the diff tool from Diff, the issue from Issue, the full recap from Recap |
| `o` | Attach to the agent from any tab; `ctrl-q` returns |
| `tab` / `shift+tab` | Switch the pane: preview, diff, issue, recap |
| `l` | Scroll the pane; `esc` returns to the list |
| `t` | Open a shell in the session's worktree |
| `s` | Commit all changes and push the branch |
| `c` | Check out: commit, stop the agent, remove the worktree, keep the branch |
| `r` | Resume a checked-out or stopped session |
| `D` | Kill the session; a branch with commits is kept |
| `a` | Toggle auto-yes, which accepts the agent's permission prompts |
| `i` | Open the grapes issues; on an issue, `a` goes to or starts its session |
| `C` | Edit the configuration |
| `q` | Quit; agents keep running |

Flags: `-p <command>` runs a different agent in new sessions, `-y` turns on
auto-yes for them, and `-i <id>` opens on grapes issue `<id>` as if you pressed
`a` there. `vineyard debug` prints where Vineyard stores its data.

## Configuration

Each repository keeps its settings in `.vineyard/config.toml`; commit it to
share them. Press `C` to edit it in Vineyard, or edit it by hand:

```toml
default_program = "claude"            # profile name or command
branch_prefix = "mboss/"              # defaults to your username
auto_yes = false
worktree_dir = ".vineyard/worktrees"  # relative to the repository, absolute, or ~/...
diff_command = "hunk diff {base} --watch"  # run by enter on the Diff tab

[[profiles]]
name = "claude"
program = "claude"

[[profiles]]
name = "codex"
program = "codex"
```

With more than one profile, the new-session dialog asks which agent to run.

`diff_command` runs through `sh` in the session's worktree, with `{base}`
replaced by the commit the session started from. The default is
[Hunk](https://github.com/modem-dev/hunk). When the command is empty or its
program is not installed, `enter` shows Vineyard's own diff full-screen instead.

## How it works

A session is a branch, a worktree under `worktree_dir`, and the agent running
in a detached tmux session on Vineyard's own tmux server (`tmux -L vineyard
ls`). The preview mirrors the agent's screen with `tmux capture-pane`. An agent
whose screen keeps changing is working; one whose screen is still is waiting
for you. Attaching hands the terminal to tmux.

Vineyard also keeps its session list and a lock file in `.vineyard/`, and writes
a `.vineyard/.gitignore` so that only the configuration reaches git.

Contributors: see [docs/README.md](docs/README.md).

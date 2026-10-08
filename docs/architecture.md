# Architecture

Stable responsibilities and data flow. Check the named code before relying on details.

## Sessions

A session (`internal/session/session.go`) is a title, a launch command, a
branch, the commit the branch started from, a worktree path, and a tmux session
name. Its `Status` is one of:

| Status | Meaning |
| --- | --- |
| `loading` | Worktree or tmux session being created |
| `running` | Agent's screen is changing: it is working |
| `ready` | Agent's screen is still: it waits for input |
| `paused` | Worktree removed, work committed to the branch |
| `stopped` | tmux session gone, worktree still on disk |

`session.Manager` performs lifecycle operations against git and a `Terminal`
(implemented by `*tmux.Client`):

- `New` names the branch from the branch prefix passed in by the TUI and the
  title. The TUI passes the prefix from its current config, so a prefix saved
  on the config screen applies to the next session.
- `Start` numbers the branch if the name is taken, branches from the HEAD of
  the checkout Vineyard runs in, adds the worktree, and launches the program
  sized to the preview pane. A failed launch removes the worktree and branch
  again.
- `Pause` commits all changes as a checkpoint, stops the agent and its shell,
  and removes the worktree so the branch can be checked out elsewhere.
- `Resume` re-adds the worktree from the branch when it is missing and restarts
  the program. For stopped sessions the worktree, and any uncommitted work in
  it, is reused.
- `Kill` stops everything and removes the worktree. The branch is deleted only
  when it has no commits beyond its base.
- `Push` commits and runs `git push -u origin <branch>`.
- `Restore` runs at startup: sessions whose tmux session vanished become stopped.

## Storage

```text
~/.vineyard/                                     (VINEYARD_HOME)
  config.toml
  projects/<name>-<hash>/
    sessions.json                                atomic rename on every write
    lock                                         flock held while the TUI runs
    worktrees/<session id>/
```

The project directory is keyed by the main checkout's path, found through git's
common directory, so starting Vineyard from any linked worktree shows the same
sessions. The lock stops two Vineyard processes from overwriting each other's state.

## tmux

All agent sessions live on one private tmux server (`tmux -L vineyard`). After
each session start, server options are applied: scrollback, mouse, a status
hint, and `ctrl-q` bound to detach. They are reapplied every time because the
server forgets them when its last session ends.

- **Preview**: `capture-pane -p -e` of the visible screen, with ANSI colours.
  Detached windows are resized to the preview's content size, so the agent lays
  itself out for that space.
- **Attach**: `tea.ExecProcess` runs `tmux attach-session` and Bubble Tea gives up
  the terminal until it exits. The window is first switched back to following
  the client's size, and `TMUX` is removed from the environment so attaching
  works when Vineyard runs inside tmux. After detaching, the preview size is
  reapplied.
- **Prompts**: initial prompts are sent as a bracketed paste (`set-buffer` and
  `paste-buffer -p`) followed by Enter, so multi-line text is submitted once.

The TUI reads from standard input and output when they are terminals and
falls back to `tea.OpenTTY` otherwise. Attached tmux clients inherit these
files, and tmux rejects a descriptor opened through `/dev/tty`.

## Status Detection

There is no structured channel from the agent, so status is inferred from the
screen, as claude-squad does. Every second each live session's screen is
captured and fingerprinted. A changed fingerprint means running; two equal
fingerprints in a row mean ready. On top of that:

- A ready session with a pending prompt receives it, and the prompt is cleared
  and saved so it is never sent twice.
- If the screen shows a workspace trust prompt, the pending prompt waits. With
  auto-yes the trust prompt is accepted; otherwise the user is told to attach.
- With auto-yes, known permission prompts (`session.AwaitingPermission`, per
  program) are answered with Enter.

## Diffs

`git.Diff` compares the session's base commit with its working tree: committed
work, uncommitted edits, and untracked files that are not ignored (up to 200
files of up to 1 MiB each). Untracked files are diffed with `git diff --no-index`
against the null device instead of `git add -N`, which would write to the index
the agent is using. `git.DiffStat` gives line counts only, for sessions not on screen.

`diff.Parse` turns the output into files, hunks, and numbered lines. It reads
hunk bodies by the line counts in their headers, so a removed line starting
with `--` is never mistaken for a file header. `diff.Pairs` and
`diff.ChangedRanges` match removed lines with the added lines that replaced
them and find the changed words.

`diffview` renders a file summary, then each file with a header, old and new
line numbers, Chroma syntax highlighting (single lines tokenised alone, and
switched off above 4000 lines), tinted backgrounds for added and removed lines,
stronger tints on changed words, and wrapping at word boundaries. The rendered
lines are cached and rebuilt only when the diff text, width, or theme changes.

## TUI Message Flow

```text
key/mouse/tick
  -> tui.Model.Update
  -> child view Update (list, preview, diffview, dialog) for local behaviour
  -> common message upward (NewSessionMsg, ConfirmedMsg, LeavePaneMsg, ...)
  -> root starts a command through Backend
  -> result message (startedMsg, lifecycleMsg, screenMsg, diffMsg, ...)
  -> root updates sessions, saves, refreshes views
```

The root model owns sessions, side effects, focus, and layout. Child views own
rendering and local navigation; they import `tui/common` and plain data types
(`config`, `session`), never the backend. `tui.Backend` is the single seam to
the outside world, including saving the config; tests replace it with a fake.

Polling loops schedule their next tick independently of their work and skip a
round while the previous one is still in flight, so slow git or tmux calls
never pile up.

## Package Dependency Direction

```text
main -> config, git, session, tmux, tui
tui -> session, git, config, diff (via diffview), tui/*
tui/* -> tui/common, and data packages
session -> git
git, tmux, diff, config -> standard library and format libraries
```

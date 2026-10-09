# Architecture

Stable responsibilities and data flow. Check the named code before relying on details.

## Sessions

A session (`internal/session/session.go`) is a title, a launch command, a
branch, the commit the branch started from, a worktree path, and a tmux session
name. Git owns the worktree path and, while a worktree exists, the branch:
agents switch branches inside their worktree, and the repository may move. The
path is never saved, and the branch is read back from git before it is used.
Its `Status` is one of:

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
  the checkout Vineyard runs in, adds the worktree with relative links
  (`git worktree add --relative-paths`, git 2.48+), and launches the program
  sized to the preview pane. A failed launch removes the worktree and branch
  again.
- `Pause` records the checked-out branch, commits all changes as a checkpoint,
  stops the agent and its shell, and removes the worktree so the branch can be
  checked out elsewhere. The recorded branch is what `Resume` checks out.
- `Resume` re-adds the worktree from the branch when it is missing and restarts
  the program. For stopped sessions the worktree, and any uncommitted work in
  it, is reused.
- `Kill` stops everything and removes the worktree. The checked-out branch is
  deleted only when it has no commits beyond its base.
- `Push` commits and runs `git push -u origin <branch>` for the checked-out
  branch.
- `Pause`, `Kill`, and `Push` refuse a worktree with a detached HEAD.
- `Restore` runs at startup. It finds each session's worktree in
  `git worktree list` by directory name, which is the session ID, and takes its
  branch from there. A worktree git lost track of because something moved is
  reconnected with `git worktree repair --relative-paths` when it is at
  `<worktree_dir>/<id>`; that is also where a session without a worktree gets
  one on `Resume`. Sessions whose tmux session vanished become stopped.

The TUI's git poll also reads each session's branch, so the list follows branch
switches within one poll.

A session started from a grapes issue records it in `Issue`, and `New` names its
branch `<issue>/<slug>` instead of using the prefix.

## Grapes

The issues screen is the grapes TUI itself, from grapes' public `embedded`
package. `main.go` loads it for the main checkout's `.grapes` directory; grapes
finds the copies in every worktree from there. Without that directory,
`Options.GrapesErr` says why, and the issues key reports it.

The root model owns one `embedded.Model` for the life of the program:

- Grapes runs while hidden. Its file watcher and reloads are Bubble Tea
  commands, so their results arrive in vineyard's `Update`; every message the
  root does not handle is forwarded to grapes. Dropping them stops grapes from
  reloading.
- Keys and mouse events go to grapes only while `issuesOpen`; `ctrl+c` still
  quits vineyard. Grapes then draws the whole screen, dialogs aside.
- Grapes sends two messages for vineyard instead of acting itself:
  `embedded.CloseMsg` when the user presses quit, and `embedded.SessionsMsg`
  when the user asks for an issue's sessions.

`vineyard --issue <id>` sets `Options.Issue`, and the first window size runs
`showSessions` for it, as `SessionsMsg` would; the dialogs need that width.
Standalone grapes runs this command for its sessions key, so sessions can be
started from grapes without vineyard open.

`issuesOf` links sessions to issues: the recorded `Issue`, plus
`embedded.Model.TouchedIssues(WorktreePath)`, the issues the session's branch
changed. Recorded issues survive pausing, which removes the worktree.

The pane's issue tab shows those issues, rendered by
`embedded.Model.RenderIssue` at the pane's width. Given the session's worktree,
grapes shows that worktree's copy of each issue, as the agent left it.
Rendering is markdown work, so `refreshIssue` runs only while the tab is shown,
and only when its inputs change: the selection, the tab, or a grapes update,
which also covers resizes and theme changes, since both reach grapes through
`updateGrapes`.

## Recaps

The pane's recap tab shows what Claude Code recorded about the selected
session, read by `internal/recap` from Claude Code's transcripts:
`$CLAUDE_CONFIG_DIR/projects` (default `~/.claude/projects`), in the directory
named after the session's worktree path with every non-alphanumeric character
replaced by `-`. Claude Code writes a recap (`"subtype":"away_summary"`) when
the user returns after being away; the tab lists them newest first under the
`ai-title`, or shows the `last-prompt` when there is none yet. The format is
undocumented and was read off Claude Code 2.1.295; a malformed line is shown as
an error, not skipped.

`recapCmd` reads the transcripts only while the tab is shown, and only for
sessions whose program is `claude`: on switching to the tab, on selecting a
session, and on each diff tick. The latest result is kept in `recapShown`, so
`refreshRecap` can re-render it on resize and theme changes.

## Storage

```text
<main checkout>/.vineyard/
  config.toml                  committed; the repository's whole configuration
  .gitignore                   written when missing: ignores all but config.toml
  sessions.json                atomic rename on every write
  lock                         flock held while the TUI runs
  worktrees/<session id>/      the default worktree_dir
```

The directory is in the main checkout, found through git's common directory, so
starting Vineyard from any linked worktree shows the same sessions. The lock
stops two Vineyard processes from overwriting each other's state.

`worktree_dir` is resolved against the main checkout
(`config.ResolveWorktreeDir`) when a session is created and at startup.
`sessions.json` holds no paths: git records where each worktree is, so changing
the setting never moves existing worktrees, and moving the repository keeps
them. Relative links survive moving worktrees inside the repository, or moving
the repository and an outside `worktree_dir` together. When only the
repository moves, `Restore` repairs worktrees found at `<worktree_dir>/<id>`.
tmux session names use `config.ProjectName`, the checkout's directory name plus
a hash of its path, because one tmux server serves every repository.

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

`diffview` renders a file summary, then each file as a card: a heavy rule, a
header bar with a fold marker, a rule, then old and new line numbers, Chroma syntax highlighting (single lines tokenised alone, and
switched off above 4000 lines), tinted backgrounds for added and removed lines,
stronger tints on changed words, and wrapping at word boundaries. The rendered
lines are cached and rebuilt only when the diff text, width, theme, or fold
state changes. Files fold individually (`enter`) or all at once (`c`, `e`); fold
state is keyed by path and survives diff refreshes.

## TUI Message Flow

```text
key/mouse/tick
  -> tui.Model.Update
  -> child view Update (list, preview, diffview, textview, dialog) for local behaviour
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
main -> config, git, session, tmux, tui, grapes/embedded
tui -> session, git, config, diff (via diffview), tui/*, grapes/embedded
tui/* -> tui/common, and data packages
session -> git
git, tmux, diff, config -> standard library and format libraries
```

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
  it, is reused. Claude Code restarts with `--resume <id>` of the conversation
  its agent was last seen in, saved in `Conversation`: on each status poll
  the leading Vineyard saves the ID of the conversation the subagent rows
  are read from (Subagents below). Without a saved ID, or once its
  transcript is gone, the plain program starts: `claude --resume` exits at
  once for a conversation it cannot find.
- `Kill` stops everything and removes the worktree. The checked-out branch is
  deleted only when it has no commits beyond its base. A detached HEAD has no
  branch, so `Kill` deletes none, not even the one saved in the session. When
  HEAD has commits that no branch, remote-tracking branch, or tag contains,
  removing the worktree would lose them, so `Kill` refuses with a
  `LostCommitsError` listing them unless told to discard them. The TUI then
  shows the commits and asks again.
- `Push` commits and runs `git push -u origin <branch>` for the checked-out
  branch.
- `Pause` and `Push` refuse a worktree with a detached HEAD.
- `Restore` runs at startup. It finds each session's worktree in
  `git worktree list` by directory name, which is the session ID, and takes its
  branch from there. A worktree git lost track of because something moved is
  reconnected with `git worktree repair --relative-paths` when it is at
  `<worktree_dir>/<id>`; that is also where a session without a worktree gets
  one on `Resume`. Sessions whose tmux session vanished become stopped.

The TUI's git poll also reads each session's branch, so the list follows branch
switches within one poll.

A session started from a grapes issue records it in `Issue`, and `New` names its
branch `<issue>/<slug>` instead of using the prefix. `Start` refuses an issue
that is not committed at the checkout's HEAD: the worktree starts there, and
the prompt tells the agent to read the issue in it.

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

`showSessions` with no session on the issue reads the prompt templates in a
command (`Backend.Templates`: `.vineyard/templates/*.md` plus `[[templates]]`
in the configuration), and `templatesMsg` opens the new-session dialog.
`promptData` describes the issue from `embedded.Model.Issue`: its labels,
parent, sub-issues with the branch of a session on each (by `issuesOf`), and
blockers not yet done. The dialog renders the chosen template with
`prompt.Render`, and again whenever the template or the Build sub-issues
checkbox changes. `config.Prepare` writes the example templates once, when the
templates directory is missing; after that they are the repository's.

`vineyard --issue <id>` sets `Options.Issue`, and the first window size runs
`showSessions` for it, as `SessionsMsg` would; the dialogs need that width.
Standalone grapes runs this command for its sessions key, so sessions can be
started from grapes without vineyard open.

`issuesOf` links sessions to issues: the recorded `Issue`, plus every
`.grapes/<id>/` the session's diff changes (see Diffs), as the git poll last saw
it in `git.Stat.Paths`. Grapes' own `embedded.Model.TouchedIssues` is not used:
it compares against the default branch's tip, so it drops a branch's issues
once the branch merges.
Recorded issues survive pausing, which removes the worktree.

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

## Subagents

Claude Code runs subagents inside its own process, so they have no terminal
of their own. It lists the running ones below its prompt, and opens one's
view in place of the main conversation from that list. It also keeps a
transcript of each: `<projects>/<dir>/<conversation>/subagents/agent-<id>.jsonl`,
next to `agent-<id>.meta.json` with its type and description, where `<dir>` is
named after the working directory (`recap.Dir`) and `<conversation>.jsonl` is
the conversation's own transcript. None of this is documented; it was read
off Claude Code 2.1.296. `internal/subagent/testdata` holds screens it drew.

The rows come from the transcripts. On each status poll, `subagent.Reader`
reads what was appended to the transcripts of each Claude session's current
conversation, the one written to last, picking up where the last poll
stopped: a conversation's transcript grows to tens of megabytes. From them it
knows each subagent's description, prompt, report, the files it edited, the
subagents it started, and its status:

- **Done** or **stopped** once a task notification for it, which Claude Code
  queues in the conversation when a subagent stops, is newer than its
  transcript's last message. `completed` is done; anything else, such as a
  failure or a subagent the user stopped, is stopped.
- **Stopped** too, at its last message, when its own transcript has not been
  written to since the session's Claude Code started (tmux's
  `session_created`, since the tmux session runs Claude Code itself).
  Subagents run inside Claude Code's process and end with it unreported, as
  when the session is paused. The conversation's transcript cannot tell: a
  Claude Code that resumes the conversation touches it as it starts. One
  restarted without a conversation to resume writes no transcript until its
  first prompt, so until then the old conversation is the newest.
- **Running** otherwise, also again after its caller resumed it with a
  message, which appends to the same transcript.

Claude Code drops a finished subagent from its list at once. Vineyard keeps
it, marked done, until the user sends the conversation a new prompt (a
transcript line from a `human` origin): `Conversation.Kept`. A row shows the
subagent's status icon (◑ running, ● done, × stopped) and description, under
the subagent that started it.

A session's subagents start collapsed into one line counting them by status,
`▸ 3 subagents: 2 running, 1 done`, so that rows appearing as subagents start
do not push the list down. Expanded, the line reads `▾` above the rows, and a
click on it expands or collapses the session. `→` (`l`) expands the selected
session into a row per subagent, and `←` (`h`) collapses it. A session stays
expanded until collapsed. Where there is nothing to expand, `→` gives the
pane focus for scrolling, as `l` always did. The selection follows the
subagent by ID as rows come and go.

The status poll still parses the screen of every Claude session
(`subagent.Parse`), because opening a subagent needs its position in Claude
Code's list. `subagent.Match` reads that list in terms of the transcripts:
Claude Code lists the running subagents the conversation started, and those
that stopped in the last 30 seconds, in the order they started; the one whose
view it shows even when that one finished, titling the view with its
description; and under that one, the subagents it started that run. Vineyard
highlights the subagent in view, and opens one only when exactly one reading
fits the list. A finished subagent that Claude Code no longer lists cannot be
opened: its caller can resume it with a message.

Claude Code draws its agent list last, so a poll that catches it redrawing,
as right after every detach, sees fewer subagents than there are, often none.
`seeSubagents` believes a screen showing fewer only when the next poll
agrees, so the subagent in view keeps its highlight and stays openable
through a redraw; the rows, read from the transcripts, do not depend on it.
Pausing or resuming a session restarts Claude Code and forgets its list at
once.

Selecting a subagent's row shows the subagent in every tab, as selecting a
session shows the session (`shownAgent` next to `shownID`):

- **Preview:** its transcript (`Reader.Events`), condensed the way Claude
  Code shows a conversation: messages, each tool call with a one-line summary,
  and the first three lines of its result. It follows new lines while
  scrolled to the bottom. A subagent has no screen of its own, and showing its
  view would mean typing into the session.
- **Diff:** the diff of the git worktree holding the directory it last worked
  in (`Agent.Dir`: where it last edited a file, else where it last `cd`-ed
  to, else its working directory), from where that worktree's branch forks
  from the session's branch or from the mainline, whichever is later
  (`git.StackedBase`). Subagents often create their branches from the
  session's, which holds commits they did not make, such as the issues the
  session opened for them. Against the session's branch, a merge commit takes
  the subagent's branch in as one on the mainline does (Diffs). In its
  session's worktree, the session's diff. `enter` runs the diff tool in that
  worktree.
- **Issue:** for a subagent in a worktree of its own, the issues of that
  diff, as for sessions; in its session's worktree, the issues whose files it
  edited (`shownIssues`).
- **Recap:** its description, type, status, report, and prompt.

Selecting a row types nothing. `enter` on the preview tab, or `o`, runs `Open`
and then attaches:

1. Refuse when the screen shows a permission or trust question, or a list of
   a different length than the one shown, which means subagents finished or
   started.
2. Press ↓ until the list has focus (`❯` on a row). In a multi-line draft each
   ↓ first moves the cursor down a line, which does not show on screen, so
   each press waits up to half a second.
3. Move the selection to the subagent, press Enter, and wait for `●` on it.
   When the subagent shown before had finished, Claude Code drops it, and
   those after it move up a place.
4. Press Esc so the prompt has focus: typing then messages the subagent.

Each step checks the screen before the next, and a failure is shown instead of
attaching. Esc is pressed only while the screen shows the list focused:
anywhere else it interrupts Claude Code, and leaving the list focused would let
a typed `x` stop a subagent. Claude Code shares the prompt's draft between the
views, so a draft survives opening a subagent.

## Storage

```text
<main checkout>/.vineyard/
  config.toml                  committed; the repository's whole configuration
  .gitignore                   written when missing: ignores all but config.toml
  sessions.json                atomic rename on every write
  sessions.lock                flock held for each read-change-write of sessions.json
  lock                         flock held by the Vineyard that leads
  worktrees/<session id>/      the default worktree_dir
```

The directory is in the main checkout, found through git's common directory, so
starting Vineyard from any linked worktree shows the same sessions.

## Several Vineyards

Any number of Vineyard processes may run on one repository, for example one per
terminal window. Three rules keep them consistent:

- **Writes change one session.** `Store.Add`, `Replace`, and `Remove` each take
  `sessions.lock`, read `sessions.json`, change one session, and write it back,
  so no process overwrites another's sessions. `Replace` of a session that is
  no longer saved does nothing: a process still showing a session another one
  killed must not bring it back. Startup's `Restore` runs under the same lock
  (`Store.Update`).
- **Each process reloads.** Every status tick reads `sessions.json` and adopts
  it (`adopt` in `internal/tui/app.go`): sessions saved elsewhere appear,
  sessions removed elsewhere go, saved fields such as auto-yes take effect. A
  session this process is starting or operating on keeps its local state. While
  a session is active both locally and as saved, the local running or ready
  state wins, because this process's own screen polls decide it. A reload
  starts only with no write of this process pending and is dropped if one was
  issued while it ran, so it never undoes this process's own change, such as a
  cleared pending prompt.
- **One process leads.** Every process polls every screen, so only the holder
  of `lock` (`session.Leader`) types into agents on its own: it delivers
  pending prompts and answers auto-yes prompts. The others try to take the lock
  on every status poll, so one of them leads within a second of the leader
  exiting. Opening a subagent is typing the user asks for, so every process
  does it.

Known limits: two processes showing the same session at different pane sizes
each resize its tmux window when their own size changes, so the other's
preview may be laid out for the wrong width until then. Saving the config
screen writes the whole `config.toml`; the last process to save wins.

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

Only the leading Vineyard (see Several Vineyards) acts on the following:

- A ready session with a pending prompt receives it, and the prompt is cleared
  and saved so it is never sent twice.
- If the screen shows a workspace trust prompt, the pending prompt waits. With
  auto-yes the trust prompt is accepted; otherwise the user is told to attach.
- With auto-yes, known permission prompts (`session.AwaitingPermission`, per
  program) are answered with Enter.

## Diffs

`git.Diff` compares the session's base with its working tree: committed
work, uncommitted edits, and untracked files that are not ignored (up to 200
files of up to 1 MiB each). Untracked files are diffed with `git diff --no-index`
against the null device instead of `git add -N`, which would write to the index
the agent is using. `git.DiffStat` gives line counts and changed paths only, for
sessions not on screen.

The base is where the branch's own work begins, found by `git.Base` on every
poll, so the diff leaves out what the branch took in from the mainline,
`origin/HEAD`, by rebasing or merging:

- Before the branch merges, the base is `git merge-base origin/HEAD HEAD`.
- Once a merge commit on the mainline's first-parent history brings it in
  (`git rev-list --first-parent --ancestry-path HEAD..origin/HEAD`, oldest
  first), the base is the merge-base of that merge's first parent and `HEAD`:
  the mainline as the merge found it. A rebase before merging does not change it.
- A fast-forward leaves no merge commit, so the branch's commits cannot be told
  from the mainline's, and the base is `HEAD`: only uncommitted work shows.
- Without `origin/HEAD` (no remote, or one added without
  `git remote set-head origin --auto`), the base is `BaseCommit`, the commit the
  session started from.

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

`enter` on the Diff tab hands the terminal to `diff_command`, run by `sh` in the
worktree with `{base}` replaced by the session's base (see Diffs), through `tea.ExecProcess`
as attach does. The backend looks up its first word on `PATH` in a command, not
in `Update`. When the command is empty or that program is missing, the pane is
zoomed instead: it fills the body in place of the list until `esc` or a tab
switch. Zooming leaves agents' tmux windows at the split-pane size
(`agentSize`), so agents never reflow. The Recap tab zooms the same way.

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

`handleMouse` routes clicks and the wheel by where they land. A dialog or the
config screen's picker gets them relative to its box, which `centerOffset`
places where `overlayCenter` draws it; the config screen, the list, and the
pane's content get them relative to their own top-left corner; the header and
the status bar get none. Each child maps a position to what its `View` draws:
the list's entry heights, the tab labels (`tabLabels`), the diff's
`fileStarts`, a dialog's content lines. Mouse release and motion events are
not handled, and reach grapes like any other message the root leaves alone.

Bubble Tea reports single clicks only. `doubleClick` pairs a click on the list
or pane with the one before it when both land within a column on the same line
within 500 ms (`lastClick`; tests replace `now`). The first click of a pair
acts as a single click; the second runs `enter` through `handleKey`, or `o` on
a subagent row, so a double click opens a subagent on every tab. Where the
first click already acted, the second is dropped: on a subagent count line, and
on a diff file's header or summary line (`clickAt.acted`, from
`diffview.Clickable`), since a jump scrolls another line under the pointer. Clicks on a dialog, the issues screen, or the config screen clear
`lastClick`, so the click that closes a dialog never pairs with a click on the
screen beneath.

The preview's scroll mode shows tmux's scrollback. A program on the alternate
screen, such as Claude Code with `"tui": "fullscreen"`, has none; when the
history is no longer than the screen, the preview stays live and the status
bar says so.

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
tui -> session, subagent, git, config, diff (via diffview), tui/*, grapes/embedded
tui/* -> tui/common, and data packages
subagent -> session
session -> git
git, tmux, diff, config -> standard library and format libraries
```

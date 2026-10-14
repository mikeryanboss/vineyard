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

`session.Manager` performs lifecycle operations against git and each session's `Terminal`
(implemented by `*tmux.Client`, or `*vterm.Client` for Vineyard's own server;
see Multiplexers per Session):

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

A session whose profile sets `main_checkout`, such as the built-in `terminal`,
records `MainCheckout` and runs in the main checkout, which is not its own:
`New` gives it no branch, `Start` only launches the program there, `Kill`
only stops its tmux sessions, `Pause` and `Push` refuse it, since they would
commit everyone's work there, and `Locate` gives it the main checkout's path.
Its branch and diff are the main checkout's.

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
- Keys and mouse events go to grapes only while `issuesOpen`; the global quit
  key, `ctrl+c`, still quits vineyard, and the global back key, `ctrl+q`,
  returns to the session list, as it does from every screen. Grapes then draws
  the whole screen, dialogs aside.
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

The list row, the issue tab, its label, and the issue picker show a session's
main issues (`mainIssues`): those whose parent is not linked too. A planner
that writes an issue and its sub-issues shows the one, whose rendering lists
the rest; a session on a sub-issue alone shows the sub-issue. Only the direct
parent counts, since grapes loads parent cycles. Matching sessions to an issue
(`showSessions`, `promptData`) uses every linked issue. The row and the label
name at most three; with more, they name two and count the rest
(`common.ClipIssues`), as in `#11 #12 +3`. The issue picker, like the picker
of an issue's sessions, is a `dialog.Pick` no taller than the screen above the
status bar: choices that do not fit scroll with the cursor, and the title shows
its position, as in `3/20`.

The pane's issue tab shows those issues, rendered by
`embedded.Model.RenderIssue` at the pane's width. Given the session's worktree,
grapes shows that worktree's copy of each issue, as the agent left it.
Rendering is markdown work, so `refreshIssue` runs only while the tab is shown,
and only when its inputs change: the selection, the tab, or a grapes update,
which also covers resizes and theme changes, since both reach grapes through
`updateGrapes`.

## Recaps

The pane's recap tab shows the selected session's conversation as Claude Code
recorded it, read by `internal/recap` from Claude Code's transcripts:
`$CLAUDE_CONFIG_DIR/projects` (default `~/.claude/projects`), in the directory
named after the session's worktree path with every non-alphanumeric character
replaced by `-`. Under the `ai-title`, it lists every message the user typed
(`"type":"user"` with `"origin":{"kind":"human"}`; tool results, slash commands,
and text Claude Code adds have no such origin) and every recap
(`"subtype":"away_summary"`, written when the user returns after being away),
oldest first across all of the directory's transcripts. It opens at the newest
entry and follows new ones while scrolled to the bottom. The format is
undocumented and was read off Claude Code 2.1.280 to 2.1.296; a malformed line
is shown as an error, not skipped.

`recapCmd` reads the transcripts only while the tab is shown, and only for
sessions whose program is `claude`: on switching to the tab, on selecting a
session, and on each diff tick. The latest result is kept in `recapShown`, so
`refreshRecap` can re-render it on resize and theme changes.

## Terminal Tab

The pane's terminal tab mirrors the selected session's shell: the terminal
named `Session.ShellName()` that `t` attaches to, running in the session's
worktree. While the tab is shown, `shellCmd` starts the shell if it is not
running (`EnsureShell`), keeps it at `agentSize` as the agent is kept, and
captures it, on selecting a session, on switching to the tab, and on each
preview tick in place of the agent's capture. A shell that exits is started
again on the next tick. A session without a worktree (paused, starting, busy)
shows the preview's placeholder instead. The shell is the session's, also
when a subagent row is selected. Kill and check out stop it with the agent.

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
  queues in the conversation when a subagent stops, is newer than its last
  message. `completed` is done; anything else, such as a failure or a
  subagent stopped with `TaskStop`, is stopped. Its last message is the last
  it wrote or was sent; tool results and `[Request interrupted by user…]`
  markers do not count, since Claude Code writes them as it stops a
  subagent, after it reports the stop.
- **Stopped** too, at that line, when the conversation records
  `agents_killed` (drawn as "All background agents stopped") no earlier than
  its last message. Claude Code writes that line, and no task notification,
  when it stops all background agents, as on ctrl+x ctrl+k.
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
each session start, server options are applied: scrollback and mouse. They
are reapplied every time because the server forgets them when its last
session ends.

- **Preview**: `capture-pane -p -e` of the visible screen, with ANSI colours.
  Detached windows are resized to the preview's content size, so the agent lays
  itself out for that space.
- **Attach**: `tea.ExecProcess` runs `tmux attach-session` and Bubble Tea gives up
  the terminal until it exits. The window is first switched back to following
  the client's size, and `TMUX` is removed from the environment so attaching
  works when Vineyard runs inside tmux. The same tmux command binds the detach
  keys in a key table named after them (`vineyard-detach-C-q`), sets it as the
  session's `key-table` with a status hint naming the keys, then attaches.
  tmux looks a key up in the session's table first and falls back to the root
  table, so other keys reach the program. Binding at attach time, per
  session, lets Vineyards with different detach keys share the server. After
  detaching, the preview size is reapplied.
- **Status bar**: tmux's status line, which names the detach keys, shows as
  `status_bar` says, and the status bar keys, bound in the same key table
  (`vineyard-detach-C-q-bar-C-k`), show and hide it. It lists no
  sessions.
- **Prompts**: initial prompts are sent as a bracketed paste (`set-buffer` and
  `paste-buffer -p`) followed by Enter, so multi-line text is submitted once.

The TUI reads from standard input and output when they are terminals and
falls back to `tea.OpenTTY` otherwise. Attached tmux clients inherit these
files, and tmux rejects a descriptor opened through `/dev/tty`.

## Multiplexers per Session

Each session runs in its own multiplexer, which it records
(`Session.Multiplexer`, saved in `sessions.json`), so one Vineyard runs
sessions in different multiplexers side by side. `main` makes every available
multiplexer: Vineyard's server always, and tmux when it is on `PATH`. Both
implement `tui.Terminal`; `session.Manager.Terminals` and
`tui.LiveBackend.Terminals` hold them by name, and every terminal operation on
a session goes to the session's own. Both name their socket `vineyard`, or
`VINEYARD_SOCKET`.

- **New sessions** take their profile's `multiplexer`, else the
  configuration's (`Config.MultiplexerFor`), `vineyard` by default. The new
  session dialog shows it.
- **Restore** looks for each session's terminal in every multiplexer, its
  recorded one first, and records where it found it. A session that recorded
  none, as those saved before sessions had one, and runs nowhere gets the
  configured default.
- **Unavailable**: a session whose multiplexer is not available, such as tmux
  when it is not installed, shows as stopped. Starting, resuming, and reading
  it fail with `session.Unavailable`; killing it only removes its worktree.

## Vineyard's Server

`internal/vterm` is a small multiplexer in tmux's design: a background server
owns the terminals, and every Vineyard is a client of it over a Unix socket,
so agents outlive the Vineyard that started them and several Vineyards share
them.

- **Socket**: `$TMPDIR/vineyard-<uid>/<name>-p<protocol>` (`vterm.SocketPath`,
  `vineyardSocketName`), in a directory only the user may use, as tmux keeps
  `/tmp/tmux-<uid>/<name>`. Servers of protocols 1 to 3 shared `<name>`.
  The server logs its start and stop, and each terminal's start and exit,
  with the exit status and the last lines of its screen, to `<socket>.log`
  (`internal/logfile`, restarted past 1 MB).
- **Starting**: a client that starts a terminal when no server answers runs
  `vineyard server <socket>` in a session of its own (`setsid`), working in
  `/`, and dials until it listens. The server checks for a live server and
  replaces a stale socket under a lock beside the socket (`<socket>.lock`,
  flock), so of several servers started at once all but one exit.
- **Calls**: a connection's first byte says what it carries. `r` starts
  `net/rpc` calls (gob encoded) to `vterm.Service`, one per `tui.Terminal`
  method; each client keeps one such connection. Every connection first asks
  `Version`, and a server of another protocol version is an error naming its
  process ID (`protocol` in `daemon.go`; change it with any change to the
  calls). `Client.Info` asks `Version` and `List` without that check, for
  `vineyard debug`.
- **Protocols side by side**: each protocol's server has its own socket, so
  an update that changes the protocol leaves the agents running on the older
  server while new sessions start on the new one. A session records the
  protocol it runs on (`Session.Protocol`; 0 for sessions recorded before,
  on `<name>`). `Manager.terminal` and `LiveBackend.terminal` send a session
  of another protocol to `vineyards.Older`, which returns:
  - a client, when that server speaks this build's protocol;
  - `vterm.NewOlderClient`, when its calls match this build's
    (`sameCallsSince` in `daemon.go`, 1: protocols 1 to 5 differ only in
    attaching). It makes every call, so the session shows its screen and
    status and can be killed;
  - otherwise `olderVineyard`, which asks only `Version` and `Exists`, through
    `Client.Holds`, and fails every other call but attaching with
    `session.ErrOlderMultiplexer`. The TUI keeps such a session's status and
    shows the error instead of its screen.
  Both attach through the older server's executable
  (`vterm.AttachThroughServer`): attaching is what protocols change, and that
  executable speaks its server's. Vineyard runs `/proc/<pid>/exe attach
  <socket> <terminal> <attachment as JSON>` with the server's process ID,
  which runs the executable even after an update replaced the file, and hands
  it the user's terminal. Those arguments, and `session.Attachment`'s JSON
  names, are the contract between versions; never change them. Builds before
  the `attach` command exit with status 2, reported as predating it; systems
  without `/proc`, such as macOS, cannot attach to an older server.
  Every protocol answers `Version` and `Exists` alike; never change them.
  Changing another call raises `sameCallsSince`, and older servers fall back
  to `olderVineyard`. Resuming restarts a session on this build's server once
  it ended on the older one.
- **Terminals**: `vterm.Server` runs each program through `$SHELL -c` on a
  pseudo-terminal (`creack/pty`) with the starting Vineyard's environment and
  `TERM=xterm-256color`, and feeds its output to a `charmbracelet/x/vt`
  emulator, which keeps 10000 lines of scrollback as tmux does. Its parser
  ends OSC and DCS strings at byte `0x9C`, the 8-bit string terminator,
  which is also inside UTF-8 characters such as the `✳` Claude Code puts
  before its title. `vterm`'s `init` keeps `0x9C` in those strings, in the
  `charmbracelet/x/ansi` table every parser in the process shares, until an
  upstream fix is released.
- **Preview**: the emulator's screen rendered with ANSI styles. Resizing
  resizes the emulator and the pseudo-terminal; lines do not reflow.
- **Keys and prompts**: go through the emulator, which encodes keys for the
  program's cursor-key mode and brackets pastes when the program asked for
  that. The emulator answers the program's terminal queries itself. One
  goroutine copies both to the program, so none of them may block on `mu`.
- **Attach**: as tmux's client does, the client passes the user's terminal to
  the server: an `a` connection whose first byte carries the input and output
  file descriptors (`SCM_RIGHTS`), then an `AttachRequest` with the user's
  environment, terminal size, keys, and status bar. The server puts that terminal in raw mode,
  enters the alternate screen, and redraws the emulator on it with
  ultraviolet's diffing renderer, at most 60 times a second. Typed input goes
  to the program until a detach byte. The client forwards SIGWINCH as new sizes and
  waits for the `AttachResult`; if its connection drops, the server ends the
  attachment. The program's output never reaches the user's terminal
  directly, so modes it sets there are only those that change input (cursor
  keys, mouse, focus, bracketed paste), set and reset as the program does and
  reset on detach.
- **Status bar** (`bar.go`): while attached, the TUI's loop is blocked in
  `tea.Exec`, so the server keeps the attached terminal's status bar. The
  `AttachRequest` carries the attached session's title, which the bar names
  first in bold, the sessions to list (`session.Peer`: terminal
  name, title, and whether Vineyard last saw it running), whether the bar
  starts shown (`status_bar`), and whether the user's terminal is dark, as
  Bubble Tea reported it to the TUI. The server cannot tell, and the bar is
  darker than a light background or lighter than a dark one, in 256-colour
  palette colours, which themes leave alone. The bar takes the last row of the user's
  terminal and the program one row less; at the bottom, the mouse reports
  the user's terminal sends still match the program's rows. Every second the
  server fingerprints each listed terminal's screen as the TUI's status poll
  does: changed is running (◑), unchanged is ready (●), and ready after
  running during the attachment is finished (✓). Until a terminal has been
  sampled twice, it shows the status sent. The status bar keys, found in the
  input as the detach bytes are, resize the program to the full height and
  back; each attachment starts from `status_bar`. The TUI lists the active
  sessions in the attached session's multiplexer but for the agent attached
  to.
- **Exit**: a terminal ends when its program exits; Kill hangs up its process
  group. The server stops once it has neither terminals nor connections, but
  not before its first terminal starts or 10 seconds pass, since the client
  that started it connects after servers racing it probe it. Killing the
  server hangs up every agent, as killing tmux's server does.

## Status Detection

There is no structured channel from the agent, so status is inferred from the
screen, as claude-squad does. Every second each live session's screen is
captured and fingerprinted. A changed fingerprint means running; a fingerprint
unchanged for `stillFor` (600 ms) since the check that first saw it means
ready. Agents animate while they work: running Claude Code agents were
measured to change their screens at least every 0.45 s.

No check runs while the user is attached, because `tea.Exec` blocks the TUI's
event loop. Detaching therefore checks at once and then every 100 ms for a
second, so the statuses and subagents that changed meanwhile show within
`stillFor`, not after several one-second checks.

On top of that:

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

## Key Bindings

`config.DefaultKeys` lists every mode (`global`, `list`, `pane`, `settings`,
`form`, `confirm`, `pick`), its actions, and their default keys.
`Config.Keys` holds only what `[keys.<mode>]` tables in `config.toml` set, so
`Save` writes back what the user wrote. `Config.ResolvedKeys` lays those over
the defaults and rejects unknown modes and actions, key names Bubble Tea never
reports (`shift+a` is reported as `A`), a key two actions of a mode share or
that `global.quit` or `global.back` takes first, and detach keys other than ctrl and a letter.
`Load` treats that like a malformed file: defaults plus the error.

`NewModel` installs the resolved keys in `common.Keys`, a `KeyMap` of
`key.Binding`s per mode that every view matches against; `NewKeyMap` panics
when it and `DefaultKeys` disagree on the actions. Footer hints, a
confirmation's answers, and messages such as "Press r to resume it." name the
first key of a binding through `common.Hint` and `common.PressTo`, and leave
out unbound actions. The detach keys go to the multiplexer with each attach.

Out of scope: the text-editing keys inside inputs (bubbles' defaults) and
grapes' keys on the issues screen, set in `.grapes/config.toml`.

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
acts as a single click; the second calls `open`, what the open key does, or
`attach` on a subagent row, so a double click opens a subagent on every tab.
The mouse calls actions, never fakes key presses, so it works whatever the
keys are. Where the
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
main -> config, git, session, tmux, vterm, tui, grapes/embedded
tui -> session, subagent, git, config, diff (via diffview), tui/*, grapes/embedded
tui/* -> tui/common, and data packages
subagent -> session
session -> git
git, tmux, diff, config -> standard library and format libraries
vterm -> pty, vt, ultraviolet, net/rpc
```

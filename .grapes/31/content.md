## Goal

Show each Claude Code session's running subagents in Vineyard's session list, and let Enter on one open that subagent's view in Claude Code and attach to it.

## Description

Claude Code runs subagents inside the parent `claude` process. They have no pane or tmux window of their own. Claude Code lists the running ones under its prompt, and its own key protocol opens one in place of the main conversation. Today a Vineyard user has to attach, press ↓, find the row, and press Enter; and Vineyard's list does not say which sessions have subagents at all.

This issue adds two things:

1. Subagent rows. For every live Claude session, Vineyard reads Claude's agent list off the screen it already captures every second, and shows one row per subagent under the session. Selecting a row types nothing: the pane keeps showing the parent session.
2. Opening. Enter on the preview tab, or `o` on any tab, on a subagent row makes Vineyard press Claude's keys to open that subagent's view, checking the screen after each step, and then attaches.

Agent teams (teammates) are out of scope.

## Context

### Claude Code's agent list (verified on 2.1.296, both `tui` renderers, 82 and 120 columns)

The list sits at the bottom of the screen, below the prompt. With ANSI codes stripped:

```text
  ● main
  ◯ general-purpose  Worker one sleeps                       5s · ↓ 19.4k tokens
  ◯ general-purpose  Worker two sleeps                       5s · ↓ 19.4k tokens
  ↓ 1 more
```

- One row per running subagent: marker, agent type, two spaces, a label, and stats. The label is Claude's live summary of what the subagent is doing; it changes every few seconds.
- `●` marks the conversation shown: `main`, or the subagent whose view is open. `◯` marks the others.
- At most five subagent rows are drawn. `↓ N more` follows when more are below; `● main` ends with `↑ N more` when the list is scrolled down.
- A subagent leaves the list when it finishes, except the one whose view is open: Claude keeps it until the user leaves the view.
- With no running subagents there is no list.
- In this setup the Agent tool runs every subagent in the background, so every subagent appears in the list.

### Claude Code's keys (verified)

- ↓ in the prompt moves the cursor down through a multi-line draft; on the draft's last line it gives the list focus. A focused list shows `❯ ` before the selected row, instead of two spaces.
- In the focused list, ↑/↓ move the selection, Enter opens the selected row's view, and the list keeps focus. Esc gives focus back to the prompt. `x` stops the selected subagent.
- In a subagent's view the prompt reads `Message @<type>…`; typing there messages the subagent. The draft typed for main is shared between views and survives a round trip.
- Esc outside the list is Claude's cancel: it interrupts a working agent. Vineyard must send Esc only while the screen shows the list focused.

### Vineyard (verified)

- `statusCmd` in `internal/tui/app.go` captures every active session's screen each second (`Backend.Capture`, `capture-pane -p -e`), and `applyStatus` applies the results.
- `internal/tui/list/list.go` draws fixed three-line entries (`itemHeight`); scrolling and clicks assume that height.
- Attach is `Backend.AttachCommand` run with `tea.ExecProcess` from `handleKey` (`keys.Attach` is `o`; `keys.Open` is Enter, which attaches on the preview tab).
- `internal/tmux/tmux.go` sends Enter (`SendEnter`) and pastes, but no other keys.
- Docs say only the leading Vineyard types into agents. That rule is about typing Vineyard does on its own (pending prompts, auto-yes).

### Planned changes

- New package `internal/subagent`: `Parse(screen)` returns the agent list found on a screen; `Open` drives the key protocol through a small interface (capture, send keys) and checks the screen after each step.
- `tmux.Client.SendKeys(name, keys...)`; `Backend.OpenSubagent(s, index, count)`.
- `statusCmd` parses each Claude session's screen; the model keeps the latest list per session and passes it to the list items.
- `list.Item.Subagents`; entries grow by one line per subagent row and one for `+N more`; the selection can rest on a subagent row; `list.Model.SelectedSubagent()`.
- `handleKey`: Enter on the preview tab, or `o`, on a subagent row runs `OpenSubagent`, then attaches on success, and shows the error otherwise.
- Docs: `docs/architecture.md` and `docs/README.md`.

### Decisions

- Read the list from the screen, not from transcripts: row k in Vineyard is then always row k in Claude's list, which is what the keys move through.
- Opening a subagent is typing the user asked for, so every Vineyard does it, not only the leader.
- Opening checks the list size against what Vineyard showed and fails if it changed, instead of opening another subagent.
- On a failure after Vineyard focused the list, Vineyard presses Esc only if the screen shows the list focused, so no stray `x` can stop a subagent later.

## Acceptance Criteria

- [ ] A live Claude session with running subagents shows one row per subagent in Claude's list, under the session, within one status poll; `+N more` stands for rows Claude hides; rows go when Claude drops them.
- [ ] Moving through the list and clicking reach subagent rows; the pane keeps showing the parent session; selecting a row sends no keys.
- [ ] Enter on the preview tab, or `o`, on a subagent row opens that subagent's view in Claude and attaches; once attached, the prompt has focus and reads `Message @…`.
- [ ] Opening works for a subagent below Claude's fifth row, and when the session already shows another subagent's view.
- [ ] Opening preserves a draft in Claude's prompt, single- or multi-line.
- [ ] When the list changed size, or a step's expected screen does not appear, Vineyard does not attach, shows an error, and leaves Claude's prompt focused.
- [ ] Sessions running other programs show no subagent rows.
- [ ] Tests cover parsing captured Claude screens, the key protocol against a fake Claude list, list rendering and selection, and Enter and `o` on a subagent row.
- [ ] `docs/architecture.md` and `docs/README.md` describe subagent rows.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual, in a real terminal: start a development build on a throwaway repository with `claude` as the program, ask the agent to run several background subagents, and check each criterion above. Capture screenshots for the PR.

## Pass Criteria

`gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass in under 10 seconds. In the manual run, the rows match Claude's list, and Enter on each row lands attached in that subagent's view.

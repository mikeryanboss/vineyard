## Goal
A Recap tab beside Preview, Diff, and Issue that shows what a Claude Code
session has done so far, from the recaps Claude Code already writes.

## Description
Claude Code writes a short recap ("I did X. Next, Y.") into its session
transcript when the user returns after being away. Vineyard reads those recaps
from disk and shows them, newest first, under the session's AI-generated title.
Recaps are sparse (1–2 per session, none in about half of the sessions
observed), so a session without one shows its last prompt instead.

Only Claude Code is supported for now.

## Context
Verified against Claude Code 2.1.295 transcripts in `~/.claude/projects/`:

- Transcripts are `<config>/projects/<encoded cwd>/<session-id>.jsonl`, where
  `<config>` is `$CLAUDE_CONFIG_DIR` or `~/.claude`, and the encoded cwd is the
  path with every non-alphanumeric character replaced by `-`
  (`/home/u/.vineyard/worktrees/x-1` → `-home-u--vineyard-worktrees-x-1`).
  The encoding is undocumented; it was inferred from directory names.
- A recap is
  `{"type":"system","subtype":"away_summary","content":"… (disable recaps in /config)","timestamp":"2026-10-09T09:40:22.130Z",…}`.
  The trailing hint is not part of the recap.
- The title is `{"type":"ai-title","aiTitle":"…"}`; the last prompt is
  `{"type":"last-prompt","lastPrompt":"…"}`. Later entries supersede earlier ones.
- `/clear` and some resumes start a new `.jsonl` in the same directory, so one
  worktree can have several transcripts.
- The agent runs with `Session.WorktreePath` as its cwd
  (`internal/session/session.go`).

Current pane tabs: `tab` constants, `tabNames`, `nextTab`, `renderPane` in
`internal/tui/app.go`; the issue tab's scrollable pane is
`internal/tui/issueview/`, which holds no issue-specific logic.

Changes:
- New `internal/recap/`: `Load(projectsDir, cwd)` reads every top-level
  `.jsonl` of the worktree's transcript directory, returns recaps sorted newest
  first, and the title and last prompt of the most recently modified
  transcript. A missing directory means no recaps, not an error; a malformed
  matching line is an error.
- `Backend.Recap(s)` reads the selected session's transcripts.
- `internal/tui/issueview/` becomes `internal/tui/textview/`, a scrollable
  text pane used by both the issue and recap tabs.
- `tabRecap` after `tabIssue`; fetched when the tab is shown, when the
  selection changes, and on each diff tick while shown. Results are tagged with
  the session ID and dropped if another session is shown.
- Sessions whose program is not `claude` show a placeholder saying recaps
  come from Claude Code.

## Acceptance Criteria
- [x] Tab cycles Preview → Diff → Issue → Recap → Preview.
- [x] The Recap tab shows the title and every recap, newest first, with local times, without the "(disable recaps in /config)" hint.
- [x] A Claude Code session with no recap shows "No recap yet" and its last prompt.
- [x] A session with no transcript, and a non-Claude session, each show a placeholder.
- [x] Recaps from all transcripts in the worktree's directory are merged.
- [x] A recap result for a session that is no longer shown is dropped.
- [x] Docs list the recap tab and package.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run vineyard against a worktree with a real Claude Code transcript and
screenshot the Recap tab.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass; the screenshot shows the
session's real recaps under its title.

## Goal
Extend the Recap tab to support both Claude Code (`claude`) and Antigravity CLI (`agy`) sessions, displaying conversation titles, user prompts, final agent responses, and Claude Code away-recaps in an interactive, collapsible card view where entries are limited to a few preview lines by default and can be expanded or folded by click or keyboard.

## Description
Previously, the Recap tab was gated on `isClaude(s)` and used a plain `textview.Model` showing only user prompts and Claude Code `away_summary` recaps in full. Sessions running `agy` showed a placeholder (`"Recaps come from Claude Code; this session runs agy."`), and long multi-line prompts or responses forced extensive scrolling without entry navigation or folding.

Extend `internal/recap` to:
1. Distinguish `KindPrompt`, `KindResponse`, and `KindRecap` entries on `recap.Entry`.
2. Read final assistant responses (`"type":"assistant"`, `!isSidechain`, `"stop_reason":"end_turn"`, non-synthetic text blocks) from Claude Code transcripts alongside titles, user prompts, and away-recaps.
3. Read Antigravity CLI (`agy`) conversations incrementally from `<gemini-dir>/antigravity-cli` (`$ANTIGRAVITY_GEMINI_DIR`, default `~/.gemini`): user prompts from `history.jsonl` (filtering out `"slash_command"` entries) and `brain/<id>/.system_generated/logs/transcript_full.jsonl` (`USER_INPUT`), final responses (`PLANNER_RESPONSE` with non-empty `content`), and generated titles (`annotations/<id>.pbtxt`), including linking first-turn conversations whose initial `history.jsonl` line omits `conversationId`.

Add `internal/tui/recapview` (modeled on `internal/tui/diffview`) to render the Recap tab with:
- Conversation title (when present) and entry-kind summary counts (`prompts`, `responses`, `recaps`).
- Entries truncated by default to 4 wrapped preview lines (`▸` chevron and `… N more lines` footer when longer), expandable and collapsible individually via mouse click or `PaneKeys.Fold` (`enter`/`o`), and in bulk via `PaneKeys.UnfoldAll` (`e`) and `PaneKeys.FoldAll` (`c`).
- Entry-to-entry navigation via `PaneKeys.NextFile` (`]`/`n`) and `PaneKeys.PrevFile` (`[`/`p`), keeping the active entry visible, plus live bottom-following when new entries arrive while scrolled to the bottom.

## Context
- `internal/recap/recap.go`, `internal/recap/agy.go`, `internal/recap/recap_test.go`: `Entry`, `Kind`, `Recap`, `Reader.Load`, `Load`, `AntigravityDir`, `Reader.LoadAgy`, `LoadAgy`.
- `internal/tui/recapview/recapview.go`, `internal/tui/recapview/recapview_test.go`: collapsible, preview-limited recap pane view with keyboard and mouse support.
- `internal/tui/backend.go`: `LiveBackend.Recap` dispatches by `session.ProgramName(s.Program)` (`claude` vs `agy`).
- `internal/tui/app.go`, `internal/tui/app_test.go`: replace `isClaude`-only gating in `recapCmd` and `refreshRecap` with `hasRecap` (`claude` and `agy`), wire `recapview.Model`, `Clickable` double-click suppression, focus tracking, entry count in tab bar, and status bar hints.
- `docs/README.md`, `docs/architecture.md`, `docs/development.md`, `README.md`: update Recap tab documentation.

## Acceptance Criteria
- [x] `internal/recap` extracts `KindPrompt`, `KindResponse` (final `end_turn` assistant text responses, excluding `tool_use` blurbs, sidechains, and `<synthetic>` messages), and `KindRecap` from Claude Code transcripts incrementally.
- [x] `internal/recap` extracts conversation titles (`annotations/<id>.pbtxt`), user prompts (`KindPrompt`, excluding slash commands), and final responses (`KindResponse`, `PLANNER_RESPONSE` with text `content`) for `agy` sessions matching `s.WorktreePath` exactly under `$ANTIGRAVITY_GEMINI_DIR/antigravity-cli` (default `~/.gemini/antigravity-cli`), including single-turn sessions whose first `history.jsonl` entry has no `conversationId`.
- [x] `LiveBackend.Recap`, `recapCmd`, and `refreshRecap` load and display recaps for both `claude` and `agy` sessions, while unsupported programs still explain which harnesses provide recaps.
- [x] `internal/tui/recapview` limits entries longer than 4 wrapped lines to a 4-line preview by default with a `▸ … N more line(s)` hint, and supports 3 fold states (`collapsed` 1-line header with snippet, `preview` 4 lines, `expanded` full), clickable header chips (`▾/▸ N prompts`, `▾/▸ N responses`, `▾/▸ N recaps`, `expand all`, `collapse all`), pane keybindings (`1`/`2`/`3` per kind, `e`/`c` all, `enter`/`o` cycle, `]`/`[` navigate), and `[recap]` config options (`prompts`, `responses`, `recaps` = `preview` | `collapsed` | `expanded` | `off`) editable in the Config screen's `Recap` category.
- [x] Selecting a subagent row continues to display the subagent's description, status, report, and prompt in the Recap tab.

## Verify
Run from `/var/home/mboss/dev/vineyard/.vineyard/worktrees/recap-69477c`:
```bash
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -count=1 ./...
```
Also drive the built TUI in tmux with `scripts/drive` on a throwaway repository containing `claude` and `agy` transcript fixtures to verify default 4-line truncation, keyboard navigation/expansion (`[`, `]`, `enter`, `e`, `c`), and mouse click expansion/folding, and capture screenshots for the PR.

## Pass Criteria
`gofmt -l .` prints nothing; `go vet ./...` exits 0; `CGO_ENABLED=0 go test -count=1 ./...` passes all packages in under 10 seconds; driven TUI renders collapsible prompts, final responses, and recaps for both `claude` and `agy` sessions cleanly.

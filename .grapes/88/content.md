## Goal
Show a session's title before its issue numbers by default so titles line up and get width priority, and let repositories configure which session list elements appear in `.vineyard/config.toml` and the config screen.

## Description
`renderItem` in `internal/tui/list/list.go` prints a session's issue numbers (`#12 #13 +1`) before its title and subtracts their width from the title's budget, and `showIssues` in `internal/tui/app.go` lists choices as `#12 Title`. Users who care primarily about session titles get ragged title alignment and lose title width to issue numbers on narrow lists, while users who do not use grapes, Claude Code subagents, or secondary row metadata have no way to hide them.

Changes:
1. **Session list display table (`[list]` in `.vineyard/config.toml`).** Add `config.ListConfig` on `Config.List` (`toml:"list"`), defaulted by `config.DefaultList()`:
   - `issues = "after"` (`"after"`, `"before"`, or `"off"`). Empty in a file defaults to `"after"`; any other value fails `config.Load`.
   - `issue_picker = "after"` (`"after"` or `"before"`). Empty defaults to `"after"`; any other value fails `config.Load`.
   - `subagents = true` (`bool`): shows Claude Code's subagent count line and expandable rows, and polls subagent screens and transcripts. When `false`, `pollStatuses` skips `subagent.Parse` and `Backend.Subagents`, using `Backend.Conversation` when leading so Claude Code's conversation ID is still saved for resume.
   - `branch = true` (`bool`): shows each session's `⑂ <branch>` second line. When `false`, sessions take one content line plus spacer (`entryHeight` drops by 1, and `LineAt` / `click` offset subagent count and subagent rows accordingly); if `diff_stats` is `true`, `+N -M` renders at the right end of the title line.
   - `diff_stats = true` (`bool`): shows `+N -M` line counts in the session list.
   - `auto_tag = true` (`bool`): shows `auto` on the title line of auto-yes sessions.
2. **Issue numbers after the title clip first.** When `issues` is `"after"`, the title renders first in the budget before `auto` (and before inline `diff_stats` when `branch` is `false`). The title truncates only when the title itself exceeds that budget; when the title fits with room left, `" #12 ..."` appends and truncates from the right with `…` so the title keeps every character.
3. **Issue picker order.** `showIssues` formats each choice as `<Title> #<id>` when `issue_picker` is `"after"` and `#<id> <Title>` when `"before"`, falling back to `#<id>` when the issue has no title.
4. **Config screen (`C`).** Add a `Session list` category in `internal/tui/settings/settings.go` with enum fields for `Issue numbers`, `Issue picker`, `Subagents`, `Branch line`, `Diff stats`, and `Auto-yes tag`. Saving applies the updated `Config.List` immediately via `syncList`, `resizeLayout`, and `refreshShown`.
5. **Documentation.** Document `[list]` in `README.md` and keep `docs/README.md` and `docs/architecture.md` accurate.

## Context
- `internal/config/config.go`: `Config`, `Defaults`, `Load`, `Save`; planned `ListConfig`, `DefaultList`, `IssuesAfter`, `IssuesBefore`, `IssuesOff`.
- `internal/tui/list/list.go`: `Model`, `New`, `entryHeight`, `LineAt`, `click`, `renderItem`; planned `Options`, `DefaultOptions`, `Model.SetOptions`.
- `internal/tui/app.go`: `NewModel`, `syncList`, `pollStatuses`, `applyStatus`, `configSavedMsg`, `showIssues`.
- `internal/tui/backend.go` and `internal/subagent/transcript.go`: `LiveBackend.Conversation` currently handles `agy` only; planned `subagent.ConversationID` lets `LiveBackend.Conversation` also return Claude Code's newest transcript ID without loading subagents.
- `internal/tui/settings/settings.go`: `categories`, `fields`, `value`, `setValue`.
- `README.md`: Configuration section.

## Acceptance Criteria
- [x] By default (`issues = "after"`), the session list renders issue numbers after the session title (`<Title> #12`), clipping issue numbers from the right before truncating a title that fits.
- [x] Setting `list.issues` to `"before"` restores `#12 <Title>` and `"off"` omits issue numbers from the session list; invalid `list.issues` or `list.issue_picker` values return an error from `config.Load`.
- [x] `list.issue_picker` formats choices in `showIssues` as `<Title> #<id>` (`"after"`, default) or `#<id> <Title>` (`"before"`).
- [x] Setting `list.subagents = false` hides subagent count and tree rows in the session list, skips subagent polling in `pollStatuses`, and still records Claude Code's conversation ID when leading.
- [x] Setting `list.branch = false` renders compact single-line session entries, keeps mouse selection/folding aligned on subagent rows, and places `+N -M` on the title line when `list.diff_stats = true`.
- [x] Setting `list.diff_stats = false` hides `+N -M` in the session list, and `list.auto_tag = false` hides `auto` on auto-yes sessions.
- [x] The `C` config screen edits and saves all six `Session list` settings, applying them to the open session list immediately.
- [x] `gofmt -l .`, `go vet ./...`, and `go test ./...` pass in under 10 seconds.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then drive a development build in tmux via `scripts/drive` with sessions linked to issues and subagents, verifying default (`after`) and configured session list views and the `Session list` category in `C`.

## Pass Criteria
`gofmt -l .` prints nothing, `go vet ./...` and `go test ./...` succeed in under 10 seconds, and captured screenshots show issue numbers after titles by default, configurable session list items, and the `Session list` config category.

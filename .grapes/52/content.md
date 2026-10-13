## Goal
Make the recap tab useful for sessions with few or no recaps: show every
message the user typed to Claude Code, with Claude Code's recaps interleaved
where they occur, styled so messages and recaps are easy to tell apart.

## Description
Today the recap tab lists only Claude Code's recaps (`away_summary` lines),
newest first, and falls back to the transcript's `last-prompt` when there are
none. Recaps are rare, so the tab is usually nearly empty.

The tab will instead show one chronological conversation log, oldest first,
merged across the working directory's transcripts:

- every human message (`"type":"user"` with `"origin":{"kind":"human"}`),
- every recap, at the time Claude Code wrote it.

It opens scrolled to the newest entry and follows new entries while scrolled
to the bottom. Messages and recaps get distinct labels, markers, and colours.
The `last-prompt` fallback goes away: the messages make it redundant.

## Context
- `internal/recap/recap.go`: `Load`, `read`, `Entry`, `Recap`.
- `internal/tui/app.go`: `refreshRecap` renders the tab; `refreshShown` scrolls it.
- Transcript format read off Claude Code 2.1.280–2.1.296: human messages carry
  `"origin":{"kind":"human"}` and `promptSource` `typed` or `queued`; slash
  commands, tool results, and meta lines carry no `origin.kind`. `message.content`
  is a string, or an array of blocks (seen from the SDK).

## Acceptance Criteria
- [x] `recap.Load` returns human messages and recaps, oldest first, merged across transcripts; tool results, meta lines, and slash commands are excluded.
- [x] The recap tab shows them in that order, each with its time, and messages look clearly different from recaps.
- [x] The tab opens at the newest entry and follows new entries while at the bottom.
- [x] `docs/architecture.md` describes the new tab.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run Vineyard on a Claude session with several messages and a recap, open the recap tab, and capture a screenshot.

## Pass Criteria
Formatting, vet, and tests pass; the screenshot shows messages and recaps interleaved in order and visually distinct.

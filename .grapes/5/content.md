# Diff view: file cards that can be collapsed

## Goal
Make file boundaries in the diff view unmistakable and let the reader fold files away.

## Description
The diff view mirrors raw `git diff` output closely enough that files are hard to tell apart. Draw each file as a card and make cards collapsible.

## Context
- `internal/tui/diffview/diffview.go`: `render`, `renderFileHeader`, `Update`.
- Hints in `internal/tui/app.go`, keys in `internal/tui/common/keys.go`.
- Docs: `docs/architecture.md` (diffview paragraph).

## Changes
- Each file renders as: heavy rule, header bar, heavy rule, body, blank line.
- The header shows `▾` (expanded) or `▸` (collapsed) in place of `▌`.
- `enter`/`o` toggles the file at the top of the view; `c` collapses all files; `e` expands all.
- Collapsed state is keyed by path and survives diff refreshes.

## Acceptance criteria
- [x] Each file is a card with rules above and below its header.
- [x] A file can be collapsed and expanded; a collapsed file shows only its header.
- [x] Collapse all and expand all work.
- [x] Collapsed state survives `SetDiff` with a changed diff.
- [x] Golden and behaviour tests updated; docs and key hints updated.
- [x] `gofmt -l .`, `go vet ./...`, `go test ./...` pass.

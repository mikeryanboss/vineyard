## Goal
Make the README a short, plain introduction that shows Vineyard working, so a new reader understands it in a minute.

## Description
The README has grown one paragraph per feature. Rewrite it to cover only the major features, cut filler and inflated phrasing, and keep install, usage, and configuration accurate. Add a short demo GIF recorded with [VHS](https://github.com/charmbracelet/vhs), following grapes, which keeps its tapes in `doc/vhs/` and its output in `doc/demo.gif`.

## Context
- `README.md`: current README; key table, flags, configuration, and "How it works".
- grapes: `doc/vhs/demo.tape` records `doc/demo.gif`; its README embeds it at the top.
- `docs/development.md`: "Screenshots for Pull Requests" records the throwaway-repo and private tmux socket approach (`VINEYARD_TMUX_SOCKET`).
- `docs/README.md`: repository map; it gains `doc/`.

## Acceptance Criteria
- [x] README covers what Vineyard is, install, the main keys, configuration, and how it works, and nothing that is out of date.
- [x] README embeds `doc/demo.gif`, recorded from `doc/vhs/demo.tape` against a throwaway repository.
- [x] `docs/` says how to re-record the demo, and the repository map lists `doc/`.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
vhs doc/vhs/demo.tape
```
Check each README key and flag against `internal/tui/common/keys.go` and `main.go`, and watch the GIF.

## Pass Criteria
Checks pass, every documented key and flag exists, and the GIF shows sessions, the preview, the diff, and the issues screen.

## Goal
Remove `idea.md`, the Symphony/Pi orchestration sketch, so it no longer reads as the current plan.

## Description
`idea.md` describes the earlier TypeScript/Pi/Chokidar design. Issue #1 refined it, and #2 replaced that prototype with the Go TUI. The user asked to remove it.

## Context
- Remaining mentions of `idea.md` are in `.grapes/1/comments.md` (append-only) and `.grapes/2/content.md` (a done issue). Both are history and stay unchanged.
- No code or docs under `docs/` reference it.

## Acceptance Criteria
- [x] `idea.md` is deleted.
- [x] No file outside issue history references `idea.md`.

## Verify
Run from the repository root:
```bash
test ! -e idea.md && grep -rn "idea.md" --exclude-dir=.git --exclude-dir=.grapes .
```

## Pass Criteria
The command prints nothing and exits 1 (grep finds no match).

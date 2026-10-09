## Goal
Restore the README's credit to claude-squad, which the rewrite in #23 dropped.

## Description
Before #23, the README said Vineyard "follows [claude-squad](https://github.com/smtg-ai/claude-squad)'s model, rebuilt on Bubble Tea v2". The rewrite removed the sentence. Vineyard's design comes from claude-squad, so the README should say so.

## Context
- `README.md`: the introduction above the demo GIF.
- The pre-#23 wording is in `git show ccb38e4:README.md`.

## Acceptance Criteria
- [x] The README introduction links to claude-squad and says Vineyard follows its model, rebuilt on Bubble Tea v2.

## Verify
Run from the worktree root:
```bash
grep -n 'claude-squad' README.md
```

## Pass Criteria
The grep prints the credit line in the introduction.

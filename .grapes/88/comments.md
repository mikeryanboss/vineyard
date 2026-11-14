### 2026-11-14T18:53
[DECISION] User confirmed design choices:
1. Session list issue numbers use a three-way setting: `after` (default), `before`, `off`.
2. When `after`, issue numbers clip off the right first so the title gets width priority (`Fix login redirect #12` -> `Fix login redirect…`).
3. Expose six settings under `[list]` in `.vineyard/config.toml` and in the `C` config screen: issue numbers (`issues`), issue picker order (`issue_picker`), subagents (`subagents`), branch line (`branch`), diff line counts (`diff_stats`), and auto-yes tag (`auto_tag`).

### 2026-11-14T18:54

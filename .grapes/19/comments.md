### 2026-10-09T11:50
[FINDINGS] sessions.json (gitignored) stores absolute worktree_path; git worktree links are absolute by default; stored branch goes stale when agents switch branches (grapes-62ff23: stored mboss/grapes, actual 18/open-at-issue). Grapes persists no paths; it discovers worktrees with git worktree list.

### 2026-10-09T11:50
[DECISION] Find worktrees through git by session ID, create them with --relative-paths, repair broken links at startup, and read live branches from git. Agreed with the user.

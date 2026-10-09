### 2026-10-09T12:05
[DECISION] Keep `sessions.json` and lock each write rather than splitting it into one file per session: a reload is needed either way, and keeping the file avoids migrating existing sessions. A lead role, not claims per prompt, stops doubled keystrokes, because auto-yes Enters have nothing to claim.

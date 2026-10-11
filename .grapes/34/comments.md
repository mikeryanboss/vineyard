### 2026-10-11T16:21
[DECISION] A double click on a subagent row opens the subagent on every tab, as `o` does, since opening it is what the row is for; elsewhere a double click is `enter`. Where the first click already acts (the count line, diff headers and summary lines), the second click of a double is dropped so the action happens once. Double clicks are told on the main screen only: a dialog's first click answers it, and the click that closes it must not pair with one on the screen beneath.

### 2026-10-11T16:21
[STARTED] Double-click detection in `handleMouse`, `list.CountLine`, tests, docs.

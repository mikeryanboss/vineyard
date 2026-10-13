### 2026-10-13T16:40
[STARTED] The user reported the bar as "just like a gray bar" in light mode (kitty Solarized light, Vineyard in VS Code's terminal). Pass the TUI's dark/light to the server and keep two palettes.

### 2026-10-13T16:55
[DECISION] The TUI sends `Dark` (its theme's IsDark, from Bubble Tea's background report) instead of the server querying the user's terminal: the TUI already knows, and reading a reply out of the attached input would race typing. Light palette: background 254, text 235, hint 242, running 130, finished 28; dark keeps 238/252/248/220/114. Protocol 4, since attaching changed.

### 2026-10-13T16:56
[VERIFY] gofmt -l . (no output), go vet ./... clean, go test ./... PASS. New: TestBarSuitsTheBackground (254 light, 238 dark, "no other sessions"), dark case in TestApp_AttachListsTheOtherSessionsInTheStatusBar. Real app in tmux, which reports no background, so the TUI is light: the bar is drawn with 48;5;254 and the render on a Solarized light canvas reads well (.grapes/64/tmp/light-{running,finished}.png). PASS

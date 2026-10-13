### 2026-10-13T11:25
[DECISION] `T` creates a session of the `terminal` profile rather than a throwaway shell outside the session list, so it shows in the list, can be reattached, and is killed with `D` like any terminal from `N`. It attaches right after start, since the point is quick access. Profiles come from `ResolvedProfiles`, so a configured `terminal` profile is honoured and a disabled one refused, also under `--program`.

### 2026-10-13T11:50
[VERIFY] `gofmt -l .` empty; `go vet ./...` and `go test ./...` pass. `TestApp_TerminalKeyStartsAndAttachesATerminal` covers start-and-attach, failed start, and the disabled profile; three key-hint goldens updated for `T new terminal`. PASS.
Manual, tmux 110x30, throwaway repo: `T` attached to a new shell in the main checkout (`echo $PWD` printed the repo root); `ctrl-q` returned to the list with `terminal` selected; a second `T` made a second `terminal` session. PASS. Screenshots: `.grapes/55/tmp/attached.png`, `.grapes/55/tmp/list.png`.

### 2026-10-13T11:51
[DONE] `T` (`ListKeyMap.Terminal`) creates the `terminal` profile's session through `createSession(msg, attach)`; `startedMsg.attach` attaches once it runs. README key table and profiles paragraph mention it.

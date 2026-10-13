### 2026-10-13T11:40
[STARTED] Keymap as `[keys.<mode>]` tables of key lists over `config.DefaultKeys`; `common.Keys` installed by `NewModel`; mouse calls actions; detach keys passed per attach.

### 2026-10-13T13:50
[DECISION] Config holds only the user's overrides, so `Save` writes back what was written; `ResolvedKeys` validates in `Load`, which reports bad keys like a malformed file. tmux binds detach keys in a per-session key table at attach time (`vineyard-detach-C-q`) instead of the server-wide root table, so Vineyards with different keys can share the server. Detach keys are limited to ctrl+letter (not i/m) because both multiplexers match one input byte. Footer now shows the first key of each binding, so subagents show `l`/`h` instead of `→`/`←`.

### 2026-10-13T13:55
[VERIFY] `gofmt -l .` empty; `go vet ./...` clean; `go test -count=1 ./...` PASS (~6.5 s). New tests: config/keys_test.go (overrides, unbinding, 13 rejection cases, README block equals DefaultKeys), tui/keys_test.go (rebound and unbound keys, hints, quit, confirm answers, config save keeps keys, double click with open/attach unbound), tmux live test runs the attach setup on a real server (it caught `set-option -t =name` failing; fixed to `=name:`), vterm attach detaches on ctrl+g and not ctrl+q. Manual run of the built binary in tmux with `new = ["+"]`, `kill = ["x"]`, `auto_yes = []`, `detach = ["ctrl+g"]`: `n` did nothing, `+` opened the dialog, footer showed `+ new`, `x kill`, no auto-yes; under tmux and under Vineyard's server, ctrl+q stayed attached and ctrl+g detached; tmux status bar read `ctrl+g: back to vineyard`. A config binding `n` to kill showed `keys.list: "n" is bound to both kill and new` on the config screen. PASS. Screenshots: .grapes/58/tmp/{footer,attached,badconfig}.png.

### 2026-10-13T13:56
[DONE] Files: internal/config/keys.go (new), config.go, tui/common/keys.go, tui/app.go, backend.go, dialog, settings, list, preview, textview, diffview, tmux, vterm (protocol 2), README, docs. Out of scope: bubbles' text-editing keys, grapes' keys, editing keys on the config screen.

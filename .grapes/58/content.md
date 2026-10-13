## Goal
Every key Vineyard responds to can be rebound or unbound from
`.vineyard/config.toml`: the session list, the panes, the config screen, the
dialogs, quitting, and the key that detaches from an attached terminal.

## Description
Grapes, which Vineyard embeds, has a `[keys]` table with one key string per
action. Its design has gaps this issue avoids:

- One key per action, plus fallback keys hard-coded beside it (arrows,
  `ctrl+c`, `backspace`) that cannot be removed.
- Some actions are not configurable at all.
- No check that two actions on one screen share a key.
- Help text built from hard-coded strings.

Vineyard's design:

- `[keys.<mode>]` tables in `config.toml`; each action takes a list of keys
  that replaces its defaults. `[]` unbinds it. Actions left out keep their
  defaults, and `Save` writes back only what the user set.
- Modes: `global` (quit, detach), `list`, `pane`, `settings`, `form` (typing
  into the new-session dialog or a config value), `confirm`, `pick`.
- `config.Load` rejects an unknown mode or action, a malformed key name, a key
  bound to two actions of one mode or shadowed by `global.quit`, and a
  detach key other than `ctrl+` a letter, the way it rejects other bad
  config: defaults plus the error, shown on the config screen.
- Footer hints, the confirm dialog's answers, and "Press r to resume"
  messages show the configured keys, and leave out unbound actions.
- Mouse actions call the actions they stand for instead of faking key
  presses, so they keep working whatever the keys are.
- Detach: tmux binds the keys in a per-session key table at attach time, so
  the shared tmux server never mixes two configs; Vineyard's server gets the
  keys in the attach request.

## Context
- Bindings today: `internal/tui/common/keys.go` (`ListKeyMap`, `PaneKeyMap`,
  `SettingsKeyMap`); dialog keys in `internal/tui/dialog/dialog.go`
  (`keyCancel` … `keyToggle`, and string switches in `Confirm.Update`,
  `Pick.Update`); settings editing in `updateEditing`
  (`internal/tui/settings/settings.go`); `ctrl+c` at the top of `handleKey`
  (`internal/tui/app.go`).
- Hard-coded hints: `renderStatusBar` in `app.go`, `Hints` in dialog and
  settings.
- Faked key presses for the mouse: `handleMouse` and `enter` in `app.go`,
  wheel and picker clicks in `settings.go`.
- Detach: `tmux.DetachKey` bound with `bind-key -n` in `configureServer`
  (`internal/tmux/tmux.go`); `detachKey` byte in `internal/vterm/attach.go`.
  Changing `AttachRequest` requires bumping `protocol` in
  `internal/vterm/daemon.go`.
- Config: `internal/config/config.go` (`Config`, `Load`, `Save`).
- Out of scope: the text-editing keys inside input fields (cursor movement,
  word deletion), which are bubbles' defaults, and grapes' own keys on the
  issues screen, configured in `.grapes/config.toml`. The config screen does
  not edit keys; they are edited in the file.

## Acceptance Criteria
- [ ] Every action of every mode listed above has default keys equal to
      today's, and a `[keys.<mode>]` entry replaces them.
- [ ] An empty list unbinds an action, and its hint disappears.
- [ ] `config.Load` reports unknown modes and actions, bad key names,
      duplicate keys within a mode or with `global.quit`, and bad or missing
      detach keys.
- [ ] Saving from the config screen keeps the `[keys]` tables as written.
- [ ] Footer hints, confirm answers, and key mentions in messages follow the
      configured keys.
- [ ] Double clicks, wheel, and picker clicks work with the default actions
      unbound.
- [ ] Detaching works with a rebound key on tmux and on Vineyard's server.
- [ ] README documents the `[keys]` tables, modes, and actions.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Manual: build, add `[keys.list]` and `[keys.global]` overrides to a
throwaway repo's config, run Vineyard in tmux, use the rebound keys, check the
footer, attach and detach with the rebound detach key under both
multiplexers.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass. In the manual run the
rebound keys act, the old ones do not, and the footer shows the new keys.

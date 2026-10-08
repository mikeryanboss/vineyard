## Goal
Edit Vineyard's configuration from inside the TUI, the way grapes' Config
screen edits grapes' configuration, instead of editing
`~/.vineyard/config.toml` by hand.

## Description
Port the design of grapes' settings screen (`internal/tui/settings` in grapes):
a full-screen view opened with `C`, a category pane and a field pane,
short option lists that cycle on `enter`, longer ones in a picker overlay,
text fields edited in place, `ctrl+s` to save, `esc` to go back without saving.

The screen covers the settings Vineyard has today:
- **General:** default agent (one of the profiles), branch prefix, auto-yes for new sessions.
- **Profiles:** the agents offered for new sessions. Add a profile (name, then
  command), edit its command, remove it.

Saved settings apply to the running app at once: the next new session uses the
new default agent, profiles, branch prefix, and auto-yes.

Out of scope: theme presets and key rebinding, which grapes has but Vineyard
does not support yet.

## Acceptance Criteria
- [x] `C` opens the config screen; `esc` leaves it and discards unsaved edits.
- [x] General and Profiles fields can be changed with the keyboard, and `ctrl+s` writes `config.toml`.
- [x] Saved settings take effect for the next new session without restarting.
- [x] A config file that failed to load is never overwritten by a save.
- [x] Golden and interaction tests cover the screen; the suite stays under 10 s.
- [x] Docs describe the screen.

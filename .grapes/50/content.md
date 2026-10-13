## Goal
Offer the built-in profiles, `claude` and `terminal`, in every repository
unless its configuration disables them, so a config that lists its own
profiles still gets new built-ins such as `terminal`.

## Description
Since #49 the defaults hold `claude` and `terminal`, but a config file's own
`[[profiles]]` replace them. Configs saved by the config screen before #49
list `claude` alone, so `N` there offers one profile and shows no picker:
`n` and `N` look the same. This repository's `.vineyard/config.toml` is one.

Change:

- The built-in profiles live in `config.BuiltinProfiles`, not in `Defaults`.
- A `[[profiles]]` entry named like a built-in replaces it, whole; other
  entries add profiles after the built-ins, in file order.
- `disabled_profiles = ["terminal"]` stops offering the named profiles.
- `ResolvedProfiles` offers the built-ins (replaced as configured), then the
  config's other profiles, without disabled ones, default first. An
  unmatched `default_program` still comes first as a command of its own; one
  naming a disabled profile does not bring it back: the first offered
  profile is the default then.
- The config screen lists the built-ins with the config's profiles. `x` on a
  built-in disables or enables it instead of removing it, and the row says
  `disabled`. Disabling or removing the default moves the default to the
  first profile still offered; the last offered profile cannot go. Saving
  writes only what differs from the built-ins: replaced built-ins, added
  profiles, and `disabled_profiles`.

## Context
- `internal/config/config.go`: `Profile`, `Config`, `Defaults`,
  `ResolvedProfiles`.
- `internal/tui/settings/settings.go`: `New`, `fields`, `updateNavigating`
  (`keys.Remove`, `keys.Save`), `removeProfile`, `Hints`, `View`.
- `internal/tui/app.go`: `profiles` uses `ResolvedProfiles`.
- Docs: `README.md` configuration section.

## Acceptance Criteria
- [ ] A config listing only `claude` offers `claude` and `terminal`.
- [ ] A `[[profiles]]` entry named `terminal` replaces the built-in.
- [ ] `disabled_profiles` hides the named profiles; a disabled default is not offered.
- [ ] The config screen shows built-ins, toggles them with `x`, and saves only the differences.
- [ ] Saving and loading round-trip `disabled_profiles` and replaced built-ins.
- [ ] README describes built-ins, replacing, and disabling.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then run Vineyard in this repository, whose config lists only `claude`:
`N` shows the picker with `claude` and `terminal`; on the config screen, `x`
on `terminal` disables it, `ctrl+s` writes `disabled_profiles`, and `N`
then offers only `claude`.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass; the manual run behaves as
described, shown by screenshots in the PR.

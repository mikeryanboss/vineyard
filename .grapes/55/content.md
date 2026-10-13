## Goal
One key, `T`, opens a new terminal session and attaches to it, for quick
access to a shell to do something in.

## Description
Today a terminal takes `N`, picking the `terminal` profile, typing a title,
and `enter`, then `o` to attach. `t` opens only the selected session's own
shell, in its worktree, and needs a session. `T` skips all of that: it starts
a session of the `terminal` profile titled `terminal` and attaches to it once
it runs.

## Context
- `internal/tui/common/keys.go`: `ListKeyMap`, where `t` is `Shell`.
- `internal/tui/app.go`: `handleKey` handles `keys.New`/`keys.NewFromProfile`;
  `createSession` creates and starts a session from `common.NewSessionMsg`;
  `startedMsg` reports the start; `AttachCommand` and `attachDoneMsg` attach.
- `internal/config/config.go`: the built-in `terminal` profile runs `$SHELL` in
  the main checkout. A configured profile named `terminal` replaces it, and
  `disabled_profiles` can disable it; `ResolvedProfiles` gives what is offered.
- Session IDs carry a time suffix (`newID`), so several sessions titled
  `terminal` do not collide.

## Acceptance Criteria
- [x] `T` on the session list starts a session of the offered `terminal`
      profile, titled `terminal`, with no dialog.
- [x] Once that session runs, Vineyard attaches to it; a failed start attaches
      to nothing and shows the error.
- [x] With the `terminal` profile disabled, `T` starts nothing and says why.
- [x] The key hints, README key table, and docs mention `T`.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then in a real terminal: run `vineyard`, press `T`, see a shell in the main
checkout, `ctrl-q` back to the list with a `terminal` session selected.

## Pass Criteria
`gofmt -l .` prints nothing, vet and tests pass, and the manual run attaches
to a new shell.

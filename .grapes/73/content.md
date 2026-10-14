## Goal
Agents in different worktrees can run `scripts/drive` at the same time
without ending or typing into each other's development build.

## Description
`scripts/drive` names its tmux server `vineyard-drive` and Vineyard's
sockets `vineyard-drive-dev` in every checkout. `start` kills the server of
that name first, so one agent's `start` ends another agent's window, and its
`keys` and `type` go to the other agent's Vineyard. Observed while verifying
#72: another agent's `start` replaced the window, and the keys typed for the
rename went into its fresh Vineyard.

Each command of `scripts/drive` runs as its own process, so the names must
be derived, not drawn at random. Vineyard already gives each worktree a
unique directory name (a title slug plus random hex, such as
`rename-531d5a`), so the names end in the name of the checkout the script
belongs to: `vineyard-drive-<checkout>` and `vineyard-drive-dev-<checkout>`.

## Context
- `scripts/drive`: `outer=(tmux -L vineyard-drive)`, `socket=vineyard-drive-dev`;
  `start` runs `kill-server` on `outer`; `stop` kills `outer`, the tmux
  server on `socket`, and Vineyard servers matching `socket`.
- `docs/development.md`, Debugging: the `vineyard debug` example hard-codes
  `VINEYARD_SOCKET=vineyard-drive-dev`.
- Socket paths stay short: a worktree name is at most a 40-character slug
  plus a dash and hex digits, well under the 108-byte limit.

## Acceptance Criteria
- [x] `scripts/drive` run in two checkouts uses two tmux servers and two
      Vineyard socket names; `start` in one leaves the other's window running.
- [x] `stop` in one checkout leaves the other's window and servers running.
- [x] `docs/development.md` and the script's usage say how the names are formed.

## Verify
Run from the worktree root, with a second checkout `<other>` of this branch
(for example a `git worktree add --detach` under `.grapes/73/tmp/`):
```bash
go build -o .grapes/73/tmp/vineyard .
scripts/drive init .grapes/73/tmp/a vineyard
scripts/drive start .grapes/73/tmp/vineyard .grapes/73/tmp/a -p bash
(cd <other> && scripts/drive init ../b vineyard && scripts/drive start <binary> ../b -p bash)
tmux -L vineyard-drive-<this> ls; tmux -L vineyard-drive-<other> ls
(cd <other> && scripts/drive stop); tmux -L vineyard-drive-<this> ls
scripts/drive stop
```

## Pass Criteria
Both `tmux ls` calls list a `drive` session after both starts, and this
checkout's still does after the other's `stop`.

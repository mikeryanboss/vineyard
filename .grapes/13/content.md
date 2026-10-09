## Goal

Store each repository's vineyard configuration and session data in that repository, in `.vineyard/` like grapes' `.grapes/`, and let the configuration choose where session worktrees go.

## Description

Today everything lives under one home directory. The user wants it next to the code, with one committed config per repository like grapes. They decided:

- **Repo file only.** `.vineyard/config.toml` is the whole configuration. `~/.vineyard/config.toml` and `VINEYARD_HOME` go away.
- **No migration.** Vineyard reads only `.vineyard/`. Sessions under `~/.vineyard/projects/` drop out of the list; their tmux sessions and worktrees keep running until removed by hand.

Planned layout, in the main checkout:

```text
.vineyard/
  config.toml        committed; default_program, branch_prefix, auto_yes, profiles, worktree_dir
  .gitignore         written by vineyard when missing: ignore everything but itself and config.toml
  sessions.json      ignored
  lock               ignored
  worktrees/<id>/    ignored; the default worktree_dir
```

`worktree_dir` (default `.vineyard/worktrees`) is relative to the repository root unless it is absolute or starts with `~/`. Like `branch_prefix`, it applies to sessions created after it changes; each session keeps the absolute `WorktreePath` it was created with. The config screen edits it.

## Context

Verified against `main` at `2097516`:

- `internal/config/config.go`: `Home` (honours `VINEYARD_HOME`, default `~/.vineyard`), `Path(home)`, `ProjectDir(home, repoRoot)` (`projects/<base>-<hash>`), `Load(home)`, `Save(home, cfg)`. `Config` has `DefaultProgram`, `BranchPrefix`, `AutoYes`, `Profiles`.
- `main.go:runTUI` loads config from `Home`, locks and stores sessions in `ProjectDir`, and shows `config.Path(home)` on the config screen. `runDebug` prints those paths; `writeHelp` names `~/.vineyard/config.toml` and `VINEYARD_HOME`.
- `internal/session/manager.go:New` sets `WorktreePath` to `<ProjectDir>/worktrees/<id>` and `TmuxName` to `vineyard-<base of ProjectDir>-<id>`. The tmux name must stay unique per repository on the shared socket, so it keeps the `<base>-<hash>` project name rather than `.vineyard`.
- `internal/tui/backend.go:LiveBackend.SaveConfig` saves to `Home`.
- The `Repo.Root` of a linked worktree is the main checkout, so every checkout of a repository shares one `.vineyard/`.
- `go` and grapes' `.grapes` search both skip dot directories, so worktrees under `.vineyard/` are not walked by `go test ./...` or picked up as issue stores. A worktree directory has a `.git` file, so `git clean -fdx` in the main checkout leaves it alone unless `-f` is given twice.

## Acceptance Criteria

- [x] Vineyard reads and writes `.vineyard/config.toml`, `sessions.json`, and `lock` in the main checkout of the repository it runs in, from any of its worktrees.
- [x] Vineyard creates `.vineyard/.gitignore` when missing, so `git status` shows only `config.toml` and `.gitignore` as new.
- [x] New sessions get worktrees under `worktree_dir`: the default, a relative path, an absolute path, and a `~/` path all resolve as described; changing it does not move existing sessions.
- [x] The config screen shows and edits `worktree_dir` and saves to `.vineyard/config.toml`.
- [x] tmux session names keep the `vineyard-<repo>-<hash>-<id>` form.
- [x] `VINEYARD_HOME` and `config.Home` are gone; `vineyard debug` and `vineyard help` describe the new locations.
- [x] README and docs describe the layout, and the development guide no longer relies on `VINEYARD_HOME` to keep dev runs away from real sessions.

## Verify

Run from the repository root:

```bash
gofmt -l . && go vet ./... && go test ./...
grep -rn "VINEYARD_HOME\|config.Home\|ProjectDir(" --include=*.go --include=*.md --exclude-dir=.grapes .
```

Manual, in a real terminal on a throwaway repository: start vineyard, create a session, and check `.vineyard/` and `git status`; set `worktree_dir` to `../wt` on the config screen, create another session, and check where its worktree is.

## Pass Criteria

Formatting prints nothing; vet and tests pass; the grep prints nothing. In the manual run, the first worktree is under `.vineyard/worktrees/`, the second under `../wt/`, and `git status` lists only `.vineyard/.gitignore` and `.vineyard/config.toml` as untracked (after the config is saved).

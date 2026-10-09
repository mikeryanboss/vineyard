### 2026-10-09T11:20
[DECISION] From the user: config lives only in the committed `.vineyard/config.toml` (no global file), and existing sessions under `~/.vineyard/projects/` are not migrated.

### 2026-10-09T12:05
[DECISION] The TUI resolves `worktree_dir` when it creates a session, as it already reads `branch_prefix`, so a saved change applies to the next session; `session.NewOptions.WorktreeDir` carries the absolute result. An empty `worktree_dir` is refused on the config screen and by `config.ResolveWorktreeDir`, since it would put worktrees in the repository root.

### 2026-10-09T12:10
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass (4.3 s). New tests: `TestResolveWorktreeDir` (default, `../wt`, absolute, `~/`, empty refused), `TestPrepare_IgnoresAllButConfig`, `TestProjectName_DistinguishesSameNamedRepos`, `TestNew_PlacesWorktreeAndNamesTmuxSession`, `TestApp_NewSessionUsesWorktreeDir`, and `TestRun_DebugPrintsPaths` (in a temp repo; debug creates nothing). Session tests now create worktrees under `<repo>/.vineyard/worktrees`, nested in the main checkout. The stale-reference grep (`--exclude-dir=.grapes`) prints nothing. Golden files change only by the new config row and the fixture's save path. Manual, tmux 110x30, throwaway repo `.grapes/13/tmp/demo`: first session at `demo/.vineyard/worktrees/first-4ae9a2`; set Worktree directory to `../wt` and saved (`config.toml` holds `worktree_dir = '../wt'`); second session at `wt/second-11076a`, first unchanged; `git status --porcelain -uall` lists only `.vineyard/.gitignore` and `.vineyard/config.toml`; `vineyard debug` prints the `.vineyard` paths and `worktrees: <tmp>/wt`. PASS. Screenshot: `.grapes/13/tmp/config.png`, published in the PR.

### 2026-10-09T12:11
[DONE] `config`: `Dir`, `Prepare`, `Path(dir)`, `ProjectName`, `ResolveWorktreeDir`, `Config.WorktreeDir`; `Home`, `ProjectDir`, and `VINEYARD_HOME` removed. `session.Manager` takes the project name; `NewOptions.WorktreeDir` places worktrees. The config screen edits the worktree directory. README, help, debug, and docs describe `.vineyard/`.

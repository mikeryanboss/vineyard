### 2026-10-13T09:00
[DECISION] `n` runs the default profile with only a title; `N` picks a profile, first and focused, with no prompt (the user confirmed dropping it). A profile's `main_checkout` decides where it runs. The issue dialog filters out main-checkout profiles, since its prompt would run as shell commands. Attaching to an existing tmux session on the user's server is deferred: it needs a session picker, and a nested tmux client shares Vineyard's prefix key.

### 2026-10-13T09:00
[DECISION] `ResolvedProfiles` puts an unmatched `default_program` first as its own command. Without it, a config with `default_program = "aider"` and no profiles would start claude once the defaults hold profiles.

### 2026-10-13T09:00
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass (suite about 7s). In tmux on a scratch repository: `N`, `→` to terminal, a title, enter started `$SHELL` in the main checkout on `main`; commands ran in it; `c` and `s` were refused with a status message; `D` asked "Its program stops; the main checkout is left as it is." and killed it; `r` restarted a stopped one in the main checkout. Afterwards `git worktree list` showed only the main checkout and `git branch` only `main`. The config screen showed `$SHELL  in the main checkout`. PASS. Screenshots are in .grapes/49/tmp/*.png and in the PR.

### 2026-10-13T09:00
[DONE] Changed config (`Profile.MainCheckout`, default profiles, `ResolvedProfiles`), session (`Session.MainCheckout` plus its handling in the manager's lifecycle), the new-session dialog, the app's key handling and issue dialog, the config screen (`w`), README, and architecture docs.

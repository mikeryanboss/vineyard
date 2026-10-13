### 2026-10-13T10:50
[DECISION] Disabling is a top-level `disabled_profiles` list, not a field on a `[[profiles]]` entry. An entry named like a built-in replaces it whole. A bool in a partial entry could not tell "unset" from `false`, so `main_checkout` would have been lost when a disabled built-in was enabled again. `Load` refuses a config that disables every profile.

### 2026-10-13T10:50
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass. In tmux on a scratch repository with this repository's `.vineyard/config.toml` (`[[profiles]]` claude only): `N` offered claude and terminal. On the config screen, `x` on terminal showed `disabled`, and `ctrl+s` wrote `disabled_profiles = ['terminal']` with no `[[profiles]]`; `N` then offered claude alone, without a picker. `x` again re-enabled it, and saving wrote neither key. PASS. Screenshots are in .grapes/50/tmp/*.png and in the PR.

### 2026-10-13T10:50
[DONE] Changed `internal/config/config.go` (`BuiltinProfiles`, `IsBuiltin`, `AllProfiles`, `Disabled`, `Customized`, `ResolvedProfiles`, `DisabledProfiles`, the `Load` check), the config screen (built-ins in the list, `x` disables or enables them, save writes only the differences), README, architecture wording, and tests.

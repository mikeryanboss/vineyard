### 2026-10-08T20:58
[STARTED] Porting grapes' settings screen (`internal/tui/settings/settings.go`) as `internal/tui/settings`, limited to Vineyard's existing config fields.

[DECISION] The branch prefix moves from `session.Manager` to `session.NewOptions`, so a saved prefix reaches the next session without the TUI mutating the manager from a background command. Config is written through the `Backend` seam (`SaveConfig`), like every other side effect.

### 2026-10-08T23:10
[VERIFY] `gofmt -l .` clean; `go vet ./...` clean; `go test -count=1 ./...` passes in 2.1 s wall. PASS.
Real terminal (tmux, 120×30, throwaway repo and `VINEYARD_HOME` under `.grapes/3/tmp/`): added profile `shell` = `bash -l`, set it as default, set the branch prefix to `trial/`, saved; `config.toml` held all three. A new session in the same run got branch `trial/demo`; after a restart the new-session dialog offered `shell` first. With a malformed `config.toml`, ctrl+s showed "Not saved: … failed to load" and the file was unchanged. PASS. Captures: `.grapes/3/tmp/config-general.png`, `config-profiles.png`.

[DONE] New `internal/tui/settings` (screen and tests), `C` in the list keys, `SaveConfigMsg`/`CloseConfigMsg`, `Backend.SaveConfig`, `config.Save`, branch prefix moved to `session.NewOptions`, docs and README updated.

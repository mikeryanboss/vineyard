### 2026-10-09T09:45
[DECISION] Standalone grapes hands the terminal to `vineyard --issue <id>` instead of creating sessions itself: grapes #52 removed grapes' own tmux sessions because running agents is vineyard's job, and vineyard imports grapes, so grapes cannot call vineyard code. A vineyard already running for the repository holds the lock; `--issue` then fails with `session.ErrLocked`, which grapes reports.

### 2026-10-09T09:58
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass, including `TestStartIssue_*` (dialog waits for the first size, does not reopen on resize, jumps to the issue's session) and `--issue x` as a usage error. Built binary in a scratch repo (`.grapes/18/tmp/repo`, private tmux sockets): `--issue 9999` prints `Error: no grapes issue #9999`, exit 1; a repo without `.grapes` prints `Error: --issue: … has no .grapes directory`, exit 1. In a 120x32 tmux pane, `-p bash --issue 18` opened `New session for #18` filled in with the issue's title and prompt; creating it and relaunching with `--issue 18` selected the session `#18 …` with no dialog. PASS.

### 2026-10-09T09:59
[DONE] `main.go` adds `-i/--issue` and checks the issue before the TUI starts; `tui.Options.Issue` and `startIssue` run `showSessions` on the first window size. README, help, and `docs/architecture.md` describe it. Version 0.1.2.

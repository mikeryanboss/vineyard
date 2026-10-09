### 2026-10-09T12:20
[DECISION] Record the demo with real Claude Code agents in a throwaway repository (`doc/vhs/demo-repo.sh`, `/tmp/vineyard-demo`, tmux socket `vineyard-demo`). Agents on #2 and #3 start off camera and recording starts once both have diffs, so the GIF stays about 30 s. The repository lives outside the vineyard checkout so its agents do not load vineyard's CLAUDE.md.

### 2026-10-09T12:55
[FINDINGS] Prompt delivery to new Claude Code sessions is unreliable: the current trust prompt is not recognized and Enter picks "No, exit", the Enter after a paste sometimes becomes a newline, and once a prompt was lost. Filed as #24. `doc/vhs/demo-submit.sh` presses Enter on unsent prompts during the recording; a lost prompt makes the tape time out.

### 2026-10-09T12:58
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass. `vhs doc/vhs/demo.tape` (VHS, ttyd, ffmpeg, and JuliaMono as fallback font, installed under `.grapes/23/tmp/`) recorded `doc/demo.gif`: 30 s, 1.1 MB, showing two sessions' previews, a diff, an issue tab, the issues board, and a new agent started from issue #4. Of six takes, three failed to prompt delivery (#24). README keys and flags checked against `internal/tui/common/keys.go` and `main.go`. PASS.

### 2026-10-09T12:59
[DONE] Rewrote `README.md` around the major features and embedded `doc/demo.gif`. Added `doc/vhs/demo.tape`, `demo-repo.sh`, `demo-submit.sh`; documented re-recording in `docs/development.md` and added `doc/` to `docs/README.md`.

### 2026-10-09T13:12
[DECISION] Record in light mode, at the reviewer's request: the tape uses VHS's `Catppuccin Latte` theme, so Vineyard picks its light theme. Re-recorded `doc/demo.gif` (30 s, 1.2 MB) on the first take; all steps show as before.

### 2026-11-13T15:33
[STARTED] Write failing regression test `TestAttachKeepsColours` in `internal/vterm/vterm_test.go`, then fix `newScreen` in `internal/vterm/attach.go` to detect the color profile against the real `out *os.File` and `env`.

### 2026-11-13T16:01
[VERIFY] PASS.
- Before fix: `go test ./internal/vterm/ -run TestAttachKeepsColours -count=1 -v` failed (`attached render missing colour "\x1b[31m"`, `"38;2;10;20;30"`, `"48;5;"`).
- After fix: `gofmt -l .` empty; `go vet ./...` clean; `go test ./internal/vterm/ -run TestAttachKeepsColours -count=1 -v` PASS (0.03s); `go test -race -count=1 ./internal/vterm/...` PASS (4.35s); `go test -count=1 ./...` PASS (`internal/vterm` 2.00s, full uncached run 11.5s under concurrent devcontainer load; cached `go test ./...` <1s).
- Real TUI via `scripts/drive` on `.grapes/82/tmp/demo` (`vineyard` server, protocol 6): attached with `o` and ran `.grapes/82/tmp/swatch.sh`. `origin/main` build produced 0 ANSI SGR lines (`before.ansi`); this branch produced full ANSI 16, 256-color, TrueColor-downsampled, prompt, and status bar colors (`after.ansi`). Screenshots rendered to `.grapes/82/tmp/attached-before.png` and `.grapes/82/tmp/attached-after.png` and published on `pr-evidence` at `36b82efd4d085cfb6fedb880ce9e7f00a7ddc41e`.

### 2026-11-13T16:01
[DONE] `internal/vterm/attach.go` detects the viewer's terminal color profile against `out *os.File` and `env` via `colorprofile.Detect` while buffering frames in `*bytes.Buffer`; `internal/vterm/vterm_test.go` adds `TestAttachKeepsColours`; `go.mod` marks `github.com/charmbracelet/colorprofile` direct.

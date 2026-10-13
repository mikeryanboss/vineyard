### 2026-10-13T16:45
[DECISION] The user wants attaching to work in general for future updates, with no workaround for servers that exist now. The older server's own executable attaches (`/proc/<pid>/exe attach <socket> <terminal> <json>`), so no old protocol code is kept. The contract is the command's arguments and `session.Attachment`'s JSON names (now explicit tags); JSON ignores unknown fields, so the attachment can grow. Builds before the command exit 2 (usage), reported as predating it; the attach command exits 1 on failure. Linux only: macOS has no `/proc`, and attaching to an older server fails there with an error saying so. The status bar now lists only peers of the same protocol, which the attached server can sample.

### 2026-10-13T16:45
[FINDINGS] An attachment from a terminal of size 0x0 hung instead of drawing (seen while writing the test, with an unsized pty). Real terminals always have a size; not changed here.

### 2026-10-13T16:45
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` passes. New: `TestAttachThroughServersExecutable` attaches through `/proc/<pid>/exe` of a real test server (the test binary handles `attach` in TestMain), types, and detaches with ctrl-g. PASS.

### 2026-10-13T16:45
[VERIFY] Real app via `scripts/drive`: a build of this branch with `protocol` set to 2 started a session; this branch's build (protocol 3) showed it, attached through `/proc/688756/exe attach /tmp/vineyard-170285/vineyard-drive-dev-p2 …` with the status bar, `echo via-old-binary` ran, ctrl+q returned to the list. A session from a build of 73d94db: attaching showed `attach: use the Vineyard that started it; … (process 689903, protocol 1), whose executable predates attaching through it`. Screenshot: .grapes/65/tmp/attached.png (published with the PR). PASS.

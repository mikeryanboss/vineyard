### 2026-10-13T15:45
[FINDINGS] The reported "crash" was gone when investigated; nothing recorded it. Reproducing on main in a throwaway repository (bash and claude sessions, attach, shell, T, quit, restore) found no failure. The user's server, process 468432, had an empty log. `vineyard debug` from this branch then showed at once that the server speaks protocol 1 while main speaks 2 (#58 raised it): see #60.

### 2026-10-13T15:45
[DECISION] Diagnosis starts from records, not reproduction: the server log gets lifecycle and exit records, the TUI gets `.vineyard/vineyard.log`, and `vineyard debug` summarizes both with the server's and sessions' state. Logs restart past 1 MB (`internal/logfile`), keeping one `.old`. A headless session CLI was left out: tests against a real server reproduce the same paths more precisely. Panics are logged from `Update` and `View` only; a panic in a command's goroutine still reaches only the terminal.

### 2026-10-13T15:45
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` passes in 7.1 s wall. New tests: `TestInfoAndLogDescribeTheServer` (Info against a real server and a fake of another protocol; exit record with status 3 and `boom`), `TestApp_LogsErrorsAndPanics`, `TestOpen_StartsALargeLogAfresh`, `TestRun_DebugPrintsPaths` (no server, log path). PASS.

### 2026-10-13T15:45
[VERIFY] Real app via `scripts/drive`: a session whose program is `echo boom; exit 3` on Vineyard's server; `vineyard debug` showed the server running, protocol 2, the session `stopped, terminal gone`, and the server log ending `exited after 2ms: exit status 3 ... its screen ended: boom`. Output: .grapes/59/tmp/debug.txt (not committed). PASS.

### 2026-10-09T10:35
[DECISION] First release is v0.1.1 (user's choice): 0.1.0 was never tagged, and auto-tag needs `main.go` to change. Windows is left out because sessions need tmux.

### 2026-10-09T10:45
[VERIFY] GoReleaser v2.18.3: `goreleaser check` validated `.goreleaser.yaml`. A snapshot release built `vineyard_0.0.0-SNAPSHOT-9d6fc18_{linux,darwin}_{amd64,arm64}.tar.gz` and `checksums.txt`, and the linux/amd64 binary printed `0.0.0-SNAPSHOT-9d6fc18`. The release criterion can only be checked after merge. PASS

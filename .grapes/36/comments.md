### 2026-10-11T21:50
[FINDINGS] Auto Tag created v0.1.1, v0.1.2, and v0.1.3 (at the merges of PRs #11, #12, #22), but `gh run list --workflow release.yml` prints `[]` and `gh release list` prints nothing: a tag pushed with `GITHUB_TOKEN` starts no workflow. #31–#35 changed no grapes code, so `go.mod` stays on grapes v0.1.14.

### 2026-10-11T21:50
[DONE] `main.go`: version 0.1.4. `auto-tag.yml`: job `tag` outputs `created`; new job `release` calls `release.yml` when it is true. `release.yml`: also `workflow_call`. `docs/development.md`: Releases. The release itself is checked after merge (last criterion).

### 2026-10-11T21:50
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test -count=1 ./...` passed in 6.2 s. `.grapes/36/tmp/vineyard version` printed `0.1.4`. actionlint v1.7.12 on `.github/workflows/*.yml` printed nothing; with `workflow_call` removed from `release.yml`, it reported `"workflow_call" event trigger is not found`, so it checks the call. PASS, except the release after merge, which is unverified.

### 2026-10-11T22:00
[VERIFY] After merge (PR #28, `ce9add4`): Auto Tag run 38169821455 succeeded, with jobs `tag` and `release / release`. Tag v0.1.4 exists, and release v0.1.4 lists `checksums.txt` and `vineyard_0.1.4_{darwin,linux}_{amd64,arm64}.tar.gz`. The downloaded `vineyard_0.1.4_linux_amd64.tar.gz` passed `sha256sum -c checksums.txt` and printed `0.1.4` for `vineyard version`. PASS.

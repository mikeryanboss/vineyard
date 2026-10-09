### 2026-10-09T11:05
[VERIFY] `go get github.com/Mibokess/grapes@v0.1.11 && go mod tidy`; `grep -n Mibokess/grapes go.mod` shows `v0.1.11`; `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass. PASS.

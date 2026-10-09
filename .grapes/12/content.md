## Goal

Build vineyard against the released grapes v0.1.11 instead of the pre-release commit #9 pinned.

## Context

- `go.mod` requires `github.com/Mibokess/grapes v0.1.11-0.20261009081705-50b5efb3d664`, the head of Mibokess/grapes#37 before it merged.
- Grapes#37 merged as `22a0c5b`, and tag `v0.1.11` exists.

## Acceptance Criteria

- [x] `go.mod` requires `github.com/Mibokess/grapes v0.1.11`.
- [x] The build and tests pass.

## Verify

Run from the repository root:

```bash
grep -n Mibokess/grapes go.mod && gofmt -l . && go vet ./... && go test ./...
```

## Pass Criteria

`grep` shows `v0.1.11`; formatting prints nothing; vet and tests pass.

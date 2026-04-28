# vineyard

Vineyard is a Grapes-driven orchestration runtime for async coding agents. It keeps Grapes as the durable work ledger, creates strict per-issue execution spaces, delegates implementation to Pi, and lets verifier/policy code own final status transitions.

The implementation follows a Pi-like shape: a small SDK factory with defaults, narrow injectable services, file-backed state, and a thin CLI.

## Status

This repository currently contains the first runtime foundation:

- `createOrchestrator(options = {})`
- file-backed Grapes store for issues and orchestration sidecars
- strict git worktree manager
- Chokidar watcher
- Pi RPC runner seam
- conservative verifier default
- review-label policy gate
- Node test coverage for the core status flow

## Usage

```bash
npm install
npm run build
npm exec vineyard reconcile --once
npm exec vineyard run
```

Issues are dispatched only when their Grapes status is `todo` and blockers are terminal. The runtime writes `claim.toml`, `run.toml`, `worktree.toml`, and `review.md` beside the issue.

## Implementation Notes

See [docs/initial-implementation.md](docs/initial-implementation.md) for the full implementation record, service boundaries, file state, status flow, tests, and current limitations.

# Initial Implementation

This document records the first Vineyard implementation slice delivered for Grapes issue #1.

## Goal

The implementation establishes a simple, Pi-inspired orchestration foundation:

- small SDK entrypoint with defaults
- lifecycle runtime that owns start, stop, dispatch, and reconciliation
- injectable service interfaces
- file-backed Grapes state
- conservative default policy and verifier behavior
- thin CLI over the runtime

The first slice is intentionally not a full autonomous production system. It creates the clean boundaries needed to add richer scheduling, verification, continuation, and operator controls without rewriting the core.

## Public Shape

The primary SDK surface is:

```ts
createOrchestrator(options = {})
```

It returns an `OrchestratorRuntime` with:

- `start()`
- `stop()`
- `reconcile(reason?)`
- `enqueueReconcile(reason)`
- `eventBus`

The CLI is a thin wrapper:

```bash
vineyard reconcile --once
vineyard run
```

## Core Services

All meaningful subsystems are behind interfaces in `src/types.ts`:

- `GrapesStore`
- `WorkflowLoader`
- `WorktreeManager`
- `AgentRunner`
- `Verifier`
- `ReviewPolicy`
- `FileWatcher`
- `EventBus`

Defaults are provided, but callers can inject alternatives for tests or deployment-specific behavior.

## Default Implementations

The initial defaults are:

- `DefaultGrapesStore`: loads Grapes issues from `.grapes/<id>`, mutates statuses, appends comments, and writes sidecars.
- `DefaultWorkflowLoader`: loads `WORKFLOW.md` with optional simple YAML front matter.
- `GitWorktreeManager`: creates one git worktree per issue under `.worktrees/issue-<id>`.
- `ChokidarFileWatcher`: watches Grapes files and workflow files.
- `PiRpcRunner`: starts Pi in RPC mode inside the issue worktree and expects an `implementation.md` handoff artifact.
- `ConservativeVerifier`: uses a worktree-local `verification.md` if present, otherwise forces `review`.
- `DefaultPolicy`: dispatches only `todo` issues with terminal blockers and forces `review` for sensitive labels.
- `createEventBus`: minimal typed event bus for lifecycle and observability events.

## File State

The runtime writes issue-local orchestration files beside the Grapes issue:

- `claim.toml`: current machine phase, attempt, owner, worktree, and session handles
- `run.toml`: current implementation run metadata
- `worktree.toml`: canonical worktree path and branch
- `review.md`: readable proof-of-work or verification packet

The Pi worker is instructed to write this worktree-local artifact:

```text
<worktree>/.grapes/<id>/implementation.md
```

The verifier may write:

```text
<worktree>/.grapes/<id>/verification.md
```

## Status Flow

The first implemented flow is:

1. Load issues from Grapes.
2. Select eligible `todo` issues.
3. Mark the issue `in_progress`.
4. Ensure the issue worktree.
5. Run the implementation agent through the runner seam.
6. Record implementation metadata.
7. Run verification.
8. Apply review policy.
9. Write `review.md`.
10. Set final Grapes status to `done`, `review`, `todo`, `backlog`, or `cancelled`.

On runtime failure, the issue is returned to `todo`, a comment is appended, and `claim.toml` records `failed`.

## Tests

The test suite covers:

- flat TOML parsing/stringifying for Grapes metadata
- dispatch of a `todo` issue through fake worktree, runner, and verifier services
- review-label override from verifier `done` to Grapes `review`
- blocker policy for unfinished dependencies

Run:

```bash
npm test
```

## Current Limitations

This slice intentionally leaves several production concerns for later issues:

- no concurrency limiter beyond the current in-memory running map
- no durable run history JSONL yet
- no strong-model continuation decision loop yet
- no live Pi end-to-end integration test
- no container or sandbox policy integration
- no PR or merge workflow
- no recursive planning/decomposition flow

The implementation keeps those paths open through service interfaces instead of baking in policy too early.

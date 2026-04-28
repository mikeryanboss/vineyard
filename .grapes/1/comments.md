### 2026-04-28T21:41
[FINDINGS] Deep familiarization pass:

- Local repo has no implementation yet; the meaningful sources are `.grapes/1/content.md` and `idea.md`.
- Issue #1 refines `idea.md` from a broad Symphony/Pi/Grapes sketch into a sharper conceptual contract: Grapes is the durable ledger, the orchestrator owns scheduling/state transitions, work happens only in strict per-issue worktrees, and Pi is the phase runtime.
- The issue intentionally separates human-facing Grapes status from issue-local machine phase. That separation is important because Grapes should remain readable and durable while orchestration details live in sidecar/artifact files.
- Upstream Symphony docs support the same broad shape: issue tracker as control plane, deterministic per-issue workspaces, repo-owned `WORKFLOW.md`, bounded orchestration, restart recovery from tracker/filesystem state, and implementation-defined safety posture.
- Current Pi docs support the required session strategy: RPC/SDK embedding, persisted JSONL sessions, session file/session id exposure, CLI resume/fork by session file or partial id, and runtime-level session replacement for new/resumed/forked sessions.
- The key unresolved design choices before implementation are: whether `review` becomes a first-class Grapes status, which canonical sidecar schemas own claim/run/worktree/session/phase state, whether agents may edit `meta.toml` directly, and how strict the verifier/orchestrator boundary should be for final status mutation.

### 2026-04-28T21:49
[FINDINGS] Follow-up familiarization pass:

- Issue #1 is now the canonical architecture contract; `idea.md` is broader background and contains earlier alternatives that the issue mostly resolves.
- The strongest invariant is authority separation: Grapes is the human-readable ledger, the orchestrator is the scheduler/state machine, Pi is the execution runtime, and agents are disposable phase workers.
- The issue deliberately diverges from stock Symphony in one important way: status/comment mutation should be mediated by orchestrator/verifier policy rather than left primarily to the worker prompt.
- Implementation should start by locking schemas for `.grapes/<id>/claim.toml`, `run.toml`, `worktree.toml`, and worktree-local phase artifacts before building the watcher loop.
- Current Chokidar docs matter for the first implementation: v5 is ESM-only and requires Node 20+, and v4 removed glob support, so watcher path selection should use explicit paths or filtered directory watches.
- `review` remains the highest-impact prerequisite decision because Grapes' documented status enum currently lacks it while issue #1 treats it as part of the human-facing workflow.

### 2026-04-28T21:52
[FINDINGS] Pi-style implementation investigation:

- Yes, the orchestrator can be implemented in a Pi-like style, but the useful pattern is not Pi's full package/plugin system. The useful pattern is a small public factory with defaults, narrow replaceable interfaces, simple file-backed state, and extension hooks layered on after the core loop works.
- Pi's core shape is: CLI parses options, SDK/runtime owns the real work, default services are created when callers do not inject their own, and runtime replacement is centralized. The orchestrator should mirror that with `createOrchestrator(options = {})`, default Grapes/worktree/Pi/watcher/policy services, and one runtime owner for start/stop/reconcile.
- Pi's `ResourceLoader` is the right inspiration for workflow/config loading: expose a simple interface first, default it to `WORKFLOW.md` plus local settings, and leave package discovery or third-party extension loading for a later milestone.
- Pi's `EventBus` is intentionally tiny. The orchestrator should start with the same kind of minimal internal event bus for observability and hooks rather than introducing a workflow engine.
- Pi sessions are append-only JSONL trees. Grapes orchestration should not copy the tree model unless needed, but it should copy the durability principle: current sidecar TOML for latest phase plus optional append-only JSONL event/run history for audit and restart diagnosis.
- First extension points should be typed service interfaces and policy hooks, not arbitrary npm package loading: tracker adapter, worktree manager, runner, verifier/continuation decider, review policy, and lifecycle hooks such as `beforeDispatch`, `afterWorkerArtifact`, `beforeStatusTransition`, and `onFailure`.
- Avoid copying Pi-owned concerns into the orchestrator: model provider registry, tool registry, prompt/skill packages, session branching UI, terminal UI, and command system. The orchestrator should delegate those to Pi and stay focused on Grapes state, worktrees, scheduling, and verification policy.

### 2026-04-28T22:06
[FINDINGS] Initial implementation completed:

- Added a Node 20+/TypeScript ESM package with `createOrchestrator(options = {})`, `OrchestratorRuntime`, strict compiler settings, and a thin `vineyard` CLI.
- Implemented the Pi-style service boundary: injectable Grapes store, workflow loader, file watcher, worktree manager, agent runner, verifier, policy, and event bus.
- Implemented file-backed Grapes orchestration state: issue loading, status mutation, comments, `claim.toml`, `run.toml`, `worktree.toml`, and `review.md` packets.
- Implemented default services: Chokidar watcher, git worktree manager, Pi RPC runner, conservative verifier, and review-label policy gate.
- Added tests for TOML metadata parsing and the main dispatch/status path, including review-label override from verifier `done` to Grapes `review`.
- Verification: `npm test` passes after installing dependencies. The initial sandboxed install failed with registry DNS `EAI_AGAIN`; approved network install succeeded.

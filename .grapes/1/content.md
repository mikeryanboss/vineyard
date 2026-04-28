## Goal

Define the conceptual model for an async agent orchestration system that uses Grapes as the task ledger, strict per-issue worktrees as execution spaces, Pi as the agent runtime, and a separate orchestrator as the scheduler and state machine.

The system should let humans or agents create Grapes issues in `backlog`, allow humans or trusted agents to promote issues to `todo`, and then have the orchestrator execute, verify, and resolve the work without requiring direct communication between agents.

## Core Idea

Grapes remains the source of truth for work.

The orchestrator watches Grapes issues and treats issue status as the high-level workflow state. When an issue becomes eligible, the orchestrator creates or reuses a strict per-issue worktree, runs an implementation agent there, waits for file-based completion artifacts, then runs a stronger verification agent to decide the outcome.

Agents do not chat with each other. They communicate asynchronously by reading and writing files associated with the issue and its worktree.

## Conceptual Components

### Grapes

Grapes is the durable, human-readable task ledger.

It owns:

- issue identity
- title, description, and comments
- status
- priority
- labels
- parent/child relationships
- blockers

Grapes does not need to understand agent orchestration internally. The orchestration tool may add issue-local machine files next to the normal Grapes files.

### Orchestrator

The orchestrator is the only scheduler and phase controller.

It owns:

- detecting eligible issues
- claiming work
- creating strict worktrees
- launching agent jobs
- detecting completion or failure
- scheduling verification
- enforcing review policy
- applying final status transitions

The orchestrator should be restartable and able to recover state from files.

### Worktrees

Every executable job happens inside a dedicated issue worktree.

No planner, implementation agent, verifier, repair agent, or landing agent should operate in the main checkout.

Each issue has one canonical worktree. All attempts for that issue use that worktree unless the orchestrator deliberately recreates it.

Worktrunk may be used as the worktree management layer.

### Agents

Agents are phase workers, not long-lived peers.

Primary roles:

- planner: breaks large or ambiguous work into Grapes issues
- implementer: performs bounded work in the issue worktree
- verifier: evaluates whether the result satisfies the issue
- landing/review agent: optional later role for PR, CI, and merge workflows

Cheaper models may do implementation work. A stronger model owns verification and high-level decisions.

## Human-Facing State Model

Use Grapes statuses for high-level workflow only:

```text
backlog      issue exists but is not approved for automation
todo         issue is approved and eligible for orchestration
in_progress  orchestrator owns active execution or verification
review       machine believes work is complete but human review is required or prudent
done         accepted as complete
cancelled    no further automation should run
```

If Grapes does not yet support `review`, add it before relying on this workflow.

## Machine Phase Model

Use issue-local machine state for orchestration phases:

```text
idle
claimed
worktree_ready
implementing
awaiting_verification
verifying
awaiting_rework
complete
failed
```

The machine phase should not replace the Grapes status. Grapes status remains the board-level view.

## Async Handoff Model

Implementation agents do not call verifier agents.

Instead, an implementation agent writes a completion artifact such as:

- summary of work
- changed files
- tests run
- known risks
- whether it believes the issue is ready for verification

The orchestrator detects that handoff and schedules verification.

The verifier reads:

- the original issue
- comments
- implementation artifact
- git diff
- test output
- relevant code

Then the verifier writes a decision artifact. The orchestrator applies the allowed status transition.

## File Watching Model

The first implementation should use Chokidar directly.

The orchestrator watches:

```text
.grapes/
WORKFLOW.md
```

It reacts to relevant files only:

```text
.grapes/<id>/meta.toml
.grapes/<id>/content.md
.grapes/<id>/comments.md
.grapes/<id>/claim.toml
.grapes/<id>/run.toml
.grapes/<id>/worktree.toml
WORKFLOW.md
```

For active issue worktrees, the orchestrator dynamically watches phase artifacts:

```text
<worktree>/.grapes/<id>/implementation.md
<worktree>/.grapes/<id>/question.md
<worktree>/.grapes/<id>/triage.md
<worktree>/.grapes/<id>/continuation.md
<worktree>/.grapes/<id>/verification.md
<worktree>/.grapes/<id>/review.md
```

File changes should enqueue small debounced reconciliation tasks. The watcher is the primary trigger. The implementation does not need a generic watcher backend or external watch service.

## Pi Session Model

The orchestrator should use Pi as the agent runtime and should store Pi session file paths for every phase.

Pi supports persisted JSONL sessions. Sessions can be opened again by file path through the SDK, and the CLI can resume sessions by path or partial id. The orchestrator should record the session file path because it is the most precise durable handle.

Example machine state:

```toml
implementation_session_file = "/home/user/.pi/agent/sessions/...jsonl"
implementation_session_id = "..."
verifier_session_file = "/home/user/.pi/agent/sessions/...jsonl"
verifier_session_id = "..."
```

Use one Pi session per phase by default. Reuse or resume a session only when the strong model explicitly asks for that continuation strategy.

## Strong-Model Continuation Decisions

The strong model does not directly spawn processes or talk to other agents.

Instead, when a worker finishes, fails, asks a question, or appears stuck, the orchestrator runs the strong model to decide what should happen next. The strong model writes a structured continuation decision. The orchestrator validates and executes that decision.

Continuation options include:

```text
same_live_session
same_session_new_turn
new_session_same_worktree
new_session_clean_worktree
escalate_to_strong_implementer
human_required
split_issue
cancel
```

Initial implementation should focus on:

```text
same_session_new_turn
new_session_same_worktree
new_session_clean_worktree
human_required
```

Example continuation artifact:

```toml
decision = "answer_and_continue"
continuation = "same_session_new_turn"
target_role = "implementer"
model_tier = "cheap"
human_required = false
reset_worktree = false
```

```markdown
## Message To Implementer

Use OIDC only. Do not implement SAML. Continue from the existing diff.

## Context To Preserve

- Keep the current auth callback changes.
- Add tests for missing state parameter.

## Context To Ignore

- Drop the SAML branch started in the previous attempt.
```

The orchestrator executes the continuation by reopening the stored Pi session file, starting a fresh session in the same worktree, or recreating the worktree depending on the selected strategy.

## Question Triage

If a cheaper worker needs clarification, it should write a durable question artifact rather than prompting a human directly.

Example:

```text
<worktree>/.grapes/<id>/question.md
```

The orchestrator detects the question and runs the strong model for triage.

The strong model may decide to:

- answer the question and continue with the same worker session
- answer the question and start a fresh worker session in the same worktree
- revise the issue or add comments, then continue
- split the work into child Grapes issues
- require human intervention
- cancel or defer the issue

Human intervention should only happen when the strong model explicitly decides that human input is required. In that case, the orchestrator should move the canonical issue to a human-visible blocked state such as `backlog` or `review`, add a label such as `needs-human-answer`, and append the question to canonical `comments.md`.

## Completion Policy

Implementation agents may say "ready for verification," but they do not decide canonical completion.

The verifier may decide:

```text
done       acceptance criteria are satisfied and no review gate applies
review     work appears complete but human review is required or prudent
todo       more implementation work is needed
backlog    issue needs clarification, decomposition, or replanning
cancelled  issue should no longer be pursued
```

Labels can force review, for example:

```text
requires-human-review
security
auth
billing
migration
public-api
data-loss-risk
production-config
```

Human review policy is enforced by the orchestrator, even if the verifier recommends `done`.

## Planning And DAGs

Issues may represent planning, investigation, implementation, verification, cleanup, or migration stages.

Large issues can be decomposed into child Grapes issues.

Grapes `parent` and `blocked_by` should be used to model a task DAG. Agents may create new Grapes issues directly using normal Grapes files. New agent-created issues should default to `backlog` unless policy explicitly allows direct promotion to `todo`.

## Review Packet

When work reaches `review` or `done`, the system should leave a readable proof-of-work packet.

It should summarize:

- what changed
- why it satisfies the issue
- tests and checks run
- known risks
- skipped checks
- follow-up issues created
- PR or merge information, if applicable

## Non-Goals

This design does not require Grapes to become an orchestrator.

This design does not require direct agent-to-agent communication.

This design does not require automatic merging in the first version.

This design does not require a database for the first version.

This design does not depend on specific model names. It depends only on model roles: stronger high-level model and cheaper implementation model.

## Conceptual Acceptance Criteria

- The responsibilities of Grapes, the orchestrator, worktrees, and agents are clearly separated.
- The human-facing Grapes status model is defined.
- The machine phase model is defined separately from Grapes status.
- The async file-based handoff model is defined.
- The file watching model is defined with Chokidar as the initial implementation mechanism.
- The Pi session model is defined, including durable storage of session file paths.
- The strong-model continuation decision model is defined.
- Worker question triage through the strong model is defined.
- The completion authority of the verifier/main model is defined.
- Human-review policy via labels is defined.
- Strict per-issue worktree semantics are defined.
- Planning and child issue/DAG behavior is defined.
- Follow-up issue creation uses normal Grapes issue creation rather than a special orchestration tool.

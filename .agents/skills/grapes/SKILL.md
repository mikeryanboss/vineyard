---
name: grapes
description: "Foundational reference for the Grapes file-based issue tracker. Use when working in a project with a .grapes/ directory."
user-invocable: false
---

# Grapes — File-Based Issue Tracker

Issues are plain files in `.grapes/`, with no database. Edit them with file tools; `grapes issue` manages ID allocation and timestamps.

## Structure

```
.grapes/<id>/
  meta.toml       # status, priority, labels, dates
  content.md      # issue description (markdown)
  comments.md     # append-only comment log
```

IDs are numeric folder names. The folder listing is the index, not an ID allocator: other worktrees may hold additional issues.

## Creating and Updating Issues

- `id=$(grapes issue)` — allocate the next ID across configured worktrees, create its directory, and set timestamps. Never choose an ID by hand.
- Read the newly created files before editing them. Populate the title, status, priority, and specification; the allocator does not supply the issue's meaning.
- New issues start in `todo`. Creating an issue does not mean starting it.
- Run `grapes issue <id>` once after each logical update to an issue's files, then `grapes validate <id>`. Never write `created` or `updated` manually.
- Issue files are tracked by Git. Commit them with the corresponding work using the `#<id>: Imperative description` convention.

## Lifecycle

```
backlog ──> todo ──> in_progress ──> done
                                 ──> cancelled
```

- **backlog**: parked. Nobody has scheduled it; do not start it.
- **todo**: ready for implementation.
- **in_progress**: being worked on. Set it before editing code.
- **done**: complete and verified. Set it on the branch, in the PR that completes the issue, not after the merge.
- **cancelled**: abandoned.

Check off each acceptance criterion in `content.md` (`- [ ]` → `- [x]`) as soon as it is satisfied, so the checkboxes always show current progress.

## meta.toml Schema

```toml
title = "Short description of the issue"
status = 'todo'
priority = 'high'
labels = ['bug', 'auth']
parent = 40
blocked_by = [19, 20]
# created and updated are supplied by grapes issue; retain its values.
```

### Field Values

- **status**: `backlog`, `todo`, `in_progress`, `done`, `cancelled`
- **priority**: `urgent`, `high`, `medium`, `low`
- **labels**: TOML list of freeform tags
- **parent**: numeric ID of parent issue (omit for top-level issues)
- **blocked_by**: TOML list of prerequisite issue IDs (omit if none). Store dependencies here rather than duplicating an inverse `blocks` field.
- **created** / **updated**: managed by `grapes issue`. Do not write manually.

## comments.md Format

```markdown
### 2026-02-27T09:15
Comment body here. Can be multiple lines.

### 2026-02-28T14:30
Another comment.
```

- Header: `### YYYY-MM-DDTHH:MM`
- Append-only. Never edit or delete existing comments.

Start each comment body with a tag:

| Tag          | When                 | Content                                           |
| ------------ | -------------------- | ------------------------------------------------- |
| `[STARTED]`  | Beginning work       | Brief description of approach                     |
| `[PROGRESS]` | Intermediate update  | What's done, what's next                          |
| `[FINDINGS]` | Research results     | Files found, patterns, data                       |
| `[DECISION]` | Choice made          | Decision + rationale                              |
| `[VERIFY]`   | Verification results | Command, concise result, PASS/FAIL, evidence path |
| `[BLOCKED]`  | Can't proceed        | Blocker, what was tried, what's needed            |
| `[DONE]`     | Task complete        | Files changed, summary                            |

- Record decisions, meaningful progress, blockers, and verification results, not individual tool calls or unchanged status.
- Keep entries concise and reference full logs and artifacts instead of copying them.
- Group related updates from one event, then run `grapes issue <id>` and `grapes validate <id>` once.

## Ownership With Several Agents

- The lead maintains the parent issue; each subagent maintains its assigned sub-issue. Do not edit another active agent's issue files.
- When several agents share one issue, the lead maintains its files and promptly records the findings, decisions, blockers, and verification the others report.
- Another worktree's copy of an issue may be stale. Use agent messages for progress before integration, and commit tracker updates with the corresponding code changes rather than mirroring them across worktrees.
- A subagent with a named lead or integrator hands its findings and verification to that agent, which closes the issue out.

## Reading and Editing Issues

- **Discovery:** search metadata first, then candidate specifications. Search comments when the information sought may be recorded there.
- **Taking ownership:** on first taking responsibility for an issue, read its metadata, specification, and comment history. Comments may contain decisions or corrections absent from the specification.
- **Large histories:** use available size information to choose bounded reads; page through rather than silently skipping older comments.
- **Specific lookups:** use targeted searches over metadata, specifications, or comments, then read surrounding context.
- **Subsequent reads:** reuse established context and inspect new or changed material; revisit earlier sections when needed.
- **Surgical edits:** change the relevant field in `meta.toml`; do not rewrite unrelated fields.
- **The filesystem is the database:** use targeted search and file tools to query and update issues.

## Authoring Issues

Implementation issues must be self-contained and ready to execute: describe the goal, verified current behavior, proposed changes, constraints, and how success will be checked. Research issues instead state the question, known evidence, scope, expected deliverable, and stopping condition.

| Section | Requirements |
|---------|--------------|
| Goal | One clear deliverable and why it matters. |
| Description | Explain the change and motivation. |
| Context | Relevant repository-relative paths, symbols, current versus desired behavior, dependencies, and constraints. |
| Acceptance Criteria | Binary pass/fail outcomes covering the scope and relevant edge cases. |
| Verify | Exact commands with their working directory, or explicit manual verification steps where commands cannot establish the result. |
| Pass Criteria | Expected output or behavior that makes success unambiguous. |

Before implementation:
- Verify referenced existing paths, symbols, behavior, and relationships against the code. Mark new paths and symbols as planned; they need not exist yet.
- Distinguish verified facts from proposed changes. Resolve material implementation questions rather than hiding them behind "as appropriate" or "TBD"; research questions may remain explicitly open in research issues.
- Replace vague goals or implicit references with enough relevant detail to execute independently.
- Ensure verification actually exercises each acceptance criterion.

### Template

````markdown
## Goal
[Deliverable and motivation]

## Description
[What changes and why]

## Context
[Relevant paths, symbols, current/desired behavior, constraints, and dependencies]

## Acceptance Criteria
- [ ] [Observable pass/fail outcome]

## Verify
Run from [working directory]:
```bash
[Exact command(s)]
```

## Pass Criteria
[Exact expected output or behavior]
````

### Sub-Issues and Dependencies

- Split independently testable changes, unrelated modules, or work with a natural dependency order. Keep a single cohesive change together.
- Each sub-issue must stand alone with its required context and verification; do not substitute "see parent for context."
- The parent holds shared context and overall acceptance criteria, not copies of child specifications.
- The children together cover the parent scope without omissions or unrelated additions.
- Set `parent = <id>` on children and `blocked_by = [19, 20]` for prerequisites. Independent work may run in parallel; dependent work waits for its blockers to complete.

Users can override specific rules with explicit instructions (e.g., "skip PR for this", "don't commit"). Override only the rule explicitly mentioned — all other rules remain in effect.

# Docs
See `docs/` for project-specific documentation: tech stack, architecture, conventions.
Keep it up to date.

# Coding
Follow the Zen of Python.

- No features, single-use abstractions, or configurability beyond what was requested.
- Do not refactor or reformat unrelated code. Mention unrelated dead code rather than deleting it.
- State material assumptions. Ask only when unresolved ambiguity affects scope, correctness, or risk.

## Research codebase
No defensive coding, no preemptive backward compatibility.
Fail explicitly.
Readability is important.
Correctness is even more important.
Move fast, break things, but break them correctly.

## Tests
Add tests for the behavior we care about, including tests that catch other agents breaking our work. Do not add tests for the sake of tests.
The whole suite must pass in under 10 seconds.

## Validation
- Run all Python with uv. Lint with `uv run ruff check` and check formatting with `uv run ruff format --check`, scoped to the affected files. Linting does not prove behavior: also run the affected behavior checks.
- Validate once a coherent change is complete, not after every edit.
- For parallel work, one agent owns shared validation and runs it after the related edits land.

# Workflow
When a step doesn't need the user's input, keep going. Stop and ask only when you can't continue without the user. Opening a PR is a stop: report the URL and wait for review.

## Worktrees
Multiple agents work on this repository at once. Make every change in a git worktree, including creating grapes issues. You may already be in one.

## Issues
Every file change needs a grapes issue first: code, docs, and instructions alike. Research, exploration, and answering questions do not. The [grapes skill](.agents/skills/grapes/SKILL.md) covers the issue format, lifecycle, and activity log.

Allocate IDs with `id=$(grapes issue)`, never by hand. The CLI allocates across every worktree in `.grapes/config.toml`; a local `ls` sees only this checkout, so two agents scanning minutes apart pick the same number.

## Git
- Branches: `<id>/short-description`, for example `42/fix-parser-crash`.
- Commits: `#<id>: Imperative description`, under 72 characters. Commit each validated unit of work; keep a fix and its regression test in the same commit.
- Stage the files you changed for the issue by name (`git add <paths>`), never with `git add -A`, `git add .`, or `git commit -a`. Review `git status` before every commit: a file you did not change showing as modified or deleted means a stale working tree, so investigate instead of committing it. A wholesale commit from a stale tree once reverted other agents' merged work (#472).

## Pull requests
A PR is the review gate. Open or reuse one with the [pr skill](.agents/skills/pr/SKILL.md).

- Mark every issue the PR solves `done` on the branch, with its acceptance criteria checked off and its verification recorded. Leave an issue `in_progress` only when the PR leaves part of it unfinished, and say which part in the description.
- Show the reviewer that the change works, in proportion to the change. Evidence must be viewable from the PR itself: the reviewer may be on another device, and the worktree is deleted after merge. The pr skill says what evidence each kind of change needs and how to publish images. For viewer changes, the [viewer skill](.agents/skills/viewer/SKILL.md) covers capturing, debugging and verifying in a real browser.
- Never merge without explicit human approval. If the reviewer requests changes, push new commits to the same branch and tell the user.
- After your PR merges, fast-forward local `main` and remove the merged branch and its worktree.

## Verify state, don't recall it
Humans and other agents change shared state concurrently; a PR you opened minutes ago may already be merged. Check PR, issue, and branch state when you report or act on it (`gh pr view <n> --json state`, `git fetch origin main`), including before adding commits to an existing PR. Empty output is evidence: `gh pr list` printing nothing means there are no open PRs.

## Subagents
- Give each subagent its grapes issue ID and working directory. Commit its issue files first: uncommitted files do not reach another worktree.
- Check a subagent's evidence before accepting its report.
- Delegate reading to a subagent only for read-only material (papers, many files, logs) whose answer is small. Ask for the answer with file paths and line numbers. Read files you will edit directly.
- The agent that changes files commits them and opens the PR, unless a lead is named as integrator. The grapes skill covers who maintains which issue files.

## Long-running jobs
Prefer completion notifications to polling. Filter monitor output so that only completion, failure, or a change needing attention wakes you. For sustained polling, use a cheap subagent given only the job identifiers, commands, paths, and success criteria. Never restart, modify, or cancel a monitored job without authorization.

# Files and data
- Temporary outputs go in `.grapes/<id>/tmp/`, or `.grapes/tmp/` before an issue exists, not in `/tmp/` or the session scratchpad. Both paths are gitignored.

# Context and output
- Read only the output you need. Select fields from structured output (`--json`, `jq`); send output that may be long to a file in the issue's tmp directory and inspect parts of it. Say what you truncated or skipped.
- Bound reads: find the range with `grep -n`, then use `Read` with `offset` and `limit`. Do not `cat` whole docs or several files at once.
- When a result says "Output too large ... saved to file", search or page the saved file with `grep -n` or `Read` `offset`/`limit`. Do not read it whole.
- If your response references a table, figure, or result, include it inline. If you saved output, give its path, absolute when it is in a worktree.

# Zen of Writing
Follow the Zen of Writing in everything you write, including responses:
Clear is better than impressive.
Precise is better than elegant.
Simple is better than complex.
Complex is better than vague.
Although leaving out the hard part is not simplicity.
Short is better than long.
Unless brevity costs meaning.
Every sentence should earn its place.
Old information comes before new.
Actors belong in subjects.
Actions belong in verbs.
Call the same thing by the same name.
One paragraph does one job.
Structure should show the reasoning.
Evidence beats assertion.
Reasoning beats evidence alone.
Separate what you know from what you infer.
A claim should be no stronger than its support.
Hedge the claim and not the sentence.
A citation is not an argument.
A technical term should save more than it costs.
Write for the reader and not for yourself.
Break any of these before writing something no one can follow.

## Goal

An agent started from a grapes issue learns from its first prompt that it owns the issue, how the issue relates to others, and what kind of work it is, from a template the repository can edit.

## Description

`a` on an issue without a session opens the new-session dialog with one sentence: "Work on grapes issue #N: <title>. Its specification is in .grapes/N/." The agent is not told it owns the issue, that Vineyard already made its worktree, or about the issue's parent, sub-issues, blockers, or labels. And the sentence can be false: the session's worktree starts at the checkout's HEAD, so an issue that is uncommitted or only on another branch is missing from it.

Changes:

1. **Templates.** The prompt is rendered from a Go `text/template`. Templates come from two places, merged: each `.vineyard/templates/<name>.md` file, and `[[templates]]` entries (`name`, `text`) in `.vineyard/config.toml`. A name defined twice is an error. When `.vineyard/templates/` is missing, Vineyard creates it with three examples: `default`, `bug`, and `research`. `.vineyard/.gitignore` shares the directory like `config.toml`.
2. **Choosing.** The dialog preselects the template named after the first of the issue's labels that has one, else `default`, else none (an empty prompt). A Template field cycles through the templates and none with ←/→.
3. **Build sub-issues.** For an issue with sub-issues, a checkbox "Build sub-issues" sets `.BuildSubIssues`; the examples then tell the agent to build the sub-issues no session works on, and otherwise to leave them to their own sessions.
4. **Re-rendering.** Changing the template or the checkbox renders the prompt again, replacing its text. The prompt stays editable. A template that fails to render shows its error in the dialog.
5. **Data.** `.ID`, `.Title`, `.Status`, `.Labels`, `.Parent` (nil or `.ID`, `.Title`, `.Status`), `.SubIssues` (each with `.Branch`, the branch of a session working on it, or empty), `.Blockers` (the issues it is blocked by that are neither done nor cancelled), `.BuildSubIssues`. Rendered prompts lose trailing spaces and repeated blank lines, so templates need no whitespace control.
6. **Missing issue.** Starting a session for an issue that is not committed at the checkout's HEAD fails with an error saying so, and creates nothing.

## Context

- `showSessions` in `internal/tui/app.go` builds today's prompt and calls `dialog.NewSession.ForIssue` (`internal/tui/dialog/dialog.go`).
- Issue relations come from `embedded.Issue`, which gains `Labels`, `Parent`, `Children`, and `BlockedBy` in grapes #56 (grapes branch `56/embedded-issue-relations`, released as v0.1.14). `go.mod` pins that branch's commit until the release exists.
- Reading templates is file I/O, so it runs in a command (`Backend.Templates`); nothing blocks `Update`.
- `config.Prepare` creates `.vineyard` and its `.gitignore`; an existing `.gitignore` is left alone, so older ones keep ignoring `templates/` until edited.
- `session.Manager.Start` creates the worktree at `git.Head(m.Repo.Checkout)`.
- Prompts are delivered by `promptCmd` as a bracketed paste; multi-line prompts work today. Delivery bugs are #24.

## Acceptance Criteria

- [x] `.vineyard/templates/` is created with `default.md`, `bug.md`, and `research.md` when missing, and git does not ignore it.
- [x] Templates load from the directory and from `[[templates]]` in `config.toml`; a duplicate or empty name is reported as an error.
- [x] `a` on an issue labelled `bug` preselects the `bug` template; an issue without a matching label gets `default`.
- [x] The rendered example prompts state ownership, the worktree, the status when not `todo`, the parent, open blockers, and sub-issues with their branches.
- [x] The Build sub-issues checkbox appears only for issues with sub-issues and toggles its paragraph in the prompt.
- [x] Starting a session for an issue not committed at HEAD fails with an error naming the issue, leaving no worktree or branch.
- [x] README and docs describe templates and their data.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual: in a scratch repository with grapes issues (a parent with two sub-issues, one labelled `bug` and blocked by an open issue), run vineyard in tmux, press `a` on each issue, toggle the template and checkbox, and capture the dialog. Start a session on an uncommitted issue and capture the error.

## Pass Criteria

`gofmt` prints nothing; vet and tests pass. The captures show the `bug` template preselected for the bug, the sub-issue list with the checkbox toggling its paragraph, and the error for the uncommitted issue.

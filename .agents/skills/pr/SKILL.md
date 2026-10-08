---
name: pr
description: "Push the current branch and create or reuse a GitHub PR. Resolves issue and target, publishes with gh against github.com, and reports the URL."
argument-hint: "[issue-id] [--target <branch>]"
user-invocable: true
---

# Create Pull Request

Parse `$ARGUMENTS` for:
1. **Issue ID** (optional): infer from the branch name (`<id>/...`) or recent commit messages (`#<id>: ...`) if omitted. Resolve one unambiguous issue before pushing.
2. **`--target <branch>`** (optional): preserve an existing open PR's target unless its stack parent merged. For a new PR, default to the repository's default branch.

Requires `gh` authenticated to `github.com`, `origin` on `github.com`, and the changes committed on a feature branch.

## Target

Branch from current `origin/main` and target `main`. If the repository has a different default branch, use it wherever this skill says `main`.

Stack the PR on another unmerged PR only when the change requires that PR and separate review is useful. Related topics, overlapping files, or working from an existing feature branch are not reasons to stack. For a stacked PR, also follow [stacks.md](stacks.md).

## Step 1: Gather Context

1. Get the current branch: `git branch --show-current`. Refuse to publish from `main` or a detached HEAD.
2. Resolve the issue from arguments, branch name, or commit messages and read its context using the [Grapes reading rules](../grapes/SKILL.md). If the issue is missing or ambiguous, stop before pushing; do not invent an ID.
3. Check PRs for the current branch across all states:
   ```bash
   gh pr list --head <branch-name> --state all --json number,state,baseRefName,headRefName,url
   ```
   - If one open PR exists, reuse it. Resolve multiple matching open PRs before pushing. If an explicit target conflicts with its actual target, ask rather than silently retargeting it.
   - If the branch's PR has merged, do not append work to that branch. Start a new branch from current `origin/main`, carry over only the unmerged work, and create a new PR.
   - If the PR closed without merging, resolve whether to reopen or replace it before pushing.
4. Fetch the target (`git fetch origin <target>`), compute `<base>` once with `git merge-base HEAD origin/<target>`, then collect `git log <base>..HEAD --oneline` and `git diff <base>..HEAD --stat`. Recompute the base if the branch or target changes.
5. List open PRs with `gh pr list` and compare each one's files (`gh pr diff <PR-number> --name-only`) with your diff. Overlapping files mean likely merge conflicts and possibly duplicated work; flag them in the report so the human can sequence the merges.

## Step 2: Sync With the Target

Rebase onto the target before pushing so conflicts are resolved locally, not discovered on GitHub after the PR opens.

1. `git fetch origin <target>`, then `git log --oneline HEAD..origin/<target>`. If this is empty, skip to Step 3.
2. `git rebase origin/<target>`.
3. On conflicts, resolve them, `git add` the files, and `git rebase --continue`. If the correct resolution is unclear, run `git rebase --abort` and ask the user.
4. Rerun the affected checks after a rebase.

## Step 3: Push

Immediately before pushing, check the branch's PR state again, and for a stack the parent's state. If either merged since Step 1, follow the branch and target rules above.

```bash
git push -u origin <branch-name>
```

If Step 2 rebased a previously pushed branch, a normal push may be rejected. Use `git push --force-with-lease` only on your own feature branch, and ask the user first if it has an open PR.

## Step 4: Create PR

Reuse an existing open PR rather than running `gh pr create`; update its title or description only when the task requires it.

For a new PR, write the description to `.grapes/<id>/tmp/pr-description.md`:
- A summary of the changes and the Grapes issue reference.
- Verification commands and results, including limitations, and the evidence that the change works (see Evidence).
- A changed-file summary.

```bash
gh pr create \
  --head <branch-name> \
  --base <target> \
  --title "#<id>: Short description" \
  --body-file ".grapes/<id>/tmp/pr-description.md"
```

The title describes the change, not necessarily the issue title, and stays under 72 characters.

### Evidence

Show the reviewer that the change works, in proportion to the change:

- **Behaviour change:** the command you ran and the part of its output that proves the claim.
- **TUI change:** screenshots of the real app, captured as [development.md](../../../docs/development.md#screenshots-for-pull-requests) describes. Show before and after when the change alters an existing view. Golden files are test fixtures, not evidence: a reviewer cannot see them rendered.
- **Instructions or docs only:** nothing beyond the diff.

A local or worktree path is not evidence. `gh` cannot upload PR attachments, so images go on the orphan branch `pr-evidence` under `<id>/<name>.png`, which keeps binaries out of `main` and outlives the worktree. Publish them from the worktree root:

```bash
bash .agents/skills/pr/scripts/publish-evidence.sh <id> .grapes/<id>/tmp/<name>.png ...
```

The script leaves your branch, index, and working tree untouched and prints the evidence commit SHA. If the push is rejected because another agent pushed first, rerun it. Never force-push `pr-evidence`.

Embed each image by that SHA, which keeps the link stable when the branch moves on, with a caption saying what the reviewer should see:

```markdown
![Diff view with one file folded](https://github.com/mikeryanboss/vineyard/blob/<commit>/<id>/<name>.png?raw=true)
```

## Step 5: Report

Tell the user:
- The PR URL, its actual title and target branch, and the number of commits.
- Any open PRs whose files overlap this PR's diff.
- Whether the work is **implemented**, **PR opened**, **merged into an intermediate branch**, or **integrated into `main`**. A `MERGED` status alone does not show integration: fetch current `main` and verify the changes are present, since ancestry may not show it after squash or rebase merges.

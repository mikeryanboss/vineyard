# Stacked PRs

A stacked PR targets another unmerged PR's branch instead of `main`. Use one only when the change requires that PR and separate review is useful.

## Creating or Updating

- Record the parent PR URL and the reason for the dependency in the child PR description.
- Before creating or updating the child, and again immediately before pushing, check the parent's current state: `gh pr view <parent-number> --json state,baseRefName,headRefName`.
- If the parent has merged, move only the child's remaining changes onto current `origin/main` and target `main` (`gh pr edit <PR-number> --base main`). Account for squash and rebase merges: do not blindly replay the parent's commits. Verify that the resulting diff contains only the intended child changes and rerun the affected checks.
- If the parent closed without merging, resolve the dependency before proceeding; do not target its abandoned branch.
- In the report, include the parent PR URL, its freshly checked state, and the remaining path into `main`.

## Merging, Once a Human Approves

- Prefer merging the parent into `main`, then moving the child's remaining changes onto current `main`, retargeting the child, and checking its diff and affected tests before merging it.
- Immediately before each merge, recheck the PR's actual base and the parent's state. Do not merge a child into a branch whose PR has already merged.
- A merge incorporates a snapshot; later merges into the source branch do not reach `main`. When reporting a stack complete, verify its changes on current `main`.

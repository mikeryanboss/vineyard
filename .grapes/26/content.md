## Goal
Enter on a session opens the tab that is shown, full-screen, instead of always
attaching to the agent. The diff opens in a diff tool the user configures,
Hunk by default.

## Description
Today `enter` and `o` share the `Attach` binding (`internal/tui/common/keys.go`),
so enter attaches whatever tab is shown (`handleKey`, `internal/tui/app.go`).
`i` opens grapes; there is no full-screen diff.

With this change, enter on the session list acts on the shown tab:

| Tab | enter |
| --- | --- |
| Preview | attach to the agent, as before |
| Diff | run the configured diff tool in the session's worktree; without one, show Vineyard's diff full-screen |
| Issue | open grapes at the session's issue, as `i` does |
| Recap | show the recap full-screen |

`o` always attaches, from any tab. `i` is unchanged.

## Context
- New config field `diff_command` (planned) in `internal/config/config.go`,
  default `hunk diff {base} --watch`. `{base}` is replaced by the session's
  base commit. The command runs through `sh -c` in the session's worktree, via
  `tea.ExecProcess`, like attach. Quitting the tool returns to Vineyard.
- The tool counts as missing when the command is empty or its first word is not
  found on `PATH`. Then enter zooms Vineyard's own diff instead. The lookup runs
  in a command, not in `Update`.
- Hunk fits Vineyard's diff requirements: `hunk diff <base>` diffs a commit
  against the working tree, includes untracked files, and reads them with
  `git --no-optional-locks status` instead of `git add -N`, so it does not write
  the worktree's index (modem-dev/hunk 0.23.0, `packages/hunk-git/src/commands.ts`,
  `isWorkingTreeGitDiffInput` and `buildGitStatusArgs`).
- Zoom (planned): the pane fills the body, without the session list. Agents'
  tmux windows keep the split-pane size (`paneContentSize`), so zooming does not
  reflow agents. `esc` leaves the zoom and returns to the list; switching tabs
  leaves the zoom.
- The settings screen (`internal/tui/settings/settings.go`) gains a text field
  for the diff command.
- A session without a worktree has nothing to diff (`diffable`); enter on its
  diff tab says so.

## Acceptance Criteria
- [x] Enter on the Preview tab attaches, as before.
- [x] Enter on the Diff tab runs `diff_command` with `{base}` replaced, in the session's worktree.
- [x] With `diff_command` empty or its program missing, enter on the Diff tab shows Vineyard's diff full-screen; `esc` returns to the split view.
- [x] Enter on the Issue tab opens grapes as `i` does.
- [x] Enter on the Recap tab shows the recap full-screen.
- [x] `o` attaches from every tab.
- [x] `diff_command` defaults to `hunk diff {base} --watch`, can be edited on the config screen, and is saved.
- [x] The status bar names what enter does on the shown tab.
- [x] Docs describe the new keys and setting.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then, in a real terminal, run vineyard with one session that has changes and
press enter on each tab, with Hunk installed and with `diff_command` pointing at
a missing program.

## Pass Criteria
`gofmt -l .` prints nothing; vet and tests pass. In the terminal, each tab
behaves as in the table above, and screenshots show the zoomed diff and the
diff tool.

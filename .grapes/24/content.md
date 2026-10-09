## Goal
A session started with a prompt (`N`, or `a` on an issue) always starts the agent working on that prompt. Today it sometimes exits, does nothing, or waits with the prompt unsent.

## Description
Observed with Claude Code 2.1.295 while recording the README demo (#23), in a fresh repository at `/tmp/vineyard-demo`:

1. **Trust prompt not recognized; Enter exits Claude Code.** Claude Code's workspace trust prompt now reads "Quick safety check: Is this a project you created or one you trust?" with `❯ No, exit` preselected and "Yes, I trust this folder" second. `trustPrompts` in `internal/session/screen.go` only matches "Do you trust the files in this folder?", so Vineyard treats the screen as idle, pastes the prompt, and sends Enter, which selects "No, exit". The session stops at once. With auto-yes, a recognized trust prompt is answered with Enter too, which would also pick "No, exit". Worktrees under a directory Claude Code already trusts skip the prompt, which hides the bug in everyday use.
2. **Enter after the paste becomes a newline.** `promptCmd` in `internal/tui/app.go` calls `backend.Paste` (tmux `paste-buffer -p`) and then `SendEnter` immediately. In about one take in three, Claude Code took that Enter as a newline: the prompt stayed in the input box with the cursor on a new line, unsent. A second Enter sent later submits it.
3. **Prompt lost.** In one take, a session's input box was empty: the prompt never arrived. A still screen during Claude Code's startup probably counted as ready, and the paste arrived before the input accepted it. This cause is unconfirmed.

## Context
- `internal/session/screen.go`: `trustPrompts`, `AwaitingTrust`, `permissionPrompts`.
- `internal/tui/app.go`: `applyStatus` (trust handling, pending prompt delivery), `promptCmd`.
- `internal/tmux/tmux.go`: `Paste`, `SendEnter`.
- `doc/vhs/demo-submit.sh` works around problem 2 for the demo by pressing Enter on prompts left in the input box; remove it, and its note in `docs/development.md`, once this is fixed.

## Acceptance Criteria
- [ ] The current Claude Code trust prompt is detected; auto-yes accepts it by choosing "Yes, I trust this folder", and without auto-yes the status line asks the user to attach.
- [ ] A prompt pasted into Claude Code is submitted, not left in the input box.
- [ ] A prompt is not sent before the agent can accept input.
- [ ] Tests cover the new trust prompt text and the answer auto-yes sends.

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
```
Then, in a repository Claude Code does not trust, start five sessions from issues with `vineyard -y`, and five without `-y`, answering the trust prompt by attaching.

## Pass Criteria
Checks pass, and every session's agent starts working on its prompt without manual input beyond the trust answer.

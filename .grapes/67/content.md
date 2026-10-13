## Goal
Pressing enter on an empty title in the new-session dialog creates the session with a random title, instead of refusing with "A title is required."

## Description
Starting a quick session should not require inventing a name. When the title is empty or only spaces, the dialog picks a random two-word title, an adjective and a grape variety such as "amber riesling", and creates the session with it. The title becomes the branch and worktree slug as before.

## Context
- `internal/tui/dialog/dialog.go`, `(*NewSession).submit`: currently sets `d.err = "A title is required."` and refocuses the title when it is empty.
- `(*NewSession).Update`: enter on the title moves to the next field only when the title is non-empty and the dialog has a prompt (issue sessions); otherwise it submits. That stays: enter on an empty title submits.
- `internal/session/session.go`, `newID` and `Slug`: the ID carries a time suffix and `freeBranch` numbers taken branches, so repeated random titles do not collide.
- Test `TestNewSession_RequiresTitle` and its golden file in `internal/tui/dialog/testdata/` encode the old behaviour.

## Acceptance Criteria
- [x] Enter on an empty title produces a `NewSessionMsg` whose title is two words from the dialog's word lists.
- [x] A typed title is still used unchanged.
- [x] The README's key table says an empty title gets a random one.
- [x] The dialog no longer shows "A title is required."

## Verify
Run from the worktree root:
```bash
gofmt -l . && go vet ./... && go test ./...
scripts/drive  # open the dialog with n, press enter, see the session listed with a random title
```

## Pass Criteria
gofmt prints nothing, vet and tests pass, and a session created with an empty title appears in the list with a two-word title and a matching branch.

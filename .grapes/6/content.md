# Evidence guidance for agents: screenshots of the real TUI

## Goal
Tell agents how to show that a Vineyard change works: screenshots of the running TUI, not golden files.

## Description
PR #3 first used a golden render as evidence. Golden files are test fixtures for agents, not something a reviewer can see. The pr skill still describes another project: "the 3D viewer, plots, figures", an example URL for `Modexus/worldmodels`, and `AGENTS.md` links a `viewer` skill that does not exist here. `publish-evidence.sh` is also not executable.

## Context
- `.agents/skills/pr/SKILL.md`, section Evidence.
- `AGENTS.md` (symlinked as `CLAUDE.md`), Pull requests.
- `docs/development.md`: add the capture recipe, since it already covers running a development build.

## Changes
- Name the TUI as the visual change and say that golden files are not evidence.
- Document the capture recipe: run the app in tmux at a fixed size, capture with `tmux capture-pane -e`, render with `freeze`.
- Fix the example URL, and run the publish script with `bash`.
- Remove the link to the missing `viewer` skill.

## Acceptance criteria
- [x] No reference to the 3D viewer, `worldmodels`, or the `viewer` skill remains.
- [x] The recipe in `docs/development.md` works as written (same commands run for PR #3 screenshots).
- [x] The pr skill and `AGENTS.md` point to it.

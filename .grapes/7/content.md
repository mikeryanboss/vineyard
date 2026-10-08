# Start in the light theme until the terminal reports its background

## Goal
Make light the default theme, so screenshots captured where the terminal does not report a background (tmux, `freeze`) come out light.

## Description
`tui.NewModel` builds a dark theme and replaces it when `tea.BackgroundColorMsg` arrives. Terminals that never answer stay dark. Change the starting theme to light. Detection stays: a terminal that reports a dark background still gets the dark theme.

## Context
- `internal/tui/app.go`: `NewModel` (initial theme) and the `tea.BackgroundColorMsg` case.
- `common.NewTheme(isDark bool)` in `internal/tui/common/theme.go`.

## Assumption
"Default" means the theme used when the terminal reports nothing. Making light unconditional, or adding a config option, was not requested.

## Acceptance criteria
- [x] A new `tui.Model` uses the light theme; a `BackgroundColorMsg` with a dark background switches it to dark.
- [x] `gofmt -l .`, `go vet ./...`, `go test ./...` pass.
- [x] A screenshot of the real app in tmux shows the light theme.

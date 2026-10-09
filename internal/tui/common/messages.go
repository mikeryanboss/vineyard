// Package common holds the types shared by the TUI's views: messages, key
// bindings, and the theme. Views emit these messages instead of acting on
// sessions themselves; the root model in internal/tui handles them.
package common

import "github.com/mikeryanboss/vineyard/internal/config"

// Action is a confirmable operation on a session.
type Action int

const (
	ActionKill Action = iota
	ActionPause
	ActionPush
)

// NewSessionMsg asks for a session to be created.
type NewSessionMsg struct {
	Title   string
	Prompt  string
	Profile config.Profile
	// Issue is the grapes issue the session is for, or 0.
	Issue int
}

// ConfirmedMsg reports that the user confirmed an action.
type ConfirmedMsg struct {
	Action    Action
	SessionID string
}

// DialogCancelledMsg reports that a dialog was dismissed.
type DialogCancelledMsg struct{}

// ScrollbackRequestMsg asks for the selected session's scrollback, because
// the user started scrolling the preview.
type ScrollbackRequestMsg struct{}

// LeavePaneMsg returns focus from the preview or diff pane to the list.
type LeavePaneMsg struct{}

// SwitchTabMsg moves the right pane to the next tab, or the previous one if Back.
type SwitchTabMsg struct{ Back bool }

// SaveConfigMsg asks for the configuration to be written and applied.
type SaveConfigMsg struct{ Config config.Config }

// CloseConfigMsg leaves the config screen without saving.
type CloseConfigMsg struct{}

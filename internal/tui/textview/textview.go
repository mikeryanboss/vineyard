// Package textview shows rendered text in a scrollable pane, or a placeholder
// in its place. The issue and recap tabs use it.
package textview

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// Model is a text pane's content area.
type Model struct {
	width       int
	height      int
	theme       common.Theme
	placeholder string
	viewport    viewport.Model
}

// New returns an empty text view.
func New(theme common.Theme) Model {
	return Model{theme: theme, viewport: viewport.New()}
}

// SetSize sets the content area's size.
func (m Model) SetSize(width, height int) Model {
	m.width, m.height = width, height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	return m
}

// SetTheme replaces the theme.
func (m Model) SetTheme(theme common.Theme) Model {
	m.theme = theme
	return m
}

// SetContent shows rendered text, keeping the scroll position, so a reload
// of the same text does not jump back to the top.
func (m Model) SetContent(content string) Model {
	m.placeholder = ""
	m.viewport.SetContent(content)
	return m
}

// SetPlaceholder replaces the text with a message.
func (m Model) SetPlaceholder(text string) Model {
	m.placeholder = text
	m.viewport.SetContent("")
	return m
}

// GotoTop scrolls to the top, for when another session's text is shown.
func (m Model) GotoTop() Model {
	m.viewport.GotoTop()
	return m
}

// Update handles keys while the pane has focus, and the mouse wheel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.viewport.ScrollUp(3)
		case tea.MouseWheelDown:
			m.viewport.ScrollDown(3)
		}
	case tea.KeyPressMsg:
		keys := common.PaneKeyMap
		switch {
		case key.Matches(msg, keys.Back):
			return m, func() tea.Msg { return common.LeavePaneMsg{} }
		case key.Matches(msg, keys.Tab):
			return m, func() tea.Msg { return common.SwitchTabMsg{} }
		case key.Matches(msg, keys.Up):
			m.viewport.ScrollUp(1)
		case key.Matches(msg, keys.Down):
			m.viewport.ScrollDown(1)
		case key.Matches(msg, keys.HalfUp):
			m.viewport.HalfPageUp()
		case key.Matches(msg, keys.HalfDown):
			m.viewport.HalfPageDown()
		case key.Matches(msg, keys.Top):
			m.viewport.GotoTop()
		case key.Matches(msg, keys.Bottom):
			m.viewport.GotoBottom()
		}
	}
	return m, nil
}

// View renders the content area at exactly its set size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.placeholder != "" {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.StyleSubtitle.Render(m.placeholder))
	}
	return m.viewport.View()
}

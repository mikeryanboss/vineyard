// Package preview mirrors a session's terminal: the live screen captured from
// tmux, or its scrollback while the user scrolls.
package preview

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// reset ends any colour a captured line leaves open, so it cannot bleed into
// the borders and panes drawn after it.
const reset = "\x1b[0m"

// Model is the preview pane's content area.
type Model struct {
	width       int
	height      int
	theme       common.Theme
	screen      string
	placeholder string
	scrolling   bool
	viewport    viewport.Model
}

// New returns an empty preview.
func New(theme common.Theme) Model {
	return Model{theme: theme, viewport: viewport.New()}
}

// SetSize sets the content area's size, which is also the size the agent's
// tmux window is kept at.
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

// SetScreen shows a freshly captured screen.
func (m Model) SetScreen(screen string) Model {
	m.screen = screen
	m.placeholder = ""
	return m
}

// SetPlaceholder replaces the screen with a message, for sessions without a
// running terminal.
func (m Model) SetPlaceholder(text string) Model {
	m.placeholder = text
	m.screen = ""
	m.scrolling = false
	return m
}

// SetScrollback enters scroll mode showing history, scrolled to the bottom
// and then up by one step, since a scroll request is what asked for it.
func (m Model) SetScrollback(history string) Model {
	lines := strings.Split(strings.TrimRight(history, "\n"), "\n")
	for i, line := range lines {
		lines[i] = line + reset
	}
	m.viewport.SetContentLines(lines)
	m.viewport.GotoBottom()
	m.viewport.ScrollUp(1)
	m.scrolling = true
	return m
}

// Scrolling reports whether the preview shows scrollback instead of the live screen.
func (m Model) Scrolling() bool { return m.scrolling }

// ExitScroll returns to the live screen.
func (m Model) ExitScroll() Model {
	m.scrolling = false
	return m
}

// Update handles keys while the pane has focus.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if wheel, ok := msg.(tea.MouseWheelMsg); ok {
			return m.wheel(wheel)
		}
		return m, nil
	}
	keys := common.PaneKeyMap
	switch {
	case key.Matches(keyMsg, keys.Back):
		m.scrolling = false
		return m, func() tea.Msg { return common.LeavePaneMsg{} }
	case key.Matches(keyMsg, keys.Tab):
		return m, func() tea.Msg { return common.SwitchTabMsg{} }
	}
	if !m.scrolling {
		if key.Matches(keyMsg, keys.Up, keys.HalfUp, keys.Top) {
			return m, func() tea.Msg { return common.ScrollbackRequestMsg{} }
		}
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, keys.Up):
		m.viewport.ScrollUp(1)
	case key.Matches(keyMsg, keys.Down):
		m.viewport.ScrollDown(1)
	case key.Matches(keyMsg, keys.HalfUp):
		m.viewport.HalfPageUp()
	case key.Matches(keyMsg, keys.HalfDown):
		m.viewport.HalfPageDown()
	case key.Matches(keyMsg, keys.Top):
		m.viewport.GotoTop()
	case key.Matches(keyMsg, keys.Bottom):
		m.scrolling = false
	}
	if m.viewport.AtBottom() {
		m.scrolling = false
	}
	return m, nil
}

func (m Model) wheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseWheelUp && !m.scrolling:
		return m, func() tea.Msg { return common.ScrollbackRequestMsg{} }
	case msg.Button == tea.MouseWheelUp:
		m.viewport.ScrollUp(3)
	case msg.Button == tea.MouseWheelDown && m.scrolling:
		m.viewport.ScrollDown(3)
		if m.viewport.AtBottom() {
			m.scrolling = false
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
	if m.scrolling {
		return m.viewport.View()
	}
	lines := strings.Split(strings.TrimRight(m.screen, "\n"), "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, line := range lines {
		line = ansi.Truncate(line, m.width, "")
		if gap := m.width - ansi.StringWidth(line); gap > 0 {
			line += reset + strings.Repeat(" ", gap)
		}
		lines[i] = line + reset
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	return strings.Join(lines, "\n")
}

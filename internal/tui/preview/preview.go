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
	rendered    string
}

// New returns an empty preview.
func New(theme common.Theme) Model {
	return Model{theme: theme, viewport: viewport.New()}
}

// SetSize sets the content area's size, which is also the size the agent's
// tmux window is kept at.
func (m Model) SetSize(width, height int) Model {
	if m.width == width && m.height == height && (width <= 0 || height <= 0 || m.rendered != "") {
		return m
	}
	m.width, m.height = width, height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	return m.rebuild()
}

// SetTheme replaces the theme.
func (m Model) SetTheme(theme common.Theme) Model {
	m.theme = theme
	return m.rebuild()
}

// SetScreen shows a freshly captured screen.
func (m Model) SetScreen(screen string) Model {
	if !m.scrolling && m.placeholder == "" && m.screen == screen && (m.width <= 0 || m.height <= 0 || m.rendered != "") {
		return m
	}
	m.screen = screen
	m.placeholder = ""
	return m.rebuild()
}

// SetPlaceholder replaces the screen with a message, for sessions without a
// running terminal.
func (m Model) SetPlaceholder(text string) Model {
	if !m.scrolling && m.placeholder == text && m.screen == "" && (m.width <= 0 || m.height <= 0 || m.rendered != "") {
		return m
	}
	m.placeholder = text
	m.screen = ""
	m.scrolling = false
	return m.rebuild()
}

// SetScrollback enters scroll mode showing history, scrolled to the bottom
// and then up by one step, since a scroll request is what asked for it.
// History no longer than the screen has nothing above it, as for a program
// drawing on the alternate screen, so the preview stays live.
func (m Model) SetScrollback(history string) Model {
	lines := strings.Split(strings.TrimRight(history, "\n"), "\n")
	if len(lines) <= len(strings.Split(strings.TrimRight(m.screen, "\n"), "\n")) {
		return m
	}
	for i, line := range lines {
		lines[i] = line + reset
	}
	m.viewport.SetContentLines(lines)
	m.viewport.GotoBottom()
	m.viewport.ScrollUp(1)
	m.scrolling = true
	return m.rebuild()
}

// Scrolling reports whether the preview shows scrollback instead of the live screen.
func (m Model) Scrolling() bool { return m.scrolling }

// ExitScroll returns to the live screen.
func (m Model) ExitScroll() Model {
	if !m.scrolling {
		return m
	}
	m.scrolling = false
	return m.rebuild()
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
	keys := common.Keys.Pane
	switch {
	case key.Matches(keyMsg, keys.Back):
		m.scrolling = false
		return m.rebuild(), func() tea.Msg { return common.LeavePaneMsg{} }
	case key.Matches(keyMsg, keys.NextTab, keys.PrevTab):
		back := key.Matches(keyMsg, keys.PrevTab)
		return m, func() tea.Msg { return common.SwitchTabMsg{Back: back} }
	}
	if !m.scrolling {
		x, y := m.width/2, min(1, max(0, m.height-1))
		switch {
		case key.Matches(keyMsg, keys.Up):
			return m, scrollRequest(x, y, 1)
		case key.Matches(keyMsg, keys.Down):
			return m, scrollRequest(x, y, -1)
		case key.Matches(keyMsg, keys.HalfUp):
			return m, scrollRequest(x, y, 5)
		case key.Matches(keyMsg, keys.HalfDown):
			return m, scrollRequest(x, y, -5)
		case key.Matches(keyMsg, keys.Top):
			return m, scrollRequest(x, y, 20)
		case key.Matches(keyMsg, keys.Bottom):
			return m, scrollRequest(x, y, -20)
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
	return m.rebuild(), nil
}

func scrollRequest(x, y, n int) tea.Cmd {
	return func() tea.Msg { return common.ScrollbackRequestMsg{X: x, Y: y, N: n} }
}

func (m Model) wheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseWheelUp && !m.scrolling:
		return m, scrollRequest(msg.X, msg.Y, 1)
	case msg.Button == tea.MouseWheelDown && !m.scrolling:
		return m, scrollRequest(msg.X, msg.Y, -1)
	case msg.Button == tea.MouseWheelUp:
		m.viewport.ScrollUp(3)
	case msg.Button == tea.MouseWheelDown:
		m.viewport.ScrollDown(3)
		if m.viewport.AtBottom() {
			m.scrolling = false
		}
	}
	return m.rebuild(), nil
}

func (m Model) rebuild() Model {
	if m.width <= 0 || m.height <= 0 {
		m.rendered = ""
		return m
	}
	if m.placeholder != "" {
		// Wrapped to fit: a longer line would wrap in the terminal and push
		// the rest of the screen down.
		m.rendered = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.StyleSubtitle.Render(ansi.Wordwrap(m.placeholder, m.width, "")))
		return m
	}
	if m.scrolling {
		m.rendered = m.viewport.View()
		return m
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
	m.rendered = strings.Join(lines, "\n")
	return m
}

// View renders the content area at exactly its set size.
func (m Model) View() string {
	return m.rendered
}

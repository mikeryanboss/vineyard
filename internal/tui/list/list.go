// Package list renders the session list: one entry per session with its
// status, branch, and diff size.
package list

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// itemHeight is the number of lines one entry occupies, spacing included.
const itemHeight = 3

// headerHeight is the number of lines above the first entry.
const headerHeight = 2

// Item is one session as the list shows it.
type Item struct {
	Session session.Session
	Stat    git.Stat
	// Issues are the grapes issues the session works on, in ascending order.
	Issues []int
}

// Model is the session list.
type Model struct {
	items    []Item
	selected int
	offset   int // index of the first visible item
	width    int
	height   int
	focused  bool
	theme    common.Theme
}

// New returns an empty list.
func New(theme common.Theme) Model {
	return Model{theme: theme, focused: true}
}

// SetItems replaces the entries, keeping the selection on the same session
// when it still exists.
func (m Model) SetItems(items []Item) Model {
	selectedID := ""
	if item, ok := m.Selected(); ok {
		selectedID = item.Session.ID
	}
	m.items = items
	for i, item := range items {
		if item.Session.ID == selectedID {
			m.selected = i
			return m.clamp()
		}
	}
	return m.clamp()
}

// Select moves the selection to the session with id, if present.
func (m Model) Select(id string) Model {
	for i, item := range m.items {
		if item.Session.ID == id {
			m.selected = i
		}
	}
	return m.clamp()
}

// SetSize sets the list's outer size.
func (m Model) SetSize(width, height int) Model {
	m.width, m.height = width, height
	return m.clamp()
}

// SetTheme replaces the theme.
func (m Model) SetTheme(theme common.Theme) Model {
	m.theme = theme
	return m
}

// SetFocused marks whether keyboard input goes to the list.
func (m Model) SetFocused(focused bool) Model {
	m.focused = focused
	return m
}

// Selected returns the selected entry.
func (m Model) Selected() (Item, bool) {
	if m.selected < 0 || m.selected >= len(m.items) {
		return Item{}, false
	}
	return m.items[m.selected], true
}

// Len returns the number of entries.
func (m Model) Len() int { return len(m.items) }

func (m Model) visibleItems() int {
	return max(1, (m.height-headerHeight)/itemHeight)
}

// clamp keeps the selection in range and scrolled into view.
func (m Model) clamp() Model {
	m.selected = max(0, min(m.selected, len(m.items)-1))
	visible := m.visibleItems()
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+visible {
		m.offset = m.selected - visible + 1
	}
	m.offset = max(0, min(m.offset, len(m.items)-visible))
	return m
}

// Update handles navigation keys and clicks.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, common.ListKeyMap.Up):
			m.selected--
		case key.Matches(msg, common.ListKeyMap.Down):
			m.selected++
		}
		return m.clamp(), nil
	case tea.MouseClickMsg:
		// Coordinates are relative to the list's top-left corner.
		if msg.Y >= headerHeight {
			if i := m.offset + (msg.Y-headerHeight)/itemHeight; i < len(m.items) {
				m.selected = i
			}
		}
		return m.clamp(), nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.selected--
		case tea.MouseWheelDown:
			m.selected++
		}
		return m.clamp(), nil
	}
	return m, nil
}

// View renders the list at exactly its set size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	title := m.theme.StyleTitle.Render("Sessions")
	if len(m.items) > 0 {
		title += m.theme.StyleFaint.Render(fmt.Sprintf(" %d", len(m.items)))
	}
	lines := []string{" " + title, ""}

	if len(m.items) == 0 {
		lines = append(lines,
			m.theme.StyleSubtitle.Render("  No sessions yet."),
			"",
			"  "+m.theme.FormatKeyHint("n", "start one"),
		)
	}
	end := min(len(m.items), m.offset+m.visibleItems())
	for i := m.offset; i < end; i++ {
		lines = append(lines, m.renderItem(m.items[i], i == m.selected)...)
	}
	return lipgloss.NewStyle().Width(m.width).Height(m.height).MaxHeight(m.height).Render(strings.Join(lines, "\n"))
}

// renderItem renders one entry as its title line, branch line, and spacer.
func (m Model) renderItem(item Item, selected bool) []string {
	t := m.theme
	s := item.Session
	style := func(base lipgloss.Style) lipgloss.Style {
		if selected {
			return base.Background(t.StyleSelected.GetBackground())
		}
		return base
	}

	marker := style(lipgloss.NewStyle()).Render(" ")
	if selected {
		marker = style(lipgloss.NewStyle().Foreground(t.ColorAccent)).Render("▌")
	}
	icon := style(lipgloss.NewStyle().Foreground(t.StatusColor(s.Status))).Render(common.StatusIcon(s.Status))

	titleStyle := t.StyleTitle
	if s.Status == session.StatusPaused || s.Status == session.StatusStopped {
		titleStyle = t.StyleSubtitle
	}
	tag := ""
	if s.AutoYes {
		tag = style(t.StyleFaint).Render(" auto")
	}
	issues := ""
	for _, id := range item.Issues {
		issues += style(lipgloss.NewStyle().Foreground(t.ColorAccent)).Render(fmt.Sprintf("#%d", id)) + style(lipgloss.NewStyle()).Render(" ")
	}
	titleWidth := m.width - 4 - ansi.StringWidth(tag) - ansi.StringWidth(issues)
	titleText := style(titleStyle).Render(ansi.Truncate(s.Title, max(0, titleWidth), "…"))
	titleLine := marker + icon + style(lipgloss.NewStyle()).Render(" ") + issues + titleText + tag

	stat := ""
	if !item.Stat.IsZero() {
		stat = style(lipgloss.NewStyle().Foreground(t.ColorAddFg)).Render(fmt.Sprintf("+%d", item.Stat.Added)) +
			style(lipgloss.NewStyle()).Render(" ") +
			style(lipgloss.NewStyle().Foreground(t.ColorDelFg)).Render(fmt.Sprintf("-%d", item.Stat.Removed))
	}
	branch := s.Branch
	if branch == "" {
		branch = string(s.Status) + "…"
	}
	branchWidth := m.width - 5 - ansi.StringWidth(stat) - 1
	branchText := style(t.StyleFaint).Render(common.IconBranch + " " + ansi.Truncate(branch, max(0, branchWidth-2), "…"))
	branchLine := marker + style(lipgloss.NewStyle()).Render("  ") + branchText

	return []string{
		pad(titleLine, m.width, style(lipgloss.NewStyle())),
		padBetween(branchLine, stat, m.width, style(lipgloss.NewStyle())),
		"",
	}
}

// pad extends line to width with fill-styled spaces.
func pad(line string, width int, fill lipgloss.Style) string {
	if gap := width - ansi.StringWidth(line); gap > 0 {
		return line + fill.Render(strings.Repeat(" ", gap))
	}
	return ansi.Truncate(line, width, "")
}

// padBetween places right at the end of the line, one cell from the edge.
func padBetween(left, right string, width int, fill lipgloss.Style) string {
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right) - 1
	if gap < 1 {
		return pad(left, width, fill)
	}
	return left + fill.Render(strings.Repeat(" ", gap)) + right + fill.Render(" ")
}

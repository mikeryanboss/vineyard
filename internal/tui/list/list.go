// Package list renders the session list: one entry per session with its
// status, branch, diff size, and running subagents.
package list

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// headerHeight is the number of lines above the first entry.
const headerHeight = 2

// Item is one session as the list shows it.
type Item struct {
	Session session.Session
	Stat    git.Stat
	// Issues are the grapes issues the session works on, in ascending order.
	Issues []int
	// Subagents are the session's running subagents, as Claude Code lists
	// them on its screen.
	Subagents subagent.List
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
	// sub is the selected subagent of the selected entry, counted from 0, or
	// -1 for its session. It may exceed the entry's subagents, so that the
	// selection stays put while Claude Code briefly draws fewer, as it does
	// while redrawing; the row method returns the subagent selected.
	sub int
	// expanded holds the IDs of the sessions whose subagent rows are shown.
	// The others show one line counting their subagents.
	expanded map[string]bool
}

// New returns an empty list.
func New(theme common.Theme) Model {
	return Model{theme: theme, focused: true, sub: -1, expanded: map[string]bool{}}
}

// SetItems replaces the entries, keeping the selection on the same session,
// and on the same place in its subagents, when the session still exists.
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
	m.sub = -1
	return m.clamp()
}

// Select moves the selection to the session with id, if present.
func (m Model) Select(id string) Model {
	for i, item := range m.items {
		if item.Session.ID == id {
			m.selected, m.sub = i, -1
		}
	}
	return m.clamp()
}

// Expand shows the selected session's subagent rows. It reports whether it
// did: the session has subagents, and their rows were hidden.
func (m Model) Expand() (Model, bool) {
	if !m.Expandable() {
		return m, false
	}
	m.expanded = maps.Clone(m.expanded)
	m.expanded[m.items[m.selected].Session.ID] = true
	return m.clamp(), true
}

// Collapse hides the selected session's subagent rows and selects the
// session.
func (m Model) Collapse() Model {
	item, ok := m.Selected()
	if !ok {
		return m
	}
	m.expanded = maps.Clone(m.expanded)
	delete(m.expanded, item.Session.ID)
	m.sub = -1
	return m.clamp()
}

// Expandable reports whether the selected session has subagents whose rows
// are hidden.
func (m Model) Expandable() bool {
	item, ok := m.Selected()
	return ok && m.collapsed(item)
}

// Collapsible reports whether the selected session shows subagent rows.
func (m Model) Collapsible() bool {
	item, ok := m.Selected()
	return ok && m.rows(item) > 0
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

// Selected returns the selected entry. It is the session of a selected
// subagent row too.
func (m Model) Selected() (Item, bool) {
	if m.selected < 0 || m.selected >= len(m.items) {
		return Item{}, false
	}
	return m.items[m.selected], true
}

// SelectedSubagent returns the selected subagent of the selected entry,
// counted from 0 in Claude Code's list, if a subagent row is selected.
func (m Model) SelectedSubagent() (int, bool) {
	if row := m.row(); row >= 0 {
		return row, true
	}
	return 0, false
}

// row returns the selected row of the selected entry: a subagent, counted
// from 0, or -1 for the session.
func (m Model) row() int {
	if m.selected >= len(m.items) {
		return -1
	}
	return min(m.sub, m.rows(m.items[m.selected])-1)
}

// rows is the number of subagent rows item shows: none while collapsed.
func (m Model) rows(item Item) int {
	if m.expanded[item.Session.ID] {
		return item.Subagents.Len()
	}
	return 0
}

// collapsed reports whether item has subagents and counts them instead of
// showing their rows.
func (m Model) collapsed(item Item) bool {
	return item.Subagents.Len() > 0 && !m.expanded[item.Session.ID]
}

// entryHeight is the number of lines item occupies: its title and branch
// lines, its subagent rows or the line counting them, and a blank line.
func (m Model) entryHeight(item Item) int {
	if m.collapsed(item) {
		return 4
	}
	return 3 + m.rows(item)
}

// Len returns the number of entries.
func (m Model) Len() int { return len(m.items) }

// move moves the selection one row down, or up when delta is -1, through
// sessions and their subagent rows.
func (m Model) move(delta int) Model {
	if len(m.items) == 0 {
		return m
	}
	m.sub = m.row() + delta
	switch {
	case m.sub < -1 && m.selected > 0:
		m.selected--
		m.sub = m.rows(m.items[m.selected]) - 1
	case m.sub >= m.rows(m.items[m.selected]) && m.selected < len(m.items)-1:
		m.selected++
		m.sub = -1
	}
	m.sub = max(-1, min(m.sub, m.rows(m.items[m.selected])-1))
	return m.clamp()
}

// room is the number of lines the entries may fill.
func (m Model) room() int { return m.height - headerHeight }

// span is the number of lines entries first through last occupy.
func (m Model) span(first, last int) int {
	lines := 0
	for _, item := range m.items[first : last+1] {
		lines += m.entryHeight(item)
	}
	return lines
}

// clamp keeps the selection in range and scrolled into view.
func (m Model) clamp() Model {
	m.selected = max(0, min(m.selected, len(m.items)-1))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	for m.offset < m.selected && m.span(m.offset, m.selected) > m.room() {
		m.offset++
	}
	for m.offset > 0 && m.span(m.offset-1, len(m.items)-1) <= m.room() {
		m.offset--
	}
	return m
}

// Update handles navigation keys and clicks.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, common.ListKeyMap.Up):
			return m.move(-1), nil
		case key.Matches(msg, common.ListKeyMap.Down):
			return m.move(1), nil
		}
		return m, nil
	case tea.MouseClickMsg:
		return m.click(msg.Y), nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			return m.move(-1), nil
		case tea.MouseWheelDown:
			return m.move(1), nil
		}
		return m, nil
	}
	return m, nil
}

// click selects the row on line y, counted from the list's top. A subagent
// row selects the subagent, any other line of an entry its session.
func (m Model) click(y int) Model {
	y -= headerHeight
	if y < 0 {
		return m
	}
	for i := m.offset; i < len(m.items); i++ {
		item := m.items[i]
		if y < m.entryHeight(item) {
			m.selected, m.sub = i, -1
			if row := y - 2; row >= 0 && row < m.rows(item) {
				m.sub = row
			}
			return m.clamp()
		}
		y -= m.entryHeight(item)
	}
	return m
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
	for i := m.offset; i < len(m.items) && len(lines) < m.height; i++ {
		sub := -2 // no row of this entry is selected
		if i == m.selected {
			sub = m.row()
		}
		lines = append(lines, m.renderItem(m.items[i], sub)...)
	}
	return lipgloss.NewStyle().Width(m.width).Height(m.height).MaxHeight(m.height).Render(strings.Join(lines, "\n"))
}

// renderItem renders one entry as its title line, branch line, subagent
// rows or the line counting its subagents, and spacer. sub is the selected
// row of the entry: -1 for the session, a subagent counted from 0, or -2 for
// none.
func (m Model) renderItem(item Item, sub int) []string {
	t := m.theme
	s := item.Session
	selected := sub == -1
	style := m.rowStyle(selected)
	marker := m.marker(selected)
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

	lines := []string{
		pad(titleLine, m.width, style(lipgloss.NewStyle())),
		padBetween(branchLine, stat, m.width, style(lipgloss.NewStyle())),
	}
	for i := range m.rows(item) {
		lines = append(lines, m.renderSubagent(item.Subagents, i, i == sub))
	}
	if m.collapsed(item) {
		count := fmt.Sprintf("▸ %d subagents", item.Subagents.Len())
		if item.Subagents.Len() == 1 {
			count = "▸ 1 subagent"
		}
		countLine := marker + style(lipgloss.NewStyle()).Render("  ") +
			style(t.StyleFaint).Render(ansi.Truncate(count, max(0, m.width-4), "…"))
		lines = append(lines, pad(countLine, m.width, style(lipgloss.NewStyle())))
	}
	return append(lines, "")
}

// renderSubagent renders the row of subagent i, counted from 0, as a branch
// of the session's tree; a subagent another started is indented under it, or
// counted after it, (+2), as Claude Code does. Claude Code draws only some of
// its subagents; the others have no label, only their place in its list. The
// subagent whose view Claude Code shows in place of the main conversation is
// highlighted.
func (m Model) renderSubagent(l subagent.List, i int, selected bool) string {
	t := m.theme
	style := m.rowStyle(selected)
	text := fmt.Sprintf("subagent %d", i+1)
	if drawn := i - l.Above; drawn >= 0 && drawn < len(l.Rows) {
		text = l.Rows[drawn].Label
		if n := l.Rows[drawn].Hidden; n > 0 {
			text += fmt.Sprintf(" (+%d)", n)
		}
	}
	branch := "├ "
	if lastSibling(l, i) {
		branch = "└ "
	}
	branch = strings.Repeat("  ", depth(l, i)) + branch
	label, tree := t.StyleFaint, t.StyleFaint
	if i+1 == l.Shown {
		tree = lipgloss.NewStyle().Foreground(t.ColorAccent)
	}
	if i+1 == l.Shown || selected {
		label = lipgloss.NewStyle()
	}
	line := m.marker(selected) + style(lipgloss.NewStyle()).Render("  ") + style(tree).Render(branch) +
		style(label).Render(ansi.Truncate(text, max(0, m.width-4-ansi.StringWidth(branch)), "…"))
	return pad(line, m.width, style(lipgloss.NewStyle()))
}

// depth returns how deep subagent i is nested; one Claude Code does not draw
// counts as not nested.
func depth(l subagent.List, i int) int {
	if drawn := i - l.Above; drawn >= 0 && drawn < len(l.Rows) {
		return l.Rows[drawn].Depth
	}
	return 0
}

// lastSibling reports whether no later subagent shares subagent i's parent.
func lastSibling(l subagent.List, i int) bool {
	for j := i + 1; j < l.Len(); j++ {
		if d := depth(l, j); d <= depth(l, i) {
			return d < depth(l, i)
		}
	}
	return true
}

// rowStyle returns a function that gives a style the selection's background
// when selected.
func (m Model) rowStyle(selected bool) func(lipgloss.Style) lipgloss.Style {
	return func(base lipgloss.Style) lipgloss.Style {
		if selected {
			return base.Background(m.theme.StyleSelected.GetBackground())
		}
		return base
	}
}

// marker is the first cell of a row: a bar when the row is selected.
func (m Model) marker(selected bool) string {
	if selected {
		return m.rowStyle(true)(lipgloss.NewStyle().Foreground(m.theme.ColorAccent)).Render("▌")
	}
	return " "
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

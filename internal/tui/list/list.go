// Package list renders the session list: one entry per session with its
// status, branch, diff size, and subagents.
package list

import (
	"fmt"
	"maps"
	"slices"
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
	// Subagents are the session's subagents worth showing, each followed by
	// those it started.
	Subagents []subagent.Agent
	// Shown is the ID of the subagent whose view Claude Code shows in place
	// of the conversation, or "".
	Shown string
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
	// sub is the ID of the selected subagent of the selected entry, or "" for
	// its session. A subagent that is no longer shown selects its session.
	sub string
	// expanded holds the IDs of the sessions whose subagent rows are shown.
	// The others show one line counting their subagents.
	expanded map[string]bool
}

// New returns an empty list.
func New(theme common.Theme) Model {
	return Model{theme: theme, focused: true, expanded: map[string]bool{}}
}

// SetItems replaces the entries, keeping the selection on the same session,
// and on the same subagent, when they still exist.
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
	m.sub = ""
	return m.clamp()
}

// Select moves the selection to the session with id, if present.
func (m Model) Select(id string) Model {
	for i, item := range m.items {
		if item.Session.ID == id {
			m.selected, m.sub = i, ""
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
	return m.setExpanded(m.items[m.selected].Session.ID, true), true
}

// Collapse hides the selected session's subagent rows and selects the
// session.
func (m Model) Collapse() Model {
	item, ok := m.Selected()
	if !ok {
		return m
	}
	m.sub = ""
	return m.setExpanded(item.Session.ID, false)
}

// setExpanded shows or hides the subagent rows of the session with id.
func (m Model) setExpanded(id string, expanded bool) Model {
	m.expanded = maps.Clone(m.expanded)
	if expanded {
		m.expanded[id] = true
	} else {
		delete(m.expanded, id)
	}
	return m.clamp()
}

// Expandable reports whether the selected session has subagents whose rows
// are hidden.
func (m Model) Expandable() bool {
	item, ok := m.Selected()
	return ok && len(item.Subagents) > 0 && !m.expanded[item.Session.ID]
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

// SelectedSubagent returns the selected subagent of the selected entry, if
// a subagent row is selected.
func (m Model) SelectedSubagent() (subagent.Agent, bool) {
	if row := m.row(); row >= 0 {
		return m.items[m.selected].Subagents[row], true
	}
	return subagent.Agent{}, false
}

// row returns the selected row of the selected entry: a subagent, counted
// from 0, or -1 for the session.
func (m Model) row() int {
	if m.sub == "" || m.selected >= len(m.items) {
		return -1
	}
	item := m.items[m.selected]
	for i, a := range item.Subagents[:m.rows(item)] {
		if a.ID == m.sub {
			return i
		}
	}
	return -1
}

// rows is the number of subagent rows item shows: none while collapsed.
func (m Model) rows(item Item) int {
	if m.expanded[item.Session.ID] {
		return len(item.Subagents)
	}
	return 0
}

// selectRow selects row of the selected entry: a subagent, counted from 0,
// or the session for -1.
func (m *Model) selectRow(row int) {
	m.sub = ""
	if row >= 0 {
		m.sub = m.items[m.selected].Subagents[row].ID
	}
}

// entryHeight is the number of lines item occupies: its title and branch
// lines, the line counting its subagents and their rows, and a blank line.
func (m Model) entryHeight(item Item) int {
	height := 3 + m.rows(item)
	if len(item.Subagents) > 0 {
		height++ // the line counting them
	}
	return height
}

// Len returns the number of entries.
func (m Model) Len() int { return len(m.items) }

// move moves the selection one row down, or up when delta is -1, through
// sessions and their subagent rows.
func (m Model) move(delta int) Model {
	if len(m.items) == 0 {
		return m
	}
	row := m.row() + delta
	switch {
	case row < -1 && m.selected > 0:
		m.selected--
		row = m.rows(m.items[m.selected]) - 1
	case row >= m.rows(m.items[m.selected]) && m.selected < len(m.items)-1:
		m.selected++
		row = -1
	}
	m.selectRow(max(-1, min(row, m.rows(m.items[m.selected])-1)))
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
		case key.Matches(msg, common.Keys.List.Up):
			return m.move(-1), nil
		case key.Matches(msg, common.Keys.List.Down):
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

// at returns the entry drawn on line y, counted from the list's top, and
// which of its lines y is: 0 its title, 1 its branch, 2 the count of its
// subagents if it has any. ok is false where no entry is drawn.
func (m Model) at(y int) (entry, line int, ok bool) {
	y -= headerHeight
	if y < 0 || y >= m.room() {
		return 0, 0, false
	}
	for i := m.offset; i < len(m.items); i++ {
		if height := m.entryHeight(m.items[i]); y >= height {
			y -= height
			continue
		}
		return i, y, true
	}
	return 0, 0, false
}

// Line is what a line of the list shows.
type Line int

const (
	LineNone     Line = iota // no entry
	LineSession              // a session's title, branch, or the blank after it
	LineCount                // the line counting a session's subagents
	LineSubagent             // a subagent's row
)

// LineAt returns what line y, counted from the list's top, shows.
func (m Model) LineAt(y int) Line {
	i, line, ok := m.at(y)
	switch {
	case !ok:
		return LineNone
	case line == 2 && len(m.items[i].Subagents) > 0:
		return LineCount
	case line >= 3 && line-3 < m.rows(m.items[i]):
		return LineSubagent
	}
	return LineSession
}

// click selects the row on line y, counted from the list's top. A subagent
// row selects the subagent, any other line of an entry its session; the line
// counting a session's subagents also shows or hides their rows.
func (m Model) click(y int) Model {
	i, line, ok := m.at(y)
	if !ok {
		return m
	}
	id := m.items[i].Session.ID
	m.selected, m.sub = i, ""
	switch m.LineAt(y) {
	case LineCount:
		return m.setExpanded(id, !m.expanded[id])
	case LineSubagent:
		m.selectRow(line - 3)
	}
	return m.clamp()
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

// renderItem renders one entry as its title line, branch line, the line
// counting its subagents and their rows, and spacer. sub is the selected
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
	if len(item.Subagents) > 0 {
		toggle := "▸ "
		if m.expanded[s.ID] {
			toggle = "▾ "
		}
		countLine := marker + style(lipgloss.NewStyle()).Render("  ") +
			style(t.StyleFaint).Render(ansi.Truncate(toggle+count(item.Subagents), max(0, m.width-4), "…"))
		lines = append(lines, pad(countLine, m.width, style(lipgloss.NewStyle())))
	}
	for i := range m.rows(item) {
		lines = append(lines, m.renderSubagent(item, i, i == sub))
	}
	return append(lines, "")
}

// count describes subagents by status: "3 subagents: 2 running, 1 done", or
// "2 subagents running" when they share one.
func count(agents []subagent.Agent) string {
	noun := "subagents"
	if len(agents) == 1 {
		noun = "subagent"
	}
	var parts []string
	for _, status := range []subagent.Status{subagent.Running, subagent.Done, subagent.Stopped} {
		if n := len(slices.DeleteFunc(slices.Clone(agents), func(a subagent.Agent) bool { return a.Status != status })); n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, statusWords[status]))
		}
	}
	if len(parts) == 1 {
		return fmt.Sprintf("%d %s %s", len(agents), noun, statusWords[agents[0].Status])
	}
	return fmt.Sprintf("%d %s: %s", len(agents), noun, strings.Join(parts, ", "))
}

var statusWords = map[subagent.Status]string{subagent.Running: "running", subagent.Done: "done", subagent.Stopped: "stopped"}

// sessionStatus is the session status whose icon and colour a subagent's
// status shares: done is ready for review.
var sessionStatus = map[subagent.Status]session.Status{
	subagent.Running: session.StatusRunning,
	subagent.Done:    session.StatusReady,
	subagent.Stopped: session.StatusStopped,
}

// renderSubagent renders the row of item's subagent i, counted from 0, as a
// branch of the session's tree, with its status and description; a subagent
// another started is indented under it. The subagent whose view Claude Code
// shows in place of the main conversation is highlighted.
func (m Model) renderSubagent(item Item, i int, selected bool) string {
	t := m.theme
	style := m.rowStyle(selected)
	a := item.Subagents[i]
	text := a.Description
	if text == "" {
		text = a.Type
	}
	branch := "├ "
	if lastSibling(item.Subagents, i) {
		branch = "└ "
	}
	branch = strings.Repeat("  ", a.Depth) + branch
	label, tree := t.StyleFaint, t.StyleFaint
	if a.ID == item.Shown {
		tree = lipgloss.NewStyle().Foreground(t.ColorAccent)
	}
	if a.ID == item.Shown || selected {
		label = lipgloss.NewStyle()
	}
	status := sessionStatus[a.Status]
	icon := style(lipgloss.NewStyle().Foreground(t.StatusColor(status))).Render(common.StatusIcon(status))
	line := m.marker(selected) + style(lipgloss.NewStyle()).Render("  ") + style(tree).Render(branch) + icon +
		style(lipgloss.NewStyle()).Render(" ") +
		style(label).Render(ansi.Truncate(text, max(0, m.width-6-ansi.StringWidth(branch)), "…"))
	return pad(line, m.width, style(lipgloss.NewStyle()))
}

// lastSibling reports whether no later subagent shares subagent i's parent.
func lastSibling(agents []subagent.Agent, i int) bool {
	for _, a := range agents[i+1:] {
		if a.Depth <= agents[i].Depth {
			return a.Depth < agents[i].Depth
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

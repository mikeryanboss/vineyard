// Package list renders the session list: one entry per session with its
// status, branch, diff size, and subagents.
package list

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
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
	// Issues are the main grapes issues the session works on, in ascending
	// order.
	Issues []int
	// Subagents are the session's subagents worth showing, each followed by
	// those it started.
	Subagents []subagent.Agent
	// Shown is the ID of the subagent whose view Claude Code shows in place
	// of the conversation, or "".
	Shown string
}

// Options controls what each session entry shows in the list.
type Options struct {
	// Issues places a session's issue numbers: config.IssuesAfter,
	// config.IssuesBefore, or config.IssuesOff. Empty means IssuesAfter.
	Issues string
	// Subagents shows the line counting a session's subagents and their rows.
	Subagents bool
	// Branch shows a session's branch line beneath its title.
	Branch bool
	// DiffStats shows a session's +N -M line counts.
	DiffStats bool
	// AutoTag shows "auto" on sessions with auto-yes enabled.
	AutoTag bool
}

// DefaultOptions returns the options New uses: issue numbers after titles and
// every element shown.
func DefaultOptions() Options {
	return Options{
		Issues:    config.IssuesAfter,
		Subagents: true,
		Branch:    true,
		DiffStats: true,
		AutoTag:   true,
	}
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
	opts     Options
	// sub is the ID of the selected subagent of the selected entry, or "" for
	// its session. A subagent that is no longer shown selects its session.
	sub string
	// expanded holds the IDs of the sessions whose subagent rows are shown.
	// The others show one line counting their subagents.
	expanded map[string]bool
	// moving marks the selected session as being moved.
	moving bool
}

// New returns an empty list.
func New(theme common.Theme) Model {
	return Model{theme: theme, focused: true, opts: DefaultOptions(), expanded: map[string]bool{}}
}

// SetOptions replaces the display options.
func (m Model) SetOptions(opts Options) Model {
	m.opts = opts
	if m.row() < 0 {
		m.sub = ""
	}
	return m.clamp()
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
	return ok && len(m.subagentsOf(item)) > 0 && !m.expanded[item.Session.ID]
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

// SetMoving marks whether the selected session is being moved.
func (m Model) SetMoving(moving bool) Model {
	m.moving = moving
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

// subagentsOf returns item's subagents when subagents are shown.
func (m Model) subagentsOf(item Item) []subagent.Agent {
	if !m.opts.Subagents {
		return nil
	}
	return item.Subagents
}

// rows is the number of subagent rows item shows: none while collapsed.
func (m Model) rows(item Item) int {
	if m.expanded[item.Session.ID] {
		return len(m.subagentsOf(item))
	}
	return 0
}

// sessionLines is the number of lines a session's own header occupies: its
// title line, plus its branch line when shown.
func (m Model) sessionLines() int {
	if m.opts.Branch {
		return 2
	}
	return 1
}

// selectRow selects row of the selected entry: a subagent, counted from 0,
// or the session for -1.
func (m *Model) selectRow(row int) {
	m.sub = ""
	if row >= 0 {
		m.sub = m.items[m.selected].Subagents[row].ID
	}
}

// entryHeight is the number of lines item occupies: its title and optional
// branch lines, the line counting its subagents and their rows, and a blank
// line.
func (m Model) entryHeight(item Item) int {
	height := m.sessionLines() + 1 + m.rows(item)
	if len(m.subagentsOf(item)) > 0 {
		height++ // the line counting them
	}
	return height
}

// Len returns the number of entries.
func (m Model) Len() int { return len(m.items) }

// move moves the selection one row down, or up when delta is -1, through
// sessions and their subagent rows. With wrap, moving past the last row
// selects the first session and moving past the first session the last row;
// without, the selection stops at the ends.
func (m Model) move(delta int, wrap bool) Model {
	if len(m.items) == 0 {
		return m
	}
	last := len(m.items) - 1
	row := m.row() + delta
	switch {
	case row < -1 && m.selected > 0:
		m.selected--
		row = m.rows(m.items[m.selected]) - 1
	case row < -1 && wrap:
		m.selected = last
		row = m.rows(m.items[last]) - 1
	case row >= m.rows(m.items[m.selected]) && m.selected < last:
		m.selected++
		row = -1
	case row >= m.rows(m.items[m.selected]) && wrap:
		m.selected = 0
		row = -1
	}
	m.selectRow(max(-1, min(row, m.rows(m.items[m.selected])-1)))
	return m.clamp()
}

// Offset returns the index of the first visible entry.
func (m Model) Offset() int { return m.offset }

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
			return m.move(-1, true), nil
		case key.Matches(msg, common.Keys.List.Down):
			return m.move(1, true), nil
		}
		return m, nil
	case tea.MouseClickMsg:
		return m.click(msg.Y), nil
	case tea.MouseWheelMsg:
		// One flick of the wheel sends a burst of events, which wrapping
		// would carry past the end and around again.
		switch msg.Button {
		case tea.MouseWheelUp:
			return m.move(-1, false), nil
		case tea.MouseWheelDown:
			return m.move(1, false), nil
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
	countLine := m.sessionLines()
	subStart := countLine + 1
	switch {
	case !ok:
		return LineNone
	case line == countLine && len(m.subagentsOf(m.items[i])) > 0:
		return LineCount
	case line >= subStart && line-subStart < m.rows(m.items[i]):
		return LineSubagent
	}
	return LineSession
}

// DragDelta returns -1 or 1 when a dragged session at m.selected should step
// up or down to follow pointer line y, counted from the list's top, or 0 to
// stay where it is.
func (m Model) DragDelta(y int) int {
	if m.selected < 0 || m.selected >= len(m.items) {
		return 0
	}
	top := headerHeight
	for i := m.offset; i < m.selected; i++ {
		top += m.entryHeight(m.items[i])
	}
	h := m.entryHeight(m.items[m.selected])
	if m.selected > 0 {
		if m.selected <= m.offset {
			if y < headerHeight {
				return -1
			}
		} else {
			prevH := m.entryHeight(m.items[m.selected-1])
			prevTop := top - prevH
			if y < prevTop+(prevH+1)/2 {
				return -1
			}
		}
	}
	if m.selected+1 < len(m.items) {
		nextH := m.entryHeight(m.items[m.selected+1])
		down := max(top+h, top+(nextH+1)/2)
		if top+h >= m.height {
			down = m.height - 1
		}
		if y >= down {
			return 1
		}
	}
	return 0
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
		m.selectRow(line - (m.sessionLines() + 1))
	}
	return m.clamp()
}

// View renders the list at exactly its set size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	fit := func(s string) string {
		if ansi.StringWidth(s) > m.width {
			return ansi.Truncate(s, m.width, "…")
		}
		return s
	}
	title := m.theme.StyleTitle.Render("Sessions")
	if len(m.items) > 0 {
		title += m.theme.StyleFaint.Render(fmt.Sprintf(" %d", len(m.items)))
	}
	lines := []string{fit(" " + title), ""}

	if len(m.items) == 0 {
		lines = append(lines,
			fit(m.theme.StyleSubtitle.Render("  No sessions yet.")),
			"",
			fit("  "+m.theme.FormatKeyHint("n", "start one")),
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
	if m.opts.AutoTag && s.AutoYes {
		tag = style(t.StyleFaint).Render(" auto")
	}
	stat := ""
	if m.opts.DiffStats && !item.Stat.IsZero() {
		stat = style(lipgloss.NewStyle().Foreground(t.ColorAddFg)).Render(fmt.Sprintf("+%d", item.Stat.Added)) +
			style(lipgloss.NewStyle()).Render(" ") +
			style(lipgloss.NewStyle().Foreground(t.ColorDelFg)).Render(fmt.Sprintf("-%d", item.Stat.Removed))
	}
	avail := m.width - 4 - ansi.StringWidth(tag)
	if !m.opts.Branch && stat != "" {
		avail -= ansi.StringWidth(stat) + 1
	}
	avail = max(0, avail)

	var body string
	switch m.opts.Issues {
	case config.IssuesOff:
		body = style(titleStyle).Render(ansi.Truncate(s.Title, avail, "…"))
	case config.IssuesBefore:
		issues := m.renderIssues(item.Issues, selected, false)
		titleWidth := max(0, avail-ansi.StringWidth(issues))
		body = issues + style(titleStyle).Render(ansi.Truncate(s.Title, titleWidth, "…"))
	default:
		titleWidth := ansi.StringWidth(s.Title)
		switch {
		case titleWidth > avail:
			body = style(titleStyle).Render(ansi.Truncate(s.Title, avail, "…"))
		case titleWidth == avail || len(item.Issues) == 0:
			body = style(titleStyle).Render(s.Title)
		default:
			issues := m.renderIssues(item.Issues, selected, true)
			if rem := avail - titleWidth; ansi.StringWidth(issues) > rem {
				issues = ansi.Truncate(issues, rem, style(t.StyleFaint).Render("…"))
			}
			body = style(titleStyle).Render(s.Title) + issues
		}
	}

	titleMarker := marker
	if selected && m.moving {
		titleMarker = style(lipgloss.NewStyle().Foreground(t.ColorAccent)).Render("↕")
	}
	titleLine := titleMarker + icon + style(lipgloss.NewStyle()).Render(" ") + body + tag

	var lines []string
	if m.opts.Branch {
		branch := s.Branch
		if branch == "" {
			branch = string(s.Status) + "…"
		}
		branchWidth := m.width - 5 - ansi.StringWidth(stat) - 1
		branchText := style(t.StyleFaint).Render(common.IconBranch + " " + ansi.Truncate(branch, max(0, branchWidth-2), "…"))
		branchLine := marker + style(lipgloss.NewStyle()).Render("  ") + branchText
		lines = []string{
			pad(titleLine, m.width, style(lipgloss.NewStyle())),
			padBetween(branchLine, stat, m.width, style(lipgloss.NewStyle())),
		}
	} else {
		lines = []string{
			padBetween(titleLine, stat, m.width, style(lipgloss.NewStyle())),
		}
	}
	if subs := m.subagentsOf(item); len(subs) > 0 {
		toggle := "▸ "
		if m.expanded[s.ID] {
			toggle = "▾ "
		}
		countLine := marker + style(lipgloss.NewStyle()).Render("  ") +
			style(t.StyleFaint).Render(ansi.Truncate(toggle+count(subs), max(0, m.width-4), "…"))
		lines = append(lines, pad(countLine, m.width, style(lipgloss.NewStyle())))
	}
	for i := range m.rows(item) {
		lines = append(lines, m.renderSubagent(item, i, i == sub))
	}
	return append(lines, "")
}

// renderIssues renders ids for a session's title line, separated from the
// title by a space on the left when after is true and on the right otherwise.
func (m Model) renderIssues(ids []int, selected, after bool) string {
	if len(ids) == 0 {
		return ""
	}
	t := m.theme
	style := m.rowStyle(selected)
	clipped, more := common.ClipIssues(ids)
	var parts []string
	for _, id := range clipped {
		parts = append(parts, style(lipgloss.NewStyle().Foreground(t.ColorAccent)).Render(fmt.Sprintf("#%d", id)))
	}
	if more > 0 {
		parts = append(parts, style(t.StyleFaint).Render(fmt.Sprintf("+%d", more)))
	}
	space := style(lipgloss.NewStyle()).Render(" ")
	joined := strings.Join(parts, space)
	if after {
		return space + joined
	}
	return joined + space
}

// count describes subagents by status: "3 subagents: 2 running, 1 done", or
// "2 subagents running" when they share one.
func count(agents []subagent.Agent) string {
	noun := "subagents"
	if len(agents) == 1 {
		noun = "subagent"
	}
	var counts [3]int
	for _, a := range agents {
		if a.Status >= 0 && int(a.Status) < len(counts) {
			counts[a.Status]++
		}
	}
	var parts []string
	for _, status := range []subagent.Status{subagent.Running, subagent.Done, subagent.Stopped} {
		if n := counts[status]; n > 0 {
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

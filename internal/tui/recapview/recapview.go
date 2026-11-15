// Package recapview renders a session's conversation on the Recap tab: its
// title and entry counts, followed by user prompts, final agent responses, and
// away recaps. Long entries are limited to a few preview lines by default and
// can be expanded or folded by mouse click or keyboard.
package recapview

import (
	"fmt"
	"image/color"
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// PreviewLines is how many wrapped body lines an unexpanded entry shows before
// truncating with a "… N more lines" hint.
const PreviewLines = 4

type viewMode int

const (
	modePlaceholder viewMode = iota
	modeRaw
	modeRecap
)

type entryKey struct {
	unixNano int64
	kind     recap.Kind
	length   int
	head     string
}

func keyOf(e recap.Entry) entryKey {
	head := e.Text
	if len(head) > 64 {
		head = head[:64]
	}
	return entryKey{
		unixNano: e.Time.UnixNano(),
		kind:     e.Kind,
		length:   len(e.Text),
		head:     head,
	}
}

type entrySpan struct {
	start       int // header line index in lines
	end         int // exclusive end line index (before blank separator)
	collapsible bool
	expanded    bool
}

// Model is the recap pane's content area.
type Model struct {
	width       int
	height      int
	visible     bool
	focused     bool
	dirty       bool
	theme       common.Theme
	mode        viewMode
	placeholder string
	rawContent  string

	sessionID string
	hasRecap  bool
	recap     recap.Recap
	expanded  map[entryKey]bool
	cursor    int
	lines     []string
	spans     []entrySpan
	viewport  viewport.Model
}

// New returns an empty recap view.
func New(theme common.Theme) Model {
	return Model{
		visible:  true,
		theme:    theme,
		expanded: map[entryKey]bool{},
		viewport: viewport.New(),
	}
}

// SetVisible marks whether the recap pane is on screen, deferring line wrapping
// while hidden.
func (m Model) SetVisible(visible bool) Model {
	if m.visible == visible {
		return m
	}
	m.visible = visible
	if !visible {
		return m
	}
	if m.dirty {
		return m.rerender()
	}
	return m
}

// SetFocused marks whether the recap pane has keyboard focus, highlighting the
// active entry's header while focused.
func (m Model) SetFocused(focused bool) Model {
	if m.focused == focused {
		return m
	}
	m.focused = focused
	if m.mode == modeRecap {
		m.dirty = true
		if m.visible {
			return m.rerender()
		}
	}
	return m
}

// SetSize sets the content area's size.
func (m Model) SetSize(width, height int) Model {
	resized := width != m.width
	m.width, m.height = width, height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	if m.mode != modeRecap {
		return m
	}
	if !m.visible {
		if resized {
			m.dirty = true
		}
		return m
	}
	if resized || m.dirty {
		following := m.viewport.AtBottom()
		m = m.rerender()
		if following {
			m.viewport.GotoBottom()
		}
	}
	return m
}

// SetTheme replaces the theme.
func (m Model) SetTheme(theme common.Theme) Model {
	m.theme = theme
	if m.mode == modeRecap {
		m.dirty = true
		if m.visible {
			return m.rerender()
		}
	}
	return m
}

// Reset clears per-session expansion state when switching to another session.
func (m Model) Reset() Model {
	m.sessionID = ""
	m.expanded = map[entryKey]bool{}
	m.hasRecap = false
	m.cursor = 0
	return m
}

// SetPlaceholder replaces the view with a centered message.
func (m Model) SetPlaceholder(text string) Model {
	m.mode = modePlaceholder
	m.placeholder = text
	m.rawContent = ""
	m.hasRecap = false
	m.lines, m.spans = nil, nil
	m.dirty = false
	m.viewport.SetContent("")
	return m
}

// SetContent shows pre-rendered text (such as a selected subagent's recap),
// scrolling to the top when entering raw content mode.
func (m Model) SetContent(content string) Model {
	wasRaw := m.mode == modeRaw
	m.mode = modeRaw
	m.placeholder = ""
	m.rawContent = content
	m.hasRecap = false
	m.lines, m.spans = nil, nil
	m.dirty = false
	m.viewport.SetContent(content)
	if !wasRaw {
		m.viewport.GotoTop()
	}
	return m
}

// SetRecap shows sessionID's conversation entries. Switching sessions opens at
// the newest entry; updates to the shown session preserve expanded cards and
// follow new entries while scrolled to the bottom.
func (m Model) SetRecap(sessionID string, r recap.Recap) Model {
	newSession := sessionID != m.sessionID
	wasRecap := m.mode == modeRecap && m.hasRecap && !newSession
	if wasRecap && m.recap.Equal(r) && !m.dirty {
		return m
	}
	following := !wasRecap || m.viewport.AtBottom()
	if newSession {
		m.sessionID = sessionID
		m.expanded = map[entryKey]bool{}
	}
	m.mode = modeRecap
	m.placeholder = ""
	m.rawContent = ""
	m.hasRecap = true
	m.recap = r
	m.dirty = true

	if len(r.Entries) == 0 {
		m.cursor = 0
	} else if !wasRecap || following {
		m.cursor = len(r.Entries) - 1
	} else if m.cursor >= len(r.Entries) {
		m.cursor = len(r.Entries) - 1
	}

	if !m.visible {
		m.lines, m.spans = nil, nil
		return m
	}
	m = m.rerender()
	if following {
		m.viewport.GotoBottom()
	}
	return m
}

// CurrentEntry returns the active entry index, or -1 when no entries are shown.
func (m Model) CurrentEntry() int {
	if m.mode != modeRecap || len(m.recap.Entries) == 0 {
		return -1
	}
	return m.cursor
}

// EntryCount returns the number of entries shown, or 0 outside recap mode.
func (m Model) EntryCount() int {
	if m.mode != modeRecap {
		return 0
	}
	return len(m.recap.Entries)
}

// AtBottom reports whether the end of the content is in view.
func (m Model) AtBottom() bool { return m.viewport.AtBottom() }

// GotoTop scrolls to the top.
func (m Model) GotoTop() Model {
	m.viewport.GotoTop()
	m = m.syncCursorToView()
	return m
}

// GotoBottom scrolls to the end.
func (m Model) GotoBottom() Model {
	m.viewport.GotoBottom()
	if m.mode == modeRecap && len(m.recap.Entries) > 0 {
		m = m.setCursor(len(m.recap.Entries) - 1)
	}
	return m
}

// Clickable reports whether a click on view row y toggles an entry: any row of
// a truncated entry expands it, and the header row of an expanded entry folds
// it back to PreviewLines.
func (m Model) Clickable(viewY int) bool {
	if m.mode != modeRecap || viewY < 0 || viewY >= m.height {
		return false
	}
	_, toggle, ok := m.entryAt(m.viewport.YOffset() + viewY)
	return ok && toggle
}

func (m Model) entryAt(lineY int) (idx int, toggle, ok bool) {
	for i, sp := range m.spans {
		if lineY >= sp.start && lineY < sp.end {
			if !sp.collapsible {
				return i, false, true
			}
			if !sp.expanded || lineY == sp.start {
				return i, true, true
			}
			return i, false, true
		}
	}
	return 0, false, false
}

func (m Model) click(viewY int) Model {
	if m.mode != modeRecap || viewY < 0 || viewY >= m.height {
		return m
	}
	lineY := m.viewport.YOffset() + viewY
	i, toggle, ok := m.entryAt(lineY)
	if !ok {
		return m
	}
	m.cursor = i
	if toggle {
		return m.toggleEntry(i)
	}
	m.dirty = true
	return m.rerender()
}

func (m Model) toggleEntry(i int) Model {
	if i < 0 || i >= len(m.recap.Entries) || i >= len(m.spans) {
		return m
	}
	if !m.spans[i].collapsible {
		return m
	}
	oldStart := m.spans[i].start
	screenRow := oldStart - m.viewport.YOffset()

	expanded := maps.Clone(m.expanded)
	k := keyOf(m.recap.Entries[i])
	if expanded[k] {
		delete(expanded, k)
	} else {
		expanded[k] = true
	}
	m.expanded = expanded
	m.dirty = true
	m = m.rerender()

	if screenRow >= 0 && screenRow < m.height {
		m.viewport.SetYOffset(max(0, m.spans[i].start-screenRow))
	} else {
		m.viewport.SetYOffset(m.spans[i].start)
	}
	return m
}

func (m Model) setAllExpanded(expand bool) Model {
	if m.mode != modeRecap || len(m.recap.Entries) == 0 {
		return m
	}
	anchor := m.cursor
	screenRow := 0
	if anchor >= 0 && anchor < len(m.spans) {
		screenRow = max(0, min(m.height-1, m.spans[anchor].start-m.viewport.YOffset()))
	}
	expanded := map[entryKey]bool{}
	if expand {
		for _, e := range m.recap.Entries {
			expanded[keyOf(e)] = true
		}
	}
	m.expanded = expanded
	m.dirty = true
	m = m.rerender()
	if anchor >= 0 && anchor < len(m.spans) {
		m.viewport.SetYOffset(max(0, m.spans[anchor].start-screenRow))
	}
	return m
}

func (m Model) setCursor(i int) Model {
	if i < 0 || i >= len(m.recap.Entries) || i == m.cursor {
		return m
	}
	m.cursor = i
	if m.focused {
		m.dirty = true
		m = m.rerender()
	}
	return m
}

func (m Model) ensureVisible(i int) Model {
	if i < 0 || i >= len(m.spans) || m.height <= 0 {
		return m
	}
	sp := m.spans[i]
	top := m.viewport.YOffset()
	bottom := top + m.height
	if sp.start < top {
		m.viewport.SetYOffset(sp.start)
		return m
	}
	if sp.end > bottom {
		if sp.end-sp.start <= m.height {
			m.viewport.SetYOffset(sp.end - m.height)
		} else {
			m.viewport.SetYOffset(sp.start)
		}
	}
	return m
}

func (m Model) syncCursorToView() Model {
	if m.mode != modeRecap || len(m.spans) == 0 || m.height <= 0 {
		return m
	}
	top := m.viewport.YOffset()
	bottom := top + m.height
	if m.cursor >= 0 && m.cursor < len(m.spans) {
		sp := m.spans[m.cursor]
		if sp.end > top && sp.start < bottom {
			return m
		}
	}
	if top == 0 {
		return m.setCursor(0)
	}
	if m.viewport.AtBottom() {
		return m.setCursor(len(m.spans) - 1)
	}
	for i, sp := range m.spans {
		if sp.end > top && sp.start < bottom {
			return m.setCursor(i)
		}
	}
	return m
}

// Update handles keys while the pane has focus, clicks, and the mouse wheel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return m.click(msg.Y), nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.viewport.ScrollUp(3)
		case tea.MouseWheelDown:
			m.viewport.ScrollDown(3)
		}
		return m.syncCursorToView(), nil
	case tea.KeyPressMsg:
		keys := common.Keys.Pane
		switch {
		case key.Matches(msg, keys.Back):
			return m, func() tea.Msg { return common.LeavePaneMsg{} }
		case key.Matches(msg, keys.NextTab, keys.PrevTab):
			back := key.Matches(msg, keys.PrevTab)
			return m, func() tea.Msg { return common.SwitchTabMsg{Back: back} }
		case key.Matches(msg, keys.Up):
			m.viewport.ScrollUp(1)
			return m.syncCursorToView(), nil
		case key.Matches(msg, keys.Down):
			m.viewport.ScrollDown(1)
			return m.syncCursorToView(), nil
		case key.Matches(msg, keys.HalfUp):
			m.viewport.HalfPageUp()
			return m.syncCursorToView(), nil
		case key.Matches(msg, keys.HalfDown):
			m.viewport.HalfPageDown()
			return m.syncCursorToView(), nil
		case key.Matches(msg, keys.Top):
			return m.GotoTop(), nil
		case key.Matches(msg, keys.Bottom):
			return m.GotoBottom(), nil
		case m.mode == modeRecap && key.Matches(msg, keys.Fold):
			return m.toggleEntry(m.cursor), nil
		case m.mode == modeRecap && key.Matches(msg, keys.FoldAll):
			return m.setAllExpanded(false), nil
		case m.mode == modeRecap && key.Matches(msg, keys.UnfoldAll):
			return m.setAllExpanded(true), nil
		case m.mode == modeRecap && key.Matches(msg, keys.NextFile):
			if m.cursor+1 < len(m.recap.Entries) {
				m = m.setCursor(m.cursor + 1)
				m = m.ensureVisible(m.cursor)
			}
			return m, nil
		case m.mode == modeRecap && key.Matches(msg, keys.PrevFile):
			if m.cursor > 0 {
				m = m.setCursor(m.cursor - 1)
				m = m.ensureVisible(m.cursor)
			} else {
				m.viewport.GotoTop()
			}
			return m, nil
		}
	}
	return m, nil
}

// View renders the content area at its set size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.mode == modePlaceholder {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.StyleSubtitle.Render(m.placeholder))
	}
	return m.viewport.View()
}

func (m Model) rerender() Model {
	if !m.visible || m.width <= 0 || m.mode != modeRecap {
		return m
	}
	m.lines, m.spans = renderRecap(m.recap, m.expanded, m.cursor, m.focused, m.width, m.theme)
	m.dirty = false
	m.viewport.SetContentLines(m.lines)
	return m
}

func renderRecap(r recap.Recap, expanded map[entryKey]bool, cursor int, focused bool, width int, t common.Theme) ([]string, []entrySpan) {
	inner := max(1, width-2)
	var lines []string
	if r.Title != "" {
		for _, tl := range strings.Split(ansi.Wrap(r.Title, inner, ""), "\n") {
			lines = append(lines, " "+t.StyleTitle.Render(tl))
		}
	}
	if counts := renderCounts(r.Entries, t); counts != "" {
		lines = append(lines, " "+counts)
	}
	if len(lines) > 0 {
		lines = append(lines, " "+t.StyleSeparator.Render(strings.Repeat("─", inner)), "")
	}

	accentFg := t.ColorAccent
	readyFg := t.StatusColor(session.StatusReady)
	selectedBg := t.StyleSelected.GetBackground()
	promptBox := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorText)
	promptHint := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorMuted)
	textStyle := lipgloss.NewStyle().Foreground(t.ColorText)
	accentBar := lipgloss.NewStyle().Foreground(accentFg).Render("│ ")

	spans := make([]entrySpan, len(r.Entries))
	for i, e := range r.Entries {
		if i > 0 {
			lines = append(lines, "")
		}
		var wrapWidth int
		switch e.Kind {
		case recap.KindPrompt:
			wrapWidth = max(1, inner-4)
		default:
			wrapWidth = max(1, inner-2)
		}
		wrapped := strings.Split(ansi.Wrap(e.Text, wrapWidth, ""), "\n")
		collapsible := len(wrapped) > PreviewLines
		isExpanded := !collapsible || expanded[keyOf(e)]

		var headerBg color.Color
		if focused && i == cursor {
			headerBg = selectedBg
		}
		start := len(lines)
		lines = append(lines, " "+renderEntryHeader(e, collapsible, isExpanded, inner, headerBg, accentFg, readyFg, t))

		shown := wrapped
		if !isExpanded {
			shown = wrapped[:PreviewLines]
		}
		switch e.Kind {
		case recap.KindPrompt:
			for _, wl := range shown {
				pad := max(0, inner-2-ansi.StringWidth(wl))
				lines = append(lines, " "+promptBox.Render("  "+wl+strings.Repeat(" ", pad)))
			}
			if !isExpanded {
				more := moreLabel(len(wrapped) - PreviewLines)
				pad := max(0, inner-2-ansi.StringWidth(more))
				lines = append(lines, " "+promptHint.Render("  "+more+strings.Repeat(" ", pad)))
			}
		case recap.KindRecap:
			for _, wl := range shown {
				lines = append(lines, " "+accentBar+textStyle.Render(wl))
			}
			if !isExpanded {
				lines = append(lines, " "+accentBar+t.StyleSubtitle.Render(moreLabel(len(wrapped)-PreviewLines)))
			}
		default: // KindResponse
			for _, wl := range shown {
				lines = append(lines, "   "+textStyle.Render(wl))
			}
			if !isExpanded {
				lines = append(lines, "   "+t.StyleSubtitle.Render(moreLabel(len(wrapped)-PreviewLines)))
			}
		}
		spans[i] = entrySpan{
			start:       start,
			end:         len(lines),
			collapsible: collapsible,
			expanded:    isExpanded,
		}
	}
	return lines, spans
}

func renderEntryHeader(e recap.Entry, collapsible, expanded bool, width int, bg, accentFg, readyFg color.Color, t common.Theme) string {
	on := func(fg color.Color, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle()
		if fg != nil {
			s = s.Foreground(fg)
		}
		if bold {
			s = s.Bold(true)
		}
		if bg != nil {
			s = s.Background(bg)
		}
		return s
	}

	var left string
	if collapsible {
		chevron := "▸ "
		if expanded {
			chevron = "▾ "
		}
		left += on(accentFg, false).Render(chevron)
	}
	switch e.Kind {
	case recap.KindPrompt:
		left += on(accentFg, false).Render("❯ ") + on(t.ColorText, true).Render("You")
	case recap.KindResponse:
		left += on(readyFg, false).Render("● ") + on(t.ColorText, true).Render("Response")
	case recap.KindRecap:
		left += on(accentFg, true).Render("✻ Recap")
	}

	right := on(t.ColorFaint, false).Render(e.Time.Local().Format("Jan 2 15:04"))
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return ansi.Truncate(left, width, "…")
	}
	return left + on(nil, false).Render(strings.Repeat(" ", gap)) + right
}

func renderCounts(entries []recap.Entry, t common.Theme) string {
	var prompts, responses, recaps int
	for _, e := range entries {
		switch e.Kind {
		case recap.KindPrompt:
			prompts++
		case recap.KindResponse:
			responses++
		case recap.KindRecap:
			recaps++
		}
	}
	var parts []string
	add := func(n int, singular, plural string) {
		if n == 0 {
			return
		}
		noun := plural
		if n == 1 {
			noun = singular
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, noun))
	}
	add(prompts, "prompt", "prompts")
	add(responses, "response", "responses")
	add(recaps, "recap", "recaps")
	if len(parts) == 0 {
		return ""
	}
	return t.StyleSubtitle.Render(strings.Join(parts, " · "))
}

func moreLabel(hidden int) string {
	if hidden == 1 {
		return "… 1 more line"
	}
	return fmt.Sprintf("… %d more lines", hidden)
}

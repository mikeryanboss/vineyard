// Package recapview renders a session's conversation on the Recap tab: its
// title and interactive filter/collapse chips, followed by user prompts, final
// agent responses, and away recaps. Entries can be shown as 4-line previews,
// collapsed 1-line headers, fully expanded cards, or hidden via configuration.
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
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// PreviewLines is how many wrapped body lines an entry in preview mode shows
// before truncating with a "▸ … N more lines" hint.
const PreviewLines = 4

type viewMode int

const (
	modePlaceholder viewMode = iota
	modeRaw
	modeRecap
)

type entryState int

const (
	stateCollapsed entryState = iota // 1-line header only
	statePreview                     // up to PreviewLines wrapped lines
	stateExpanded                    // all wrapped lines
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
	start int // header line index in m.lines
	end   int // exclusive end line index in m.lines
	long  bool
	state entryState
}

type chipAction int

const (
	chipTogglePrompts chipAction = iota
	chipToggleResponses
	chipToggleRecaps
	chipExpandAll
	chipCollapseAll
)

type chipSpan struct {
	line   int
	startX int
	endX   int
	action chipAction
}

// Model is the recap pane's content area.
type Model struct {
	width       int
	height      int
	visible     bool
	focused     bool
	dirty       bool
	theme       common.Theme
	cfg         config.RecapConfig
	mode        viewMode
	placeholder string
	rawContent  string

	sessionID   string
	hasRecap    bool
	recap       recap.Recap
	entries     []recap.Entry
	states      map[entryKey]entryState
	cursor      int
	headerLines []string
	chips       []chipSpan
	lines       []string
	spans       []entrySpan
	viewport    viewport.Model
}

// New returns an empty recap view with default recap settings.
func New(theme common.Theme) Model {
	return Model{
		visible:  true,
		theme:    theme,
		cfg:      config.DefaultRecap(),
		states:   map[entryKey]entryState{},
		viewport: viewport.New(),
	}
}

// SetConfig updates which entry kinds are shown and their default fold state.
func (m Model) SetConfig(cfg config.RecapConfig) Model {
	if m.cfg == cfg {
		return m
	}
	m.cfg = cfg
	if m.mode != modeRecap || !m.hasRecap {
		return m
	}
	m.entries = filterEntries(m.recap.Entries, m.cfg)
	if len(m.entries) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	m.dirty = true
	if m.visible {
		return m.rerender()
	}
	return m
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
	resized := width != m.width || height != m.height
	m.width, m.height = width, height
	m.viewport.SetWidth(width)
	if m.mode != modeRecap {
		m.viewport.SetHeight(height)
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
	m.states = map[entryKey]entryState{}
	m.hasRecap = false
	m.entries = nil
	m.headerLines = nil
	m.chips = nil
	m.cursor = 0
	return m
}

// SetPlaceholder replaces the view with a centered message.
func (m Model) SetPlaceholder(text string) Model {
	m.mode = modePlaceholder
	m.placeholder = text
	m.rawContent = ""
	m.hasRecap = false
	m.entries = nil
	m.headerLines, m.chips = nil, nil
	m.lines, m.spans = nil, nil
	m.dirty = false
	m.viewport.SetHeight(m.height)
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
	m.entries = nil
	m.headerLines, m.chips = nil, nil
	m.lines, m.spans = nil, nil
	m.dirty = false
	m.viewport.SetHeight(m.height)
	m.viewport.SetContent(content)
	if !wasRaw {
		m.viewport.GotoTop()
	}
	return m
}

// SetRecap shows sessionID's conversation entries. Switching sessions opens at
// the newest entry; updates to the shown session preserve per-entry state and
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
		m.states = map[entryKey]entryState{}
	}
	m.mode = modeRecap
	m.placeholder = ""
	m.rawContent = ""
	m.hasRecap = true
	m.recap = r
	m.entries = filterEntries(r.Entries, m.cfg)
	m.dirty = true

	if len(m.entries) == 0 {
		m.cursor = 0
	} else if !wasRecap || following {
		m.cursor = len(m.entries) - 1
	} else if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}

	if !m.visible {
		m.headerLines, m.chips = nil, nil
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
	if m.mode != modeRecap || len(m.entries) == 0 {
		return -1
	}
	return m.cursor
}

// EntryCount returns the number of visible entries shown, or 0 outside recap mode.
func (m Model) EntryCount() int {
	if m.mode != modeRecap {
		return 0
	}
	return len(m.entries)
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
	if m.mode == modeRecap && len(m.entries) > 0 {
		m = m.setCursor(len(m.entries) - 1)
	}
	return m
}

// Clickable reports whether a click at (viewX, viewY) triggers an action: a
// header chip button, any entry's header row (to collapse/open), or any body
// row of a truncated preview entry (to expand in full).
func (m Model) Clickable(viewX, viewY int) bool {
	if m.mode != modeRecap || len(m.entries) == 0 || viewY < 0 || viewY >= m.height {
		return false
	}
	hdrH := len(m.headerLines)
	if viewY < hdrH {
		_, ok := m.chipAt(viewX, viewY)
		return ok
	}
	lineY := m.viewport.YOffset() + (viewY - hdrH)
	_, _, toggle, ok := m.entryAt(lineY)
	return ok && toggle
}

func (m Model) chipAt(x, y int) (chipAction, bool) {
	for _, c := range m.chips {
		if c.line == y && x >= c.startX && x < c.endX {
			return c.action, true
		}
	}
	return 0, false
}

func (m Model) entryAt(lineY int) (idx int, onHeader, toggle, ok bool) {
	for i, sp := range m.spans {
		if lineY >= sp.start && lineY < sp.end {
			if lineY == sp.start {
				return i, true, true, true
			}
			if sp.state == statePreview && sp.long {
				return i, false, true, true
			}
			return i, false, false, true
		}
	}
	return 0, false, false, false
}

func (m Model) click(viewX, viewY int) Model {
	if m.mode != modeRecap || len(m.entries) == 0 || viewY < 0 || viewY >= m.height {
		return m
	}
	hdrH := len(m.headerLines)
	if viewY < hdrH {
		act, ok := m.chipAt(viewX, viewY)
		if !ok {
			return m
		}
		switch act {
		case chipTogglePrompts:
			return m.toggleKind(recap.KindPrompt)
		case chipToggleResponses:
			return m.toggleKind(recap.KindResponse)
		case chipToggleRecaps:
			return m.toggleKind(recap.KindRecap)
		case chipExpandAll:
			return m.setAllState(stateExpanded)
		case chipCollapseAll:
			return m.setAllState(stateCollapsed)
		}
		return m
	}

	lineY := m.viewport.YOffset() + (viewY - hdrH)
	i, onHeader, toggle, ok := m.entryAt(lineY)
	if !ok {
		return m
	}
	m.cursor = i
	if !toggle {
		m.dirty = true
		return m.rerender()
	}
	sp := m.spans[i]
	if !onHeader {
		// Clicking a truncated preview body or "… N more lines" expands in full.
		return m.setEntryState(i, stateExpanded)
	}
	// Clicking an entry's header:
	// - Collapsed (▸) -> open to configured non-collapsed state (Preview by default).
	// - Expanded (▾) on a long entry whose default is Preview -> fold back to Preview.
	// - Otherwise (Preview or short Expanded) -> collapse to 1-line header.
	switch sp.state {
	case stateCollapsed:
		return m.setEntryState(i, m.uncollapsedState(m.entries[i].Kind))
	case stateExpanded:
		if sp.long && m.defaultState(m.entries[i].Kind) == statePreview {
			return m.setEntryState(i, statePreview)
		}
		return m.setEntryState(i, stateCollapsed)
	default:
		return m.setEntryState(i, stateCollapsed)
	}
}

func (m Model) cycleEntry(i int) Model {
	if i < 0 || i >= len(m.entries) || i >= len(m.spans) {
		return m
	}
	sp := m.spans[i]
	var next entryState
	if sp.long {
		switch sp.state {
		case statePreview:
			next = stateExpanded
		case stateExpanded:
			next = stateCollapsed
		default:
			next = statePreview
		}
	} else {
		if sp.state == stateCollapsed {
			next = stateExpanded
		} else {
			next = stateCollapsed
		}
	}
	return m.setEntryState(i, next)
}

func (m Model) setEntryState(i int, next entryState) Model {
	if i < 0 || i >= len(m.entries) || i >= len(m.spans) {
		return m
	}
	oldStart := m.spans[i].start
	vpRow := oldStart - m.viewport.YOffset()

	states := maps.Clone(m.states)
	k := keyOf(m.entries[i])
	if next == m.defaultState(m.entries[i].Kind) {
		delete(states, k)
	} else {
		states[k] = next
	}
	m.states = states
	m.dirty = true
	m = m.rerender()

	vpH := m.viewport.Height()
	if vpRow >= 0 && vpRow < vpH {
		m.viewport.SetYOffset(max(0, m.spans[i].start-vpRow))
	} else {
		m.viewport.SetYOffset(m.spans[i].start)
	}
	if next != stateCollapsed {
		m = m.ensureVisible(i)
	}
	return m
}

func (m Model) toggleKind(kind recap.Kind) Model {
	if m.mode != modeRecap || len(m.entries) == 0 {
		return m
	}
	hasKind := false
	allCollapsed := true
	for _, e := range m.entries {
		if e.Kind != kind {
			continue
		}
		hasKind = true
		if m.stateOf(e) != stateCollapsed {
			allCollapsed = false
			break
		}
	}
	if !hasKind {
		return m
	}
	target := stateCollapsed
	if allCollapsed {
		target = m.uncollapsedState(kind)
	}
	return m.applyAnchor(func(states map[entryKey]entryState) {
		def := m.defaultState(kind)
		for _, e := range m.entries {
			if e.Kind != kind {
				continue
			}
			k := keyOf(e)
			if target == def {
				delete(states, k)
			} else {
				states[k] = target
			}
		}
	})
}

func (m Model) setAllState(target entryState) Model {
	if m.mode != modeRecap || len(m.entries) == 0 {
		return m
	}
	return m.applyAnchor(func(states map[entryKey]entryState) {
		for _, e := range m.entries {
			k := keyOf(e)
			if target == m.defaultState(e.Kind) {
				delete(states, k)
			} else {
				states[k] = target
			}
		}
	})
}

func (m Model) applyAnchor(mutate func(map[entryKey]entryState)) Model {
	anchor := m.cursor
	vpRow := 0
	vpH := max(1, m.viewport.Height())
	if anchor >= 0 && anchor < len(m.spans) {
		vpRow = max(0, min(vpH-1, m.spans[anchor].start-m.viewport.YOffset()))
	}
	states := maps.Clone(m.states)
	mutate(states)
	m.states = states
	m.dirty = true
	m = m.rerender()
	if anchor >= 0 && anchor < len(m.spans) {
		m.viewport.SetYOffset(max(0, m.spans[anchor].start-vpRow))
	}
	return m
}

func (m Model) defaultState(kind recap.Kind) entryState {
	switch modeForKind(m.cfg, kind) {
	case config.RecapCollapsed:
		return stateCollapsed
	case config.RecapExpanded:
		return stateExpanded
	default:
		return statePreview
	}
}

func (m Model) uncollapsedState(kind recap.Kind) entryState {
	if m.defaultState(kind) == stateExpanded {
		return stateExpanded
	}
	return statePreview
}

func (m Model) stateOf(e recap.Entry) entryState {
	if st, ok := m.states[keyOf(e)]; ok {
		return st
	}
	return m.defaultState(e.Kind)
}

func (m Model) setCursor(i int) Model {
	if i < 0 || i >= len(m.entries) || i == m.cursor {
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
	vpH := m.viewport.Height()
	if i < 0 || i >= len(m.spans) || vpH <= 0 {
		return m
	}
	sp := m.spans[i]
	top := m.viewport.YOffset()
	bottom := top + vpH
	if sp.start < top {
		m.viewport.SetYOffset(sp.start)
		return m
	}
	if sp.end > bottom {
		if sp.end-sp.start <= vpH {
			m.viewport.SetYOffset(sp.end - vpH)
		} else {
			m.viewport.SetYOffset(sp.start)
		}
	}
	return m
}

func (m Model) syncCursorToView() Model {
	vpH := m.viewport.Height()
	if m.mode != modeRecap || len(m.spans) == 0 || vpH <= 0 {
		return m
	}
	top := m.viewport.YOffset()
	bottom := top + vpH
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
		return m.click(msg.X, msg.Y), nil
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
			return m.cycleEntry(m.cursor), nil
		case m.mode == modeRecap && key.Matches(msg, keys.FoldAll):
			return m.setAllState(stateCollapsed), nil
		case m.mode == modeRecap && key.Matches(msg, keys.UnfoldAll):
			return m.setAllState(stateExpanded), nil
		case m.mode == modeRecap && key.Matches(msg, keys.FoldPrompts):
			return m.toggleKind(recap.KindPrompt), nil
		case m.mode == modeRecap && key.Matches(msg, keys.FoldResponses):
			return m.toggleKind(recap.KindResponse), nil
		case m.mode == modeRecap && key.Matches(msg, keys.FoldRecaps):
			return m.toggleKind(recap.KindRecap), nil
		case m.mode == modeRecap && key.Matches(msg, keys.NextFile):
			if m.cursor+1 < len(m.entries) {
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
	if m.mode == modeRecap && len(m.entries) == 0 {
		msg := "Nothing said in this session yet."
		if len(m.recap.Entries) > 0 {
			msg = "All entries in this session are hidden by recap settings."
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.StyleSubtitle.Render(msg))
	}
	if len(m.headerLines) > 0 {
		return strings.Join(m.headerLines, "\n") + "\n" + m.viewport.View()
	}
	return m.viewport.View()
}

func (m Model) rerender() Model {
	if !m.visible || m.width <= 0 || m.mode != modeRecap {
		return m
	}
	if len(m.entries) == 0 {
		m.headerLines, m.chips = nil, nil
		m.lines, m.spans = nil, nil
		m.dirty = false
		m.viewport.SetHeight(m.height)
		m.viewport.SetContent("")
		return m
	}
	hdr, chips := renderHeaderBar(m.recap.Title, m.entries, m.stateOf, m.width, m.theme)
	body, spans := renderEntries(m.entries, m.stateOf, m.cursor, m.focused, m.width, m.theme)
	if m.height > 0 && m.height < len(hdr)+4 {
		// In a very short viewport, fold the header lines into the scrollable body
		// so entries still have room to scroll.
		offset := len(hdr)
		for i := range spans {
			spans[i].start += offset
			spans[i].end += offset
		}
		m.headerLines, m.chips = nil, nil
		m.lines = append(hdr, body...)
		m.spans = spans
		m.viewport.SetHeight(m.height)
	} else {
		m.headerLines, m.chips = hdr, chips
		m.lines, m.spans = body, spans
		m.viewport.SetHeight(max(1, m.height-len(hdr)))
	}
	m.dirty = false
	m.viewport.SetWidth(m.width)
	m.viewport.SetContentLines(m.lines)
	return m
}

func modeForKind(cfg config.RecapConfig, kind recap.Kind) string {
	var v string
	switch kind {
	case recap.KindPrompt:
		v = cfg.Prompts
	case recap.KindResponse:
		v = cfg.Responses
	case recap.KindRecap:
		v = cfg.Recaps
	}
	if v == "" {
		return config.RecapPreview
	}
	return v
}

func filterEntries(entries []recap.Entry, cfg config.RecapConfig) []recap.Entry {
	out := make([]recap.Entry, 0, len(entries))
	for _, e := range entries {
		if modeForKind(cfg, e.Kind) != config.RecapOff {
			out = append(out, e)
		}
	}
	return out
}

type renderedChip struct {
	text   string
	width  int
	action chipAction
}

func renderHeaderBar(title string, entries []recap.Entry, stateOf func(recap.Entry) entryState, width int, t common.Theme) ([]string, []chipSpan) {
	inner := max(1, width-2)
	var lines []string
	if title != "" {
		for _, tl := range strings.Split(ansi.Wrap(title, inner, ""), "\n") {
			lines = append(lines, " "+t.StyleTitle.Render(tl))
		}
	}

	var prompts, responses, recaps int
	allPromptsCollapsed, allResponsesCollapsed, allRecapsCollapsed := true, true, true
	allCollapsed, allExpanded := true, true

	for _, e := range entries {
		st := stateOf(e)
		if st != stateCollapsed {
			allCollapsed = false
		}
		if st != stateExpanded {
			allExpanded = false
		}
		switch e.Kind {
		case recap.KindPrompt:
			prompts++
			if st != stateCollapsed {
				allPromptsCollapsed = false
			}
		case recap.KindResponse:
			responses++
			if st != stateCollapsed {
				allResponsesCollapsed = false
			}
		case recap.KindRecap:
			recaps++
			if st != stateCollapsed {
				allRecapsCollapsed = false
			}
		}
	}

	pillActive := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorText)
	pillMuted := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorMuted)
	pillAccent := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorAccent)
	pillReady := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.StatusColor(session.StatusReady))

	makeKindChip := func(n int, singular, plural string, collapsed bool, iconFg lipgloss.Style, action chipAction) (renderedChip, bool) {
		if n == 0 {
			return renderedChip{}, false
		}
		noun := plural
		if n == 1 {
			noun = singular
		}
		label := fmt.Sprintf("%d %s", n, noun)
		if collapsed {
			raw := " ▸ " + label + " "
			return renderedChip{text: pillMuted.Render(raw), width: ansi.StringWidth(raw), action: action}, true
		}
		txt := pillActive.Render(" ") + iconFg.Render("▾ ") + pillActive.Render(label+" ")
		return renderedChip{text: txt, width: 3 + ansi.StringWidth(label) + 1, action: action}, true
	}

	makeActionChip := func(label string, dimmed bool, action chipAction) renderedChip {
		raw := " " + label + " "
		style := pillActive
		if dimmed {
			style = pillMuted
		}
		return renderedChip{text: style.Render(raw), width: ansi.StringWidth(raw), action: action}
	}

	var kindChips []renderedChip
	if c, ok := makeKindChip(prompts, "prompt", "prompts", allPromptsCollapsed, pillAccent, chipTogglePrompts); ok {
		kindChips = append(kindChips, c)
	}
	if c, ok := makeKindChip(responses, "response", "responses", allResponsesCollapsed, pillReady, chipToggleResponses); ok {
		kindChips = append(kindChips, c)
	}
	if c, ok := makeKindChip(recaps, "recap", "recaps", allRecapsCollapsed, pillAccent, chipToggleRecaps); ok {
		kindChips = append(kindChips, c)
	}

	actionChips := []renderedChip{
		makeActionChip("expand all", allExpanded, chipExpandAll),
		makeActionChip("collapse all", allCollapsed, chipCollapseAll),
	}

	var spans []chipSpan
	var curLine strings.Builder
	curCol := 0
	placeChip := func(c renderedChip, gap int) {
		if curCol > 1 && curCol+gap+c.width > width-1 {
			lines = append(lines, curLine.String())
			curLine.Reset()
			curCol = 0
		}
		if curCol == 0 {
			curLine.WriteByte(' ')
			curCol = 1
		} else if gap > 0 {
			curLine.WriteString(strings.Repeat(" ", gap))
			curCol += gap
		}
		lineIdx := len(lines)
		spans = append(spans, chipSpan{
			line:   lineIdx,
			startX: curCol,
			endX:   curCol + c.width,
			action: c.action,
		})
		curLine.WriteString(c.text)
		curCol += c.width
	}

	for i, c := range kindChips {
		gap := 1
		if i == 0 {
			gap = 0
		}
		placeChip(c, gap)
	}
	for i, c := range actionChips {
		gap := 1
		if i == 0 {
			gap = 2
		}
		placeChip(c, gap)
	}
	if curCol > 0 {
		lines = append(lines, curLine.String())
	}

	lines = append(lines, " "+t.StyleSeparator.Render(strings.Repeat("─", inner)))
	return lines, spans
}

func renderEntries(entries []recap.Entry, stateOf func(recap.Entry) entryState, cursor int, focused bool, width int, t common.Theme) ([]string, []entrySpan) {
	inner := max(1, width-2)
	accentFg := t.ColorAccent
	readyFg := t.StatusColor(session.StatusReady)
	selectedBg := t.StyleSelected.GetBackground()
	promptBox := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorText)
	promptHint := lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorMuted)
	textStyle := lipgloss.NewStyle().Foreground(t.ColorText)
	accentBar := lipgloss.NewStyle().Foreground(accentFg).Render("│ ")

	var lines []string
	spans := make([]entrySpan, len(entries))
	for i, e := range entries {
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
		long := len(wrapped) > PreviewLines
		st := stateOf(e)

		var headerBg color.Color
		if focused && i == cursor {
			headerBg = selectedBg
		}
		start := len(lines)
		lines = append(lines, " "+renderEntryHeader(e, st == stateCollapsed, inner, headerBg, accentFg, readyFg, t))

		if st != stateCollapsed {
			shown := wrapped
			truncated := false
			if st == statePreview && long {
				shown = wrapped[:PreviewLines]
				truncated = true
			}
			switch e.Kind {
			case recap.KindPrompt:
				for _, wl := range shown {
					pad := max(0, inner-2-ansi.StringWidth(wl))
					lines = append(lines, " "+promptBox.Render("  "+wl+strings.Repeat(" ", pad)))
				}
				if truncated {
					more := moreLabel(len(wrapped) - PreviewLines)
					pad := max(0, inner-2-ansi.StringWidth(more))
					lines = append(lines, " "+promptHint.Render("  "+more+strings.Repeat(" ", pad)))
				}
			case recap.KindRecap:
				for _, wl := range shown {
					lines = append(lines, " "+accentBar+textStyle.Render(wl))
				}
				if truncated {
					lines = append(lines, " "+accentBar+t.StyleSubtitle.Render(moreLabel(len(wrapped)-PreviewLines)))
				}
			default: // KindResponse
				for _, wl := range shown {
					lines = append(lines, "   "+textStyle.Render(wl))
				}
				if truncated {
					lines = append(lines, "   "+t.StyleSubtitle.Render(moreLabel(len(wrapped)-PreviewLines)))
				}
			}
		}

		spans[i] = entrySpan{
			start: start,
			end:   len(lines),
			long:  long,
			state: st,
		}
	}
	return lines, spans
}

func renderEntryHeader(e recap.Entry, collapsed bool, width int, bg, accentFg, readyFg color.Color, t common.Theme) string {
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

	chevron := "▾ "
	if collapsed {
		chevron = "▸ "
	}
	left := on(accentFg, false).Render(chevron)
	switch e.Kind {
	case recap.KindPrompt:
		left += on(accentFg, false).Render("❯ ") + on(t.ColorText, true).Render("You")
	case recap.KindResponse:
		left += on(readyFg, false).Render("● ") + on(t.ColorText, true).Render("Response")
	case recap.KindRecap:
		left += on(accentFg, true).Render("✻ Recap")
	}

	right := on(t.ColorFaint, false).Render(e.Time.Local().Format("Jan 2 15:04"))
	leftW := ansi.StringWidth(left)
	rightW := ansi.StringWidth(right)
	if leftW+1+rightW > width {
		return ansi.Truncate(left, width, "…")
	}

	var middle string
	if collapsed {
		if snippet := firstLine(e.Text); snippet != "" {
			avail := width - leftW - rightW - 4
			if avail >= 6 {
				snip := ansi.Truncate(snippet, avail, "…")
				middle = on(t.ColorMuted, false).Render("  " + snip)
			}
		}
	}
	gap := max(1, width-leftW-ansi.StringWidth(middle)-rightW)
	return left + middle + on(nil, false).Render(strings.Repeat(" ", gap)) + right
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func moreLabel(hidden int) string {
	if hidden == 1 {
		return "▸ … 1 more line"
	}
	return fmt.Sprintf("▸ … %d more lines", hidden)
}

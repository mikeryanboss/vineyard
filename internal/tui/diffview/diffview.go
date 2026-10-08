// Package diffview renders a session's changes: a summary of changed files,
// then each file as a collapsible card of hunks with line numbers, syntax
// highlighting, and the changed words of edited lines marked.
package diffview

import (
	"fmt"
	"image/color"
	"maps"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/diff"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// maxHighlightedLines bounds syntax highlighting. Beyond it a diff is shown
// uncoloured, because tokenising tens of thousands of lines on every refresh
// would stall the UI.
const maxHighlightedLines = 4000

// Model is the diff pane's content area.
type Model struct {
	width      int
	height     int
	theme      common.Theme
	raw        string
	files      []diff.File
	lines      []string
	fileStarts []int           // rendered line index of each file's card
	collapsed  map[string]bool // paths of folded files
	viewport   viewport.Model
}

// New returns an empty diff view.
func New(theme common.Theme) Model {
	return Model{theme: theme, viewport: viewport.New(), collapsed: map[string]bool{}}
}

// SetDiff shows the diff in raw `git diff` form. Unchanged input is a no-op,
// so callers can refresh on a timer without re-rendering.
func (m Model) SetDiff(raw string) Model {
	if raw == m.raw && m.files != nil {
		return m
	}
	m.raw = raw
	m.files = diff.Parse(raw)
	if m.files == nil {
		m.files = []diff.File{}
	}
	return m.rerender()
}

// SetSize sets the content area's size.
func (m Model) SetSize(width, height int) Model {
	resized := width != m.width
	m.width, m.height = width, height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	if resized {
		return m.rerender()
	}
	return m.setContent()
}

// SetTheme replaces the theme.
func (m Model) SetTheme(theme common.Theme) Model {
	m.theme = theme
	return m.rerender()
}

// Files returns the parsed files.
func (m Model) Files() []diff.File { return m.files }

// CurrentFile returns the index of the file at the top of the view, or -1
// while the summary is showing.
func (m Model) CurrentFile() int {
	current := -1
	for i, start := range m.fileStarts {
		if start <= m.viewport.YOffset() {
			current = i
		}
	}
	return current
}

// fold changes which files are collapsed and keeps file anchor (an index into
// Files) at the top of the view, since its position moves when files above it
// shrink or grow. A negative anchor leaves the scroll position alone.
func (m Model) fold(anchor int, collapsed map[string]bool) Model {
	m.collapsed = collapsed
	m = m.rerender()
	if anchor >= 0 {
		m.viewport.SetYOffset(m.fileStarts[anchor])
	}
	return m
}

func (m Model) rerender() Model {
	if m.width <= 0 || m.files == nil {
		return m
	}
	m.lines, m.fileStarts = render(m.files, m.collapsed, m.width, m.theme)
	return m.setContent()
}

// setContent hands the rendered lines to the viewport, padded at the end so
// that jumping to the last file can scroll its header to the top.
func (m Model) setContent() Model {
	lines := m.lines
	if n := len(m.fileStarts); n > 0 {
		if pad := m.height - (len(lines) - m.fileStarts[n-1]); pad > 0 {
			lines = append(lines[:len(lines):len(lines)], make([]string, pad)...)
		}
	}
	m.viewport.SetContentLines(lines)
	return m
}

// Update handles keys while the pane has focus.
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
		case key.Matches(msg, keys.Toggle):
			if i := m.CurrentFile(); i >= 0 {
				collapsed := maps.Clone(m.collapsed)
				path := m.files[i].Path()
				if collapsed[path] {
					delete(collapsed, path)
				} else {
					collapsed[path] = true
				}
				return m.fold(i, collapsed), nil
			}
		case key.Matches(msg, keys.Collapse):
			collapsed := make(map[string]bool, len(m.files))
			for _, f := range m.files {
				collapsed[f.Path()] = true
			}
			return m.fold(max(m.CurrentFile(), 0), collapsed), nil
		case key.Matches(msg, keys.Expand):
			return m.fold(m.CurrentFile(), map[string]bool{}), nil
		case key.Matches(msg, keys.NextFile):
			for _, start := range m.fileStarts {
				if start > m.viewport.YOffset() {
					m.viewport.SetYOffset(start)
					break
				}
			}
		case key.Matches(msg, keys.PrevFile):
			target := 0
			for _, start := range m.fileStarts {
				if start < m.viewport.YOffset() {
					target = start
				}
			}
			m.viewport.SetYOffset(target)
		}
	}
	return m, nil
}

// View renders the content area at exactly its set size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.files) == 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.StyleSubtitle.Render("No changes yet"))
	}
	return m.viewport.View()
}

// render lays out the summary and every file as a card. It returns the lines
// and the index of each card's first line.
func render(files []diff.File, collapsed map[string]bool, width int, t common.Theme) ([]string, []int) {
	total := 0
	for _, f := range files {
		for _, h := range f.Hunks {
			total += len(h.Lines)
		}
	}
	highlight := total <= maxHighlightedLines

	lines := renderSummary(files, width, t)
	starts := make([]int, 0, len(files))
	for _, f := range files {
		starts = append(starts, len(lines))
		rule := lipgloss.NewStyle().Foreground(t.ColorFaint).Render(strings.Repeat("━", width))
		lines = append(lines, rule, renderFileHeader(f, collapsed[f.Path()], width, t), rule)
		if !collapsed[f.Path()] {
			lines = append(lines, renderFileBody(f, width, t, highlight)...)
		}
		lines = append(lines, "")
	}
	return lines, starts
}

func renderSummary(files []diff.File, width int, t common.Theme) []string {
	added, removed := 0, 0
	for _, f := range files {
		added += f.Added
		removed += f.Removed
	}
	noun := "files"
	if len(files) == 1 {
		noun = "file"
	}
	lines := []string{
		" " + t.StyleTitle.Render(fmt.Sprintf("%d %s changed", len(files), noun)) + "  " + renderStat(added, removed, t),
		"",
	}
	for _, f := range files {
		left := " " + statusLetter(f, t) + " " + renderPath(f, t.StyleSubtitle)
		lines = append(lines, alignRight(left, renderStat(f.Added, f.Removed, t), width))
	}
	return append(lines, "")
}

func renderFileHeader(f diff.File, collapsed bool, width int, t common.Theme) string {
	chevron := "▾"
	if collapsed {
		chevron = "▸"
	}
	bg := t.StyleFileHeader.GetBackground()
	on := func(s lipgloss.Style) lipgloss.Style { return s.Background(bg) }
	left := on(lipgloss.NewStyle().Foreground(t.ColorAccent)).Render(chevron) +
		on(lipgloss.NewStyle()).Render(" ") +
		statusLetterOn(f, t, bg) +
		on(lipgloss.NewStyle()).Render(" ") +
		renderPath(f, t.StyleFileHeader)
	right := renderStatOn(f.Added, f.Removed, t, bg)
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right) - 1
	if gap < 1 {
		return ansi.Truncate(left, width, "…")
	}
	return left + on(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap)) + right + on(lipgloss.NewStyle()).Render(" ")
}

func renderFileBody(f diff.File, width int, t common.Theme, highlight bool) []string {
	switch {
	case f.Binary:
		return []string{t.StyleFaint.Render("  Binary file changed")}
	case len(f.Hunks) == 0 && f.Status == diff.Renamed:
		return []string{t.StyleFaint.Render("  Renamed without changes")}
	case len(f.Hunks) == 0:
		return []string{t.StyleFaint.Render("  Empty file")}
	}

	digits := 1
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			digits = max(digits, len(strconv.Itoa(max(l.OldNo, l.NewNo))))
		}
	}
	hl := newHighlighter(f.Path(), t.SyntaxStyle, t.ColorText, highlight)

	var lines []string
	for _, h := range f.Hunks {
		header := t.StyleFaint.Render(fmt.Sprintf("  @@ -%d,%d +%d,%d @@", h.OldStart, h.OldLines, h.NewStart, h.NewLines))
		if h.Section != "" {
			header += " " + t.StyleHunkHeader.Render(h.Section)
		}
		lines = append(lines, ansi.Truncate(header, width, "…"))

		texts := make([]string, len(h.Lines))
		for i, l := range h.Lines {
			texts[i] = expandTabs(l.Text)
		}
		emph := make([]diff.Range, len(h.Lines))
		for _, pair := range diff.Pairs(h.Lines) {
			if oldR, newR, ok := diff.ChangedRanges(texts[pair[0]], texts[pair[1]]); ok {
				emph[pair[0]], emph[pair[1]] = oldR, newR
			}
		}
		for i, l := range h.Lines {
			lines = append(lines, renderLine(l, texts[i], emph[i], hl, digits, width, t)...)
		}
	}
	return lines
}

// renderLine draws one diff line, wrapped to width, as one or more rows.
func renderLine(l diff.Line, text string, emph diff.Range, hl *highlighter, digits, width int, t common.Theme) []string {
	var bg, emphBg, signFg color.Color
	sign := " "
	switch l.Kind {
	case diff.Add:
		bg, emphBg, signFg, sign = t.ColorAddBg, t.ColorAddEmph, t.ColorAddFg, "+"
	case diff.Delete:
		bg, emphBg, signFg, sign = t.ColorDelBg, t.ColorDelEmph, t.ColorDelFg, "-"
	}
	style := func(fg color.Color, emphasized bool) lipgloss.Style {
		s := lipgloss.NewStyle()
		if fg != nil {
			s = s.Foreground(fg)
		}
		switch {
		case emphasized && emphBg != nil:
			return s.Background(emphBg)
		case bg != nil:
			return s.Background(bg)
		}
		return s
	}

	number := func(n int) string {
		if n == 0 {
			return strings.Repeat(" ", digits)
		}
		return fmt.Sprintf("%*d", digits, n)
	}
	gutter := style(t.ColorFaint, false).Render(" " + number(l.OldNo) + " " + number(l.NewNo) + " ")
	blankGutter := style(nil, false).Render(strings.Repeat(" ", 2*digits+3))
	signCell := style(signFg, false).Render(sign + " ")
	if l.NoNewline {
		text += " ⏎̸"
	}

	codeWidth := width - (2*digits + 3) - 2
	spans := emphasize(hl.spans(text), emph)
	rows := wrapSpans(spans, codeWidth)

	out := make([]string, 0, len(rows))
	for i, row := range rows {
		var b strings.Builder
		if i == 0 {
			b.WriteString(gutter + signCell)
		} else {
			b.WriteString(blankGutter + style(nil, false).Render("  "))
		}
		for _, s := range row {
			b.WriteString(style(s.fg, s.emph).Render(s.text))
		}
		if gap := codeWidth - spansWidth(row); gap > 0 && bg != nil {
			b.WriteString(style(nil, false).Render(strings.Repeat(" ", gap)))
		}
		out = append(out, b.String())
	}
	return out
}

func renderPath(f diff.File, style lipgloss.Style) string {
	if f.Status == diff.Renamed && f.OldPath != f.NewPath {
		return style.Render(f.OldPath + " → " + f.NewPath)
	}
	return style.Render(f.Path())
}

func statusLetter(f diff.File, t common.Theme) string {
	return statusLetterOn(f, t, nil)
}

func statusLetterOn(f diff.File, t common.Theme, bg color.Color) string {
	letter, fg := "M", t.ColorMuted
	switch f.Status {
	case diff.Added:
		letter, fg = "A", t.ColorAddFg
	case diff.Deleted:
		letter, fg = "D", t.ColorDelFg
	case diff.Renamed:
		letter, fg = "R", t.ColorAccent
	}
	s := lipgloss.NewStyle().Foreground(fg).Bold(true)
	if bg != nil {
		s = s.Background(bg)
	}
	return s.Render(letter)
}

func renderStat(added, removed int, t common.Theme) string {
	return renderStatOn(added, removed, t, nil)
}

func renderStatOn(added, removed int, t common.Theme, bg color.Color) string {
	on := func(fg color.Color) lipgloss.Style {
		s := lipgloss.NewStyle()
		if fg != nil {
			s = s.Foreground(fg)
		}
		if bg != nil {
			s = s.Background(bg)
		}
		return s
	}
	var parts []string
	if added > 0 {
		parts = append(parts, on(t.ColorAddFg).Render(fmt.Sprintf("+%d", added)))
	}
	if removed > 0 {
		parts = append(parts, on(t.ColorDelFg).Render(fmt.Sprintf("-%d", removed)))
	}
	return strings.Join(parts, on(nil).Render(" "))
}

// alignRight places right at the end of a width-wide line.
func alignRight(left, right string, width int) string {
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right) - 1
	if gap < 1 {
		return ansi.Truncate(left, width, "…")
	}
	return left + strings.Repeat(" ", gap) + right
}

package diffview

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/diff"
)

// tabWidth is how many cells a tab expands to.
const tabWidth = 4

// span is a run of text drawn in one foreground colour. Emph marks the
// changed part of an edited line, which gets a stronger background.
type span struct {
	text string
	fg   color.Color
	emph bool
}

// highlighter colours code for one file. Without a lexer for the file type,
// or when highlighting is switched off for size, text gets the plain colour.
type highlighter struct {
	lexer  chroma.Lexer
	style  *chroma.Style
	plain  color.Color
	colors map[chroma.TokenType]color.Color
}

func newHighlighter(path, styleName string, plain color.Color, enabled bool) *highlighter {
	h := &highlighter{plain: plain, colors: map[chroma.TokenType]color.Color{}}
	if !enabled {
		return h
	}
	if lexer := lexers.Match(path); lexer != nil {
		h.lexer = chroma.Coalesce(lexer)
		h.style = styles.Get(styleName)
	}
	return h
}

// spans splits one line into coloured runs. Lines are tokenised on their own,
// so constructs spanning lines, such as block comments, may be coloured as
// code; a diff hunk has no reliable start of file to tokenise from anyway.
func (h *highlighter) spans(line string) []span {
	if h.lexer == nil {
		return []span{{text: line, fg: h.plain}}
	}
	iter, err := h.lexer.Tokenise(nil, line)
	if err != nil {
		return []span{{text: line, fg: h.plain}}
	}
	var out []span
	for _, tok := range iter.Tokens() {
		text := strings.TrimRight(tok.Value, "\n")
		if text == "" {
			continue
		}
		out = append(out, span{text: text, fg: h.color(tok.Type)})
	}
	return out
}

func (h *highlighter) color(t chroma.TokenType) color.Color {
	if c, ok := h.colors[t]; ok {
		return c
	}
	c := h.plain
	if entry := h.style.Get(t); entry.Colour.IsSet() {
		c = lipgloss.Color(entry.Colour.String())
	}
	h.colors[t] = c
	return c
}

// expandTabs replaces tabs with spaces up to the next tab stop, so widths
// measured from the text match what the terminal shows.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col += ansi.StringWidth(string(r))
	}
	return b.String()
}

// emphasize marks the runes in r as changed, splitting spans at its edges.
func emphasize(spans []span, r diff.Range) []span {
	if r.Empty() {
		return spans
	}
	var out []span
	pos := 0
	for _, s := range spans {
		runes := []rune(s.text)
		start, end := pos, pos+len(runes)
		pos = end
		cuts := []int{0}
		if r.Start > start && r.Start < end {
			cuts = append(cuts, r.Start-start)
		}
		if r.End > start && r.End < end {
			cuts = append(cuts, r.End-start)
		}
		cuts = append(cuts, len(runes))
		for i := 0; i+1 < len(cuts); i++ {
			piece := span{text: string(runes[cuts[i]:cuts[i+1]]), fg: s.fg}
			at := start + cuts[i]
			piece.emph = at >= r.Start && at < r.End
			out = append(out, piece)
		}
	}
	return out
}

// wrapSpans breaks spans into rows at most width cells wide.
func wrapSpans(spans []span, width int) [][]span {
	if width <= 0 {
		return [][]span{spans}
	}
	var text strings.Builder
	for _, s := range spans {
		text.WriteString(s.text)
	}
	breaks := wrapPoints([]rune(text.String()), width)

	rows := make([][]span, 0, len(breaks)+1)
	var row []span
	pos, next := 0, 0
	for _, s := range spans {
		runes := []rune(s.text)
		for len(runes) > 0 {
			take := len(runes)
			if next < len(breaks) && pos+take > breaks[next] {
				take = breaks[next] - pos
			}
			if take > 0 {
				row = append(row, span{text: string(runes[:take]), fg: s.fg, emph: s.emph})
				runes, pos = runes[take:], pos+take
			}
			if next < len(breaks) && pos == breaks[next] {
				rows = append(rows, row)
				row = nil
				next++
			}
		}
	}
	return append(rows, row)
}

// wrapPoints returns the rune indices at which text starts a new row. It
// breaks after the last space in a row when that keeps the row at least half
// full, so prose wraps between words, and otherwise breaks mid-word, which
// long identifiers and URLs need.
func wrapPoints(text []rune, width int) []int {
	var points []int
	rowStart, used, lastSpace := 0, 0, -1
	for i := 0; i < len(text); i++ {
		w := ansi.StringWidth(string(text[i]))
		if used+w > width && used > 0 {
			brk := i
			if lastSpace >= rowStart && lastSpace+1-rowStart >= width/2 {
				brk = lastSpace + 1
			}
			points = append(points, brk)
			rowStart, lastSpace = brk, -1
			used = 0
			for j := brk; j < i; j++ {
				used += ansi.StringWidth(string(text[j]))
			}
		}
		if text[i] == ' ' {
			lastSpace = i
		}
		used += w
	}
	return points
}

// spansWidth is the display width of a row of spans.
func spansWidth(spans []span) int {
	w := 0
	for _, s := range spans {
		w += ansi.StringWidth(s.text)
	}
	return w
}

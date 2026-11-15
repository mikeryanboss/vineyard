package recapview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

func multiline(prefix string, n int) string {
	lines := make([]string, n)
	for i := range n {
		lines[i] = fmt.Sprintf("%s line %d", prefix, i+1)
	}
	return strings.Join(lines, "\n")
}

func sampleRecap() recap.Recap {
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	return recap.Recap{
		Found: true,
		Title: "Recap Tab Feature",
		Entries: []recap.Entry{
			{Time: at, Kind: recap.KindPrompt, Text: multiline("prompt-one", 7)},
			{Time: at.Add(time.Minute), Kind: recap.KindResponse, Text: multiline("response-one", 6)},
			{Time: at.Add(2 * time.Minute), Kind: recap.KindPrompt, Text: "short follow-up"},
			{Time: at.Add(3 * time.Minute), Kind: recap.KindRecap, Text: multiline("away-recap", 5)},
		},
	}
}

func TestRecapView_DefaultPreviewAndHeaderChips(t *testing.T) {
	m := New(common.NewTheme(true)).SetSize(80, 40).SetRecap("s1", sampleRecap())
	view := ansi.Strip(m.View())

	for _, want := range []string{
		"Recap Tab Feature",
		"▾ 2 prompts",
		"▾ 1 response",
		"▾ 1 recap",
		"expand all",
		"collapse all",
		"▸ … 3 more lines",
		"▸ … 2 more lines",
		"▸ … 1 more line",
		"short follow-up",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
	if strings.Contains(view, "prompt-one line 5") || strings.Contains(view, "response-one line 5") {
		t.Errorf("lines past PreviewLines should be hidden in preview mode:\n%s", view)
	}
}

func TestRecapView_KeyboardNavigationCyclingAndKindFolding(t *testing.T) {
	m := New(common.NewTheme(true)).SetSize(80, 18).SetFocused(true).SetRecap("s1", sampleRecap())
	if got := m.CurrentEntry(); got != 3 {
		t.Fatalf("initial CurrentEntry = %d, want 3 (newest)", got)
	}

	// On a long entry, enter cycles: Preview -> Expanded -> Collapsed -> Preview.
	m, _ = m.Update(testutil.Key("enter"))
	if view := ansi.Strip(m.View()); !strings.Contains(view, "away-recap line 5") {
		t.Fatalf("first enter should expand bottom entry:\n%s", view)
	}
	m, _ = m.Update(testutil.Key("enter"))
	if view := ansi.Strip(m.View()); strings.Contains(view, "away-recap line 2") || !strings.Contains(view, "▸ ✻ Recap") {
		t.Fatalf("second enter should collapse bottom entry to 1-line header:\n%s", view)
	}
	m, _ = m.Update(testutil.Key("enter"))
	if view := ansi.Strip(m.View()); !strings.Contains(view, "away-recap line 4") || strings.Contains(view, "away-recap line 5") {
		t.Fatalf("third enter should return bottom entry to 4-line preview:\n%s", view)
	}

	// Keys 1, 2, 3 toggle prompts, responses, and recaps between collapsed and preview.
	m, _ = m.Update(testutil.Key("1"))
	rendered := ansi.Strip(m.View())
	if !strings.Contains(rendered, "▸ 2 prompts") || strings.Contains(rendered, "short follow-up\n") {
		// Note: 1-line collapsed header shows snippet on the You header line, not a body row.
		if m.spans[0].state != stateCollapsed || m.spans[2].state != stateCollapsed {
			t.Fatalf("key 1 should collapse all prompts: spans=%+v\n%s", m.spans, rendered)
		}
	}
	m, _ = m.Update(testutil.Key("1"))
	if m.spans[0].state != statePreview || m.spans[2].state != statePreview {
		t.Fatalf("pressing 1 again should uncollapse prompts to preview: spans=%+v", m.spans)
	}

	m, _ = m.Update(testutil.Key("2"))
	if m.spans[1].state != stateCollapsed || !strings.Contains(ansi.Strip(m.View()), "▸ 1 response") {
		t.Fatalf("key 2 should collapse responses")
	}
	m, _ = m.Update(testutil.Key("3"))
	if m.spans[3].state != stateCollapsed || !strings.Contains(ansi.Strip(m.View()), "▸ 1 recap") {
		t.Fatalf("key 3 should collapse recaps")
	}

	// Expand all with e, collapse all with c.
	m, _ = m.Update(testutil.Key("e"))
	fullBody := ansi.Strip(strings.Join(m.lines, "\n"))
	if !strings.Contains(fullBody, "prompt-one line 7") || !strings.Contains(fullBody, "response-one line 6") || !strings.Contains(fullBody, "away-recap line 5") {
		t.Errorf("e should expand every entry:\n%s", fullBody)
	}
	m, _ = m.Update(testutil.Key("c"))
	for i, sp := range m.spans {
		if sp.state != stateCollapsed || sp.end-sp.start != 1 {
			t.Errorf("after c, entry %d span = %+v, want 1-line stateCollapsed", i, sp)
		}
	}
}

func TestRecapView_MouseClicksOnChipsHeadersAndPreviewBodies(t *testing.T) {
	m := New(common.NewTheme(true)).SetSize(80, 40).SetRecap("s1", sampleRecap())
	hdrH := len(m.headerLines)
	if hdrH == 0 {
		t.Fatal("expected pinned header lines")
	}

	// Clicking a chip toggles its kind; clicking past the chips is not clickable.
	var respChip chipSpan
	for _, c := range m.chips {
		if c.action == chipToggleResponses {
			respChip = c
		}
	}
	if !m.Clickable(respChip.startX+1, respChip.line) {
		t.Fatal("response chip should be clickable")
	}
	if m.Clickable(79, respChip.line) {
		t.Fatal("empty space at right edge of chip bar should not be clickable")
	}

	m, _ = m.Update(tea.MouseClickMsg{X: respChip.startX + 1, Y: respChip.line})
	if m.spans[1].state != stateCollapsed || !strings.Contains(ansi.Strip(m.View()), "▸ 1 response") {
		t.Fatalf("clicking response chip should collapse all responses:\n%s", ansi.Strip(m.View()))
	}
	// Click again to reopen responses to preview.
	m, _ = m.Update(tea.MouseClickMsg{X: respChip.startX + 1, Y: respChip.line})
	if m.spans[1].state != statePreview {
		t.Fatalf("clicking collapsed response chip should restore preview, got %v", m.spans[1].state)
	}

	// Clicking "▸ … 2 more lines" on response-one expands it to full.
	moreViewY := hdrH + m.spans[1].end - 1
	if !m.Clickable(4, moreViewY) {
		t.Fatal("more-lines row should be clickable")
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: moreViewY})
	if m.spans[1].state != stateExpanded || !strings.Contains(ansi.Strip(m.View()), "response-one line 6") {
		t.Fatalf("clicking preview body should expand entry:\n%s", ansi.Strip(m.View()))
	}

	// Body of an expanded entry is NOT clickable (so double-click zooms), while header IS clickable.
	headerViewY := hdrH + m.spans[1].start
	if !m.Clickable(4, headerViewY) || m.Clickable(4, headerViewY+2) {
		t.Errorf("expanded entry: header clickable=%v, body clickable=%v; want true, false",
			m.Clickable(4, headerViewY), m.Clickable(4, headerViewY+2))
	}

	// Clicking expanded header folds back to preview, and clicking header again collapses to 1 line.
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: headerViewY})
	if m.spans[1].state != statePreview {
		t.Fatalf("clicking expanded header should fold to preview, got %v", m.spans[1].state)
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: hdrH + m.spans[1].start})
	if m.spans[1].state != stateCollapsed {
		t.Fatalf("clicking preview header should collapse to 1-line header, got %v", m.spans[1].state)
	}
}

func TestRecapView_ConfigModesHideCollapseAndExpand(t *testing.T) {
	cfg := config.RecapConfig{
		Prompts:   config.RecapCollapsed,
		Responses: config.RecapOff,
		Recaps:    config.RecapExpanded,
	}
	m := New(common.NewTheme(true)).SetConfig(cfg).SetSize(80, 40).SetRecap("s1", sampleRecap())
	view := ansi.Strip(m.View())

	if m.EntryCount() != 3 {
		t.Fatalf("EntryCount = %d, want 3 (responses hidden)", m.EntryCount())
	}
	if strings.Contains(view, "response") || strings.Contains(view, "Response") {
		t.Errorf("responses = off should hide response entries and chip:\n%s", view)
	}
	if !strings.Contains(view, "▸ 2 prompts") || m.spans[0].state != stateCollapsed {
		t.Errorf("prompts = collapsed should start collapsed:\n%s", view)
	}
	if !strings.Contains(view, "away-recap line 5") || m.spans[2].state != stateExpanded {
		t.Errorf("recaps = expanded should show all lines of recap:\n%s", view)
	}

	// Hiding all kinds shows a clear message rather than claiming nothing was said.
	allOff := config.RecapConfig{
		Prompts:   config.RecapOff,
		Responses: config.RecapOff,
		Recaps:    config.RecapOff,
	}
	m = m.SetConfig(allOff)
	if got := ansi.Strip(m.View()); !strings.Contains(got, "hidden by recap settings") || m.EntryCount() != 0 {
		t.Errorf("all kinds off should explain entries are hidden:\n%s", got)
	}
}

func TestRecapView_PreservesStateAcrossLiveUpdatesAndResetsOnNewSession(t *testing.T) {
	r := sampleRecap()
	m := New(common.NewTheme(true)).SetSize(80, 20).SetRecap("s1", r)
	m, _ = m.Update(testutil.Key("g"))
	m, _ = m.Update(testutil.Key("enter")) // expand entry 0
	m, _ = m.Update(testutil.Key("G"))

	r.Entries = append(r.Entries, recap.Entry{
		Time: r.Entries[len(r.Entries)-1].Time.Add(time.Minute),
		Kind: recap.KindResponse,
		Text: "brand new final reply",
	})
	m = m.SetRecap("s1", r)
	allRendered := ansi.Strip(strings.Join(m.lines, "\n"))
	if !strings.Contains(allRendered, "prompt-one line 7") {
		t.Errorf("expanded entry should remain expanded when new entries arrive:\n%s", allRendered)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "brand new final reply") {
		t.Errorf("view at bottom should follow new entry:\n%s", view)
	}

	m = m.SetRecap("s2", sampleRecap())
	s2Rendered := ansi.Strip(strings.Join(m.lines, "\n"))
	if strings.Contains(s2Rendered, "prompt-one line 7") {
		t.Errorf("switching session should reset per-entry overrides:\n%s", s2Rendered)
	}
}

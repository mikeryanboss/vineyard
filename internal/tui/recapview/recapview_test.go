package recapview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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

func TestRecapView_DefaultLimitsLongEntriesAndShowsCounts(t *testing.T) {
	m := New(common.NewTheme(true)).SetSize(72, 40).SetRecap("s1", sampleRecap())
	view := ansi.Strip(m.View())

	if !strings.Contains(view, "Recap Tab Feature") {
		t.Fatalf("missing title:\n%s", view)
	}
	if !strings.Contains(view, "2 prompts · 1 response · 1 recap") {
		t.Errorf("missing summary counts:\n%s", view)
	}
	if !strings.Contains(view, "▸ ❯ You") || !strings.Contains(view, "… 3 more lines") {
		t.Errorf("7-line prompt should show ▸ and 3 hidden lines:\n%s", view)
	}
	if strings.Contains(view, "prompt-one line 5") {
		t.Errorf("line 5 should be hidden by default:\n%s", view)
	}
	if !strings.Contains(view, "▸ ● Response") || !strings.Contains(view, "… 2 more lines") {
		t.Errorf("6-line response should show ▸ and 2 hidden lines:\n%s", view)
	}
	if !strings.Contains(view, "short follow-up") || strings.Contains(view, "▸ ❯ You\n  short follow-up") {
		t.Errorf("1-line prompt should be shown in full without a fold chevron:\n%s", view)
	}
	if !strings.Contains(view, "▸ ✻ Recap") || !strings.Contains(view, "… 1 more line") {
		t.Errorf("5-line recap should show singular 1 more line:\n%s", view)
	}
}

func TestRecapView_KeyboardNavigationAndToggleIncludesBottomEntry(t *testing.T) {
	m := New(common.NewTheme(true)).SetSize(72, 16).SetFocused(true).SetRecap("s1", sampleRecap())
	// Opens at the newest entry (index 3), even though multiple compact cards fit on screen.
	if got := m.CurrentEntry(); got != 3 {
		t.Fatalf("initial CurrentEntry = %d, want 3 (newest)", got)
	}

	// Pressing enter expands the bottom entry even when maxYOffset < its start line.
	m, _ = m.Update(testutil.Key("enter"))
	if view := ansi.Strip(m.View()); !strings.Contains(view, "away-recap line 5") || !strings.Contains(view, "▾ ✻ Recap") {
		t.Fatalf("enter on bottom entry should expand it:\n%s", view)
	}
	m, _ = m.Update(testutil.Key("enter"))
	if view := ansi.Strip(m.View()); strings.Contains(view, "away-recap line 5") {
		t.Fatalf("second enter should fold bottom entry back to preview:\n%s", view)
	}

	// Walk backward with [ to entry 0, then forward with ].
	for want := 2; want >= 0; want-- {
		m, _ = m.Update(testutil.Key("["))
		if got := m.CurrentEntry(); got != want {
			t.Fatalf("after [, CurrentEntry = %d, want %d", got, want)
		}
	}
	// One more [ at entry 0 reveals the title at line 0.
	m, _ = m.Update(testutil.Key("["))
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Recap Tab Feature") {
		t.Errorf("[ at first entry should scroll to show title:\n%s", view)
	}

	m, _ = m.Update(testutil.Key("]"))
	if got := m.CurrentEntry(); got != 1 {
		t.Fatalf("after ], CurrentEntry = %d, want 1", got)
	}

	// Expand all with e, collapse all with c.
	m, _ = m.Update(testutil.Key("e"))
	fullText := ansi.Strip(strings.Join(m.lines, "\n"))
	if !strings.Contains(fullText, "prompt-one line 7") || !strings.Contains(fullText, "response-one line 6") || !strings.Contains(fullText, "away-recap line 5") {
		t.Errorf("e should expand every entry:\n%s", fullText)
	}
	m, _ = m.Update(testutil.Key("c"))
	compactText := ansi.Strip(strings.Join(m.lines, "\n"))
	if strings.Contains(compactText, "prompt-one line 7") || strings.Contains(compactText, "response-one line 6") {
		t.Errorf("c should collapse every entry back to preview:\n%s", compactText)
	}
}

func TestRecapView_ClicksExpandTruncatedBodyAndFoldExpandedHeader(t *testing.T) {
	m := New(common.NewTheme(true)).SetSize(72, 40).SetRecap("s1", sampleRecap())
	// Entry 1 (response-one): click on its "… 2 more lines" row expands it.
	sp := m.spans[1]
	moreRow := sp.end - 1
	if !m.Clickable(moreRow) || !m.Clickable(sp.start) {
		t.Fatalf("truncated card header (%d) and more-lines row (%d) should both be clickable", sp.start, moreRow)
	}
	// Short card (entry 2) has nothing hidden, so Clickable is false.
	if m.Clickable(m.spans[2].start) {
		t.Errorf("short non-collapsible entry should not capture clicks as folds")
	}

	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: moreRow})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "response-one line 6") || !strings.Contains(view, "▾ ● Response") {
		t.Fatalf("click on more-lines row should expand response:\n%s", view)
	}
	// While expanded, header is clickable, body lines are not (so double-click can still zoom).
	expSpan := m.spans[1]
	if !m.Clickable(expSpan.start) || m.Clickable(expSpan.start+2) {
		t.Errorf("expanded card: header clickable=%v, body clickable=%v; want true, false", m.Clickable(expSpan.start), m.Clickable(expSpan.start+2))
	}

	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: expSpan.start})
	if view := ansi.Strip(m.View()); strings.Contains(view, "response-one line 6") {
		t.Errorf("click on expanded header should collapse back to preview:\n%s", view)
	}
}

func TestRecapView_PreservesExpandedAcrossLiveUpdatesAndResetsOnNewSession(t *testing.T) {
	r := sampleRecap()
	m := New(common.NewTheme(true)).SetSize(72, 20).SetRecap("s1", r)
	// Expand entry 0, then return to bottom so live updates follow.
	m, _ = m.Update(testutil.Key("g"))
	m, _ = m.Update(testutil.Key("enter"))
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

	// Switching session resets expanded state.
	m = m.SetRecap("s2", sampleRecap())
	s2Rendered := ansi.Strip(strings.Join(m.lines, "\n"))
	if strings.Contains(s2Rendered, "prompt-one line 7") {
		t.Errorf("switching session should reset expanded entries:\n%s", s2Rendered)
	}
}

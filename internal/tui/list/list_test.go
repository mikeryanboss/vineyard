package list_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/list"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

func sampleItems() []list.Item {
	var items []list.Item
	for i, s := range testutil.Sessions() {
		items = append(items, list.Item{Session: s, Stat: git.Stat{Added: 10 * i, Removed: i}})
	}
	return items
}

func newList(width, height int) list.Model {
	return list.New(common.NewTheme(true)).SetItems(sampleItems()).SetSize(width, height)
}

func TestListView_Default(t *testing.T) {
	testutil.RequireGolden(t, newList(36, 20).View())
}

func TestListView_Narrow(t *testing.T) {
	testutil.RequireGolden(t, newList(22, 20).View())
}

func TestListView_Empty(t *testing.T) {
	testutil.RequireGolden(t, list.New(common.NewTheme(true)).SetSize(36, 10).View())
}

func TestListView_ScrollsSelectionIntoView(t *testing.T) {
	m := newList(36, 8) // room for two entries
	for range 4 {
		m, _ = m.Update(testutil.Key("j"))
	}
	testutil.RequireGolden(t, m.View())
}

func TestList_NavigationClamps(t *testing.T) {
	m := newList(36, 20)
	m, _ = m.Update(testutil.Key("k"))
	if item, _ := m.Selected(); item.Session.ID != "login-1" {
		t.Errorf("up at the top selected %q", item.Session.ID)
	}
	for range 10 {
		m, _ = m.Update(testutil.Key("down"))
	}
	if item, _ := m.Selected(); item.Session.ID != "new-5" {
		t.Errorf("down past the end selected %q", item.Session.ID)
	}
}

func TestList_SetItemsKeepsSelectedSession(t *testing.T) {
	m := newList(36, 20).Select("docs-3")
	items := sampleItems()
	m = m.SetItems(items[2:]) // drop the first two sessions
	if item, _ := m.Selected(); item.Session.ID != "docs-3" {
		t.Errorf("selection moved to %q after items changed", item.Session.ID)
	}
}

func TestList_ClickSelectsEntry(t *testing.T) {
	m := newList(36, 20)
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 3*2}) // third entry's title line
	if item, _ := m.Selected(); item.Session.ID != "docs-3" {
		t.Errorf("click selected %q, want docs-3", item.Session.ID)
	}
}

// withSubagents gives the first session six subagents: four drawn, the
// second of them shown and running one of its own, and two more out of sight.
func withSubagents() []list.Item {
	items := sampleItems()
	items[0].Subagents = subagent.List{
		Rows: []subagent.Row{
			{Type: "general-purpose", Label: "Running the parser tests"},
			{Type: "Explore", Label: "Finding where redirects are built"},
			{Type: "general-purpose", Label: "Reading the router", Depth: 1},
			{Type: "general-purpose", Label: "Reviewing the diff", Hidden: 2},
		},
		Below: 2, Shown: 2, Focus: -1,
	}
	return items
}

func TestListView_Subagents(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20)
	m, _ = m.Update(testutil.Key("j"))
	testutil.RequireGolden(t, m.View())
}

func TestList_NavigationVisitsSubagentRows(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20)
	want := []struct {
		id  string
		sub int // -1 for the session row
	}{{"login-1", 0}, {"login-1", 1}, {"login-1", 2}, {"login-1", 3}, {"login-1", 4}, {"login-1", 5}, {"tests-2", -1}}
	for _, w := range want {
		m, _ = m.Update(testutil.Key("j"))
		item, _ := m.Selected()
		sub, ok := m.SelectedSubagent()
		if !ok {
			sub = -1
		}
		if item.Session.ID != w.id || sub != w.sub {
			t.Fatalf("selected %s row %d, want %s row %d", item.Session.ID, sub, w.id, w.sub)
		}
	}
	m, _ = m.Update(testutil.Key("k"))
	if sub, ok := m.SelectedSubagent(); !ok || sub != 5 {
		t.Errorf("up from the next session selected row %d (%v), want the last subagent", sub, ok)
	}
}

// Claude Code draws fewer rows for a moment while it redraws, as after a
// detach. The selection must come back to the subagent the user chose.
func TestList_SubagentSelectionSurvivesARedraw(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20)
	for range 3 {
		m, _ = m.Update(testutil.Key("j")) // the third subagent
	}
	items := withSubagents()
	items[0].Subagents = subagent.List{Rows: items[0].Subagents.Rows[:1], Shown: 0, Focus: -1}
	m = m.SetItems(items)
	if sub, ok := m.SelectedSubagent(); !ok || sub != 0 {
		t.Errorf("selected subagent %d (%v), want the one left", sub, ok)
	}
	m = m.SetItems(sampleItems())
	if _, ok := m.SelectedSubagent(); ok {
		t.Error("with no subagents, the session must be selected")
	}
	m = m.SetItems(withSubagents())
	if sub, ok := m.SelectedSubagent(); !ok || sub != 2 {
		t.Errorf("selected subagent %d (%v) after the redraw, want 2", sub, ok)
	}
}

func TestList_ClickSelectsSubagentRow(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20)
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 3}) // the second subagent line
	if sub, ok := m.SelectedSubagent(); !ok || sub != 1 {
		t.Errorf("click selected row %d (%v), want subagent 1", sub, ok)
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 9 + 1}) // the next session's branch line
	if item, _ := m.Selected(); item.Session.ID != "tests-2" {
		t.Errorf("click selected %s, want tests-2", item.Session.ID)
	}
	if _, ok := m.SelectedSubagent(); ok {
		t.Error("a session line must select the session")
	}
}

func TestList_ScrollsTallEntriesIntoView(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 10) // eight lines of entries
	for range 8 {
		m, _ = m.Update(testutil.Key("j"))
	}
	if item, _ := m.Selected(); item.Session.ID != "docs-3" {
		t.Fatalf("selected %s, want docs-3", item.Session.ID)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Docs pass") {
		t.Errorf("the selected session is scrolled out of view:\n%s", view)
	}
}

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

// withSubagents gives the first session six subagents: the second shown in
// Claude Code's view and running one of its own, one done, one failed.
func withSubagents() []list.Item {
	items := sampleItems()
	agent := func(id, description string, depth int, status subagent.Status) subagent.Agent {
		return subagent.Agent{ID: id, Type: "general-purpose", Description: description, Depth: depth, Status: status}
	}
	items[0].Subagents = []subagent.Agent{
		agent("a1", "parser tests", 0, subagent.Running),
		agent("a2", "find where redirects are built", 0, subagent.Running),
		agent("a3", "read the router", 1, subagent.Running),
		agent("a4", "review the diff", 0, subagent.Done),
		agent("a5", "update the docs", 0, subagent.Failed),
		agent("a6", "write the changelog", 0, subagent.Running),
	}
	items[0].Shown = "a2"
	return items
}

// selectedSub returns the ID of the selected subagent, or "" for none.
func selectedSub(m list.Model) string {
	a, _ := m.SelectedSubagent()
	return a.ID
}

// expanded returns a list of withSubagents with the first session's
// subagent rows shown.
func expanded(t *testing.T, height int) list.Model {
	t.Helper()
	m, ok := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, height).Expand()
	if !ok {
		t.Fatal("the first session's subagents did not expand")
	}
	return m
}

func TestListView_SubagentsCollapsed(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20)
	testutil.RequireGolden(t, m.View())
}

func TestListView_Subagents(t *testing.T) {
	m := expanded(t, 20)
	m, _ = m.Update(testutil.Key("j"))
	testutil.RequireGolden(t, m.View())
}

func TestList_CollapsedSubagentsAreSkipped(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20)
	m, _ = m.Update(testutil.Key("j"))
	if item, _ := m.Selected(); item.Session.ID != "tests-2" {
		t.Errorf("down from a collapsed session selected %s, want tests-2", item.Session.ID)
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 4}) // the next session's title line
	if item, _ := m.Selected(); item.Session.ID != "tests-2" {
		t.Errorf("click selected %s, want tests-2", item.Session.ID)
	}
	if _, ok := m.Expand(); ok {
		t.Error("a session without subagents must not expand")
	}
}

func TestList_ClickOnTheCountLineFoldsSubagents(t *testing.T) {
	m := list.New(common.NewTheme(true)).SetItems(withSubagents()).SetSize(36, 20).Select("tests-2")
	countLine := tea.MouseClickMsg{X: 4, Y: 2 + 2}
	m, _ = m.Update(countLine)
	view := testutil.StripANSI(m.View())
	if item, _ := m.Selected(); item.Session.ID != "login-1" || !strings.Contains(view, "▾ 6 subagents: 4 running, 1 don…") || !strings.Contains(view, "parser tests") {
		t.Errorf("a click on the count line should select the session and show its rows:\n%s", view)
	}
	if _, ok := m.SelectedSubagent(); ok {
		t.Error("the count line must select the session")
	}
	m, _ = m.Update(countLine)
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "▸ 6 subagents") || strings.Contains(view, "parser tests") {
		t.Errorf("a second click should hide the rows:\n%s", view)
	}
}

func TestList_CollapseSelectsTheSession(t *testing.T) {
	m := expanded(t, 20)
	if _, ok := m.Expand(); ok {
		t.Error("an expanded session must not expand again")
	}
	m, _ = m.Update(testutil.Key("j"))
	m, _ = m.Update(testutil.Key("j"))
	m = m.Collapse()
	if item, _ := m.Selected(); item.Session.ID != "login-1" {
		t.Errorf("collapse selected %s, want login-1", item.Session.ID)
	}
	if _, ok := m.SelectedSubagent(); ok {
		t.Error("collapse must select the session")
	}
	if view := testutil.StripANSI(m.View()); strings.Contains(view, "parser tests") || !strings.Contains(view, "▸ 6 subagents") {
		t.Errorf("collapsed session shows its rows:\n%s", view)
	}
	if _, ok := m.Expand(); !ok {
		t.Error("a collapsed session must expand again")
	}
}

func TestList_NavigationVisitsSubagentRows(t *testing.T) {
	m := expanded(t, 20)
	want := []struct{ id, sub string }{
		{"login-1", "a1"}, {"login-1", "a2"}, {"login-1", "a3"}, {"login-1", "a4"}, {"login-1", "a5"}, {"login-1", "a6"}, {"tests-2", ""},
	}
	for _, w := range want {
		m, _ = m.Update(testutil.Key("j"))
		if item, _ := m.Selected(); item.Session.ID != w.id || selectedSub(m) != w.sub {
			t.Fatalf("selected %s row %q, want %s row %q", item.Session.ID, selectedSub(m), w.id, w.sub)
		}
	}
	m, _ = m.Update(testutil.Key("k"))
	if sub := selectedSub(m); sub != "a6" {
		t.Errorf("up from the next session selected row %q, want the last subagent", sub)
	}
}

// The selection follows the subagent as others start before it and stop
// being shown; once it is gone, the session is selected.
func TestList_SubagentSelectionFollowsTheSubagent(t *testing.T) {
	m := expanded(t, 20)
	for range 4 {
		m, _ = m.Update(testutil.Key("j")) // a4, review the diff
	}
	items := withSubagents()
	items[0].Subagents = items[0].Subagents[3:]
	m = m.SetItems(items)
	if sub := selectedSub(m); sub != "a4" {
		t.Errorf("selected subagent %q, want a4 where it now is", sub)
	}
	items[0].Subagents = items[0].Subagents[1:]
	m = m.SetItems(items)
	if item, _ := m.Selected(); selectedSub(m) != "" || item.Session.ID != "login-1" {
		t.Errorf("selected %s row %q, want the session once its subagent is gone", item.Session.ID, selectedSub(m))
	}
}

func TestList_ClickSelectsSubagentRow(t *testing.T) {
	m := expanded(t, 20)
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 4}) // the second subagent line
	if sub := selectedSub(m); sub != "a2" {
		t.Errorf("click selected row %q, want the second subagent", sub)
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 10 + 1}) // the next session's branch line
	if item, _ := m.Selected(); item.Session.ID != "tests-2" {
		t.Errorf("click selected %s, want tests-2", item.Session.ID)
	}
	if _, ok := m.SelectedSubagent(); ok {
		t.Error("a session line must select the session")
	}
}

func TestList_ScrollsTallEntriesIntoView(t *testing.T) {
	m := expanded(t, 10) // eight lines of entries
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

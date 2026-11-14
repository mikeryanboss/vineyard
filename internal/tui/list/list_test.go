package list_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
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

func TestList_NavigationWrapsAround(t *testing.T) {
	m := newList(36, 8) // room for two entries
	m, _ = m.Update(testutil.Key("k"))
	if item, _ := m.Selected(); item.Session.ID != "new-5" {
		t.Errorf("up at the top selected %q, want the last session", item.Session.ID)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Starting up") {
		t.Errorf("the last session is scrolled out of view:\n%s", view)
	}
	m, _ = m.Update(testutil.Key("down"))
	if item, _ := m.Selected(); item.Session.ID != "login-1" {
		t.Errorf("down at the bottom selected %q, want the first session", item.Session.ID)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Fix login redirect") {
		t.Errorf("the first session is scrolled out of view:\n%s", view)
	}
}

// Up from the first session wraps to the last row, the last session's last
// subagent when it is expanded, and down from there wraps to the first
// session itself.
func TestList_NavigationWrapsThroughSubagentRows(t *testing.T) {
	items := withSubagents()
	items[0].Subagents, items[4].Subagents = nil, items[0].Subagents
	m, ok := list.New(common.NewTheme(true)).SetItems(items).SetSize(36, 20).Select("new-5").Expand()
	if !ok {
		t.Fatal("the last session's subagents did not expand")
	}
	m = m.Select("login-1")
	m, _ = m.Update(testutil.Key("k"))
	if item, _ := m.Selected(); item.Session.ID != "new-5" || selectedSub(m) != "a6" {
		t.Errorf("up at the top selected %s row %q, want new-5 row a6", item.Session.ID, selectedSub(m))
	}
	m, _ = m.Update(testutil.Key("j"))
	if item, _ := m.Selected(); item.Session.ID != "login-1" || selectedSub(m) != "" {
		t.Errorf("down at the bottom selected %s row %q, want the session login-1", item.Session.ID, selectedSub(m))
	}
}

func TestList_WheelStopsAtTheEnds(t *testing.T) {
	m := newList(36, 20)
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if item, _ := m.Selected(); item.Session.ID != "login-1" {
		t.Errorf("wheel up at the top selected %q", item.Session.ID)
	}
	for range 10 {
		m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	if item, _ := m.Selected(); item.Session.ID != "new-5" {
		t.Errorf("wheel down past the end selected %q", item.Session.ID)
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
		agent("a5", "update the docs", 0, subagent.Stopped),
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

func TestList_DragDelta(t *testing.T) {
	m := newList(36, 20)
	if got := m.DragDelta(2); got != 0 {
		t.Fatalf("at start: DragDelta(2) = %d, want 0", got)
	}
	if got := m.DragDelta(5); got != 1 {
		t.Fatalf("onto second entry: DragDelta(5) = %d, want 1", got)
	}

	// Across a 10-line expanded entry and a 3-line entry, swapping at the
	// threshold must settle at the same pointer y without oscillating.
	items := withSubagents()
	m = list.New(common.NewTheme(true)).SetItems(items).SetSize(36, 25)
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 4}) // expand login-1 (height 10)
	if got := m.DragDelta(11); got != 0 {
		t.Fatalf("still inside expanded entry: DragDelta(11) = %d, want 0", got)
	}
	if got := m.DragDelta(12); got != 1 {
		t.Fatalf("onto next entry below expanded: DragDelta(12) = %d, want 1", got)
	}
	items = slices.Clone(items)
	items[0], items[1] = items[1], items[0]
	m = m.SetItems(items) // login-1 is now index 1, tests-2 (height 3) is index 0
	if got := m.DragDelta(12); got != 0 {
		t.Fatalf("after swapping past tests-2: DragDelta(12) = %d, want 0", got)
	}
	// Now swap tests-2 above expanded login-1 (index 1, height 10) by dragging up.
	items = slices.Clone(items)
	items[0], items[1] = items[1], items[0] // login-1 back at 0 (height 10), tests-2 at 1
	m = m.SetItems(items).Select("tests-2")
	if got := m.DragDelta(8); got != 0 {
		t.Fatalf("below midpoint of expanded entry: DragDelta(8) = %d, want 0", got)
	}
	if got := m.DragDelta(6); got != -1 {
		t.Fatalf("above midpoint of expanded entry: DragDelta(6) = %d, want -1", got)
	}
	items = slices.Clone(items)
	items[0], items[1] = items[1], items[0]
	m = m.SetItems(items)
	if got := m.DragDelta(6); got != 0 {
		t.Fatalf("after swapping above expanded entry: DragDelta(6) = %d, want 0", got)
	}

	empty := list.New(common.NewTheme(true)).SetSize(16, 10).View()
	if got := len(strings.Split(empty, "\n")); got != 10 {
		t.Errorf("empty list at width 16 rendered %d lines, want 10:\n%s", got, empty)
	}
}

func TestList_IssuePlacementAndClipping(t *testing.T) {
	items := sampleItems()[:1]
	items[0].Issues = []int{12}
	titleRow := func(m list.Model) string {
		return strings.Split(testutil.StripANSI(m.View()), "\n")[2]
	}

	m := list.New(common.NewTheme(true)).SetItems(items).SetSize(36, 10)
	if row := titleRow(m); !strings.Contains(row, "Fix login redirect #12") {
		t.Errorf("default placement = %q, want title before #12", row)
	}

	// Widths include 4 border/icon/margin cells; "Fix login redirect" is 18 wide.
	for _, tc := range []struct {
		width int
		want  string
	}{
		{4 + 22, "Fix login redirect #12"},
		{4 + 21, "Fix login redirect #…"},
		{4 + 19, "Fix login redirect…"},
		{4 + 18, "Fix login redirect"},
		{4 + 17, "Fix login redire…"},
	} {
		row := titleRow(m.SetSize(tc.width, 10))
		if !strings.Contains(row, tc.want) {
			t.Errorf("width %d: row = %q, want %q", tc.width, row, tc.want)
		}
	}

	opts := list.DefaultOptions()
	opts.Issues = config.IssuesBefore
	if row := titleRow(m.SetOptions(opts)); !strings.Contains(row, "#12 Fix login redirect") {
		t.Errorf("before placement = %q, want #12 before title", row)
	}

	opts.Issues = config.IssuesOff
	if row := titleRow(m.SetOptions(opts)); strings.Contains(row, "#12") || !strings.Contains(row, "Fix login redirect") {
		t.Errorf("off placement = %q, want title without #12", row)
	}
}

func TestList_DisplayOptions(t *testing.T) {
	m := expanded(t, 20)
	m, _ = m.Update(testutil.Key("j")) // select subagent a1
	if selectedSub(m) != "a1" {
		t.Fatalf("selected subagent = %q, want a1", selectedSub(m))
	}

	// Turning subagents off hides rows and clears subagent selection.
	opts := list.DefaultOptions()
	opts.Subagents = false
	m = m.SetOptions(opts)
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "subagent") || strings.Contains(view, "parser tests") || selectedSub(m) != "" || m.Expandable() {
		t.Errorf("subagents off still shows or selects subagents:\n%s", view)
	}

	// Turning branch off compacts rows, moves diff stats onto the title line,
	// and keeps subagent click coordinates aligned.
	opts = list.DefaultOptions()
	opts.Branch = false
	m = list.New(common.NewTheme(true)).SetOptions(opts).SetItems(withSubagents()).SetSize(36, 20)
	view = testutil.StripANSI(m.View())
	if strings.Contains(view, common.IconBranch) {
		t.Errorf("branch off still shows branch line:\n%s", view)
	}
	if !strings.Contains(view, "Docs pass") || !strings.Contains(view, "+20 -2") {
		t.Errorf("branch off should keep diff stats on the title line:\n%s", view)
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 1}) // count line without branch line
	m, _ = m.Update(tea.MouseClickMsg{X: 4, Y: 2 + 2}) // first subagent row without branch line
	if sub := selectedSub(m); sub != "a1" {
		t.Errorf("click with branch off selected subagent %q, want a1", sub)
	}

	// Turning diff stats and auto tag off hides both.
	opts.DiffStats = false
	opts.AutoTag = false
	view = testutil.StripANSI(m.SetOptions(opts).View())
	if strings.Contains(view, "+10 -1") || strings.Contains(view, " auto ") {
		t.Errorf("diff stats and auto tag off still show them:\n%s", view)
	}
}

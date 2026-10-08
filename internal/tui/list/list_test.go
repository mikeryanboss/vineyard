package list_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/git"
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

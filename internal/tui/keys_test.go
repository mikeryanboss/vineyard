package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

// newModelWithKeys is newTestModel with keys in its configuration, wide
// enough for the whole status bar. The default keys come back after the
// test.
func newModelWithKeys(t *testing.T, sessions []session.Session, keys config.Keys) (Model, *fakeBackend) {
	t.Helper()
	m, backend := newTestModel(t, sessions)
	t.Cleanup(func() { common.Keys = common.NewKeyMap(config.DefaultKeys()) })
	opts := m.opts
	opts.Config.Keys = keys
	return update(NewModel(backend, sessions, opts), tea.WindowSizeMsg{Width: 220, Height: 26}), backend
}

// quits reports whether msg makes the model quit.
func quits(m Model, msg tea.KeyPressMsg) bool {
	_, cmd := m.Update(msg)
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// Configured keys replace an action's default keys, an empty list unbinds
// the action, and the status bar shows the keys bound.
func TestApp_ConfiguredKeysReplaceTheDefaults(t *testing.T) {
	ctrlG := tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	m, _ := newModelWithKeys(t, testutil.Sessions(), config.Keys{
		"global": {"quit": {"ctrl+g"}, "back": {"ctrl+b"}},
		"list":   {"kill": {"x"}, "auto_yes": {}},
	})

	bar := screen(m)[strings.LastIndex(screen(m), "\n"):]
	if !strings.Contains(bar, "x kill") || strings.Contains(bar, "D kill") || strings.Contains(bar, "auto-yes") {
		t.Errorf("the status bar should show x for kill and leave out auto-yes:%s", bar)
	}
	if m = keys(m, "a"); m.sessions[0].AutoYes {
		t.Error("a unbound must not toggle auto-yes")
	}
	if m = keys(m, "D"); m.dialog != nil {
		t.Fatal("D unbound must not ask to kill")
	}
	if m = keys(m, "x"); m.dialog == nil || !strings.Contains(screen(m), "Kill \"Fix login redirect\"?") {
		t.Fatalf("x should ask to kill:\n%s", screen(m))
	}
	if m = keys(m, "ctrl+q"); m.dialog == nil {
		t.Fatal("ctrl+q unbound must not leave the dialog")
	}
	if m = send(m, tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}); m.dialog != nil {
		t.Fatal("ctrl+b should return to the list in place of ctrl+q")
	}
	if quits(m, testutil.Key("ctrl+c")) || !quits(m, ctrlG) {
		t.Error("ctrl+g should quit in place of ctrl+c")
	}
}

// Dialogs, their answers as drawn, and the config screen follow the keys.
func TestApp_DialogsAndConfigFollowTheKeys(t *testing.T) {
	configured := config.Keys{
		"confirm":  {"yes": {"o"}, "no": {"esc"}},
		"settings": {"save": {"ctrl+w"}},
	}
	m, backend := newModelWithKeys(t, testutil.Sessions(), configured)
	m = keys(m, "C")
	if !strings.Contains(screen(m), "ctrl+w save") {
		t.Errorf("the config screen should offer ctrl+w to save:\n%s", screen(m))
	}
	m = keys(m, "ctrl+s")
	if len(backend.configs) != 0 {
		t.Fatal("ctrl+s unbound must not save")
	}
	m = send(m, tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if len(backend.configs) != 1 || !reflect.DeepEqual(backend.configs[0].Keys, configured) {
		t.Fatalf("saving should keep the keys as configured; saved %+v", backend.configs)
	}

	m = keys(m, "D")
	if !strings.Contains(screen(m), "o confirm") {
		t.Fatalf("the kill question should offer o to confirm:\n%s", screen(m))
	}
	if m = keys(m, "y"); len(backend.killed) != 0 || m.dialog == nil {
		t.Fatal("y unbound must not confirm")
	}
	if keys(m, "o"); len(backend.killed) != 1 {
		t.Errorf("o should confirm the kill; killed %v", backend.killed)
	}
}

// The mouse acts without the keys it stands for: a double click on a
// session opens it with open and attach unbound.
func TestApp_DoubleClickWorksWithTheKeysUnbound(t *testing.T) {
	c := clock(t)
	m, backend := newModelWithKeys(t, testutil.Sessions(), config.Keys{"list": {"open": {}, "attach": {}}})
	m = keys(m, "enter", "o")
	if backend.attaches != 0 {
		t.Fatal("enter and o unbound must not attach")
	}
	*c = c.Add(time.Second)
	doubleClickOn(t, m, c, "Fix login redirect", 0)
	if backend.attaches != 1 {
		t.Errorf("a double click on a session attached %d times, want 1", backend.attaches)
	}
}

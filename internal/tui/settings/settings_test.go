package settings_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/settings"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

func newSettings(cfg config.Config, loadErr error) settings.Model {
	return settings.New(cfg, "~/.vineyard/config.toml", loadErr, common.NewTheme(true)).SetSize(80, 14)
}

func twoProfiles() config.Config {
	return config.Config{
		DefaultProgram: "claude",
		BranchPrefix:   "mboss/",
		Profiles: []config.Profile{
			{Name: "claude", Program: "claude"},
			{Name: "codex", Program: "codex --full-auto"},
		},
	}
}

// press applies keys in order and returns the command the last one produced.
func press(m settings.Model, keys ...string) (settings.Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = m.Update(testutil.Key(k))
	}
	return m, cmd
}

// msgOf runs cmd. Only call it for commands that return at once: the text
// input's cursor blink sleeps.
func msgOf(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func typeText(m settings.Model, text string) settings.Model {
	for _, r := range text {
		m, _ = m.Update(testutil.Key(string(r)))
	}
	return m
}

func TestSettingsView_General(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "l")
	testutil.RequireGolden(t, m.View())
}

func TestSettingsView_Profiles(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "j", "l", "j")
	testutil.RequireGolden(t, m.View())
}

func TestSettings_WithoutProfilesEditsTheDefaultAsOne(t *testing.T) {
	m := newSettings(config.Config{DefaultProgram: "aider"}, nil)
	profiles := m.Config().Profiles
	if len(profiles) != 1 || profiles[0] != (config.Profile{Name: "aider", Program: "aider"}) {
		t.Errorf("profiles = %+v, want the default agent as the only profile", profiles)
	}
}

func TestSettings_EditBranchPrefixAndSave(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "l", "j", "enter")
	for range len("mboss/") {
		m, _ = m.Update(testutil.Key("backspace"))
	}
	m = typeText(m, "agents/")
	m, cmd := press(m, "enter", "ctrl+s")
	msg := msgOf(cmd)
	saved, ok := msg.(common.SaveConfigMsg)
	if !ok {
		t.Fatalf("ctrl+s produced %T, want SaveConfigMsg", msg)
	}
	if saved.Config.BranchPrefix != "agents/" {
		t.Errorf("saved branch prefix = %q", saved.Config.BranchPrefix)
	}
}

func TestSettings_AddProfileAsksForNameThenCommand(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "j", "l", "j", "j", "enter") // + Add profile
	m = typeText(m, "yolo")
	m, _ = press(m, "enter")
	if !strings.Contains(testutil.StripANSI(m.View()), "Enter the command that starts yolo.") {
		t.Fatalf("after the name the screen should ask for the command:\n%s", testutil.StripANSI(m.View()))
	}
	m = typeText(m, " --dangerously-skip-permissions") // the input starts with the name
	m, _ = press(m, "enter")

	profiles := m.Config().Profiles
	want := config.Profile{Name: "yolo", Program: "yolo --dangerously-skip-permissions"}
	if len(profiles) != 3 || profiles[2] != want {
		t.Errorf("profiles = %+v, want %+v added", profiles, want)
	}
}

func TestSettings_AddProfileRejectsDuplicateName(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "j", "l", "j", "j", "enter")
	m = typeText(m, "codex")
	m, _ = press(m, "enter")
	if len(m.Config().Profiles) != 2 || !strings.Contains(testutil.StripANSI(m.View()), "already exists") {
		t.Errorf("a duplicate name should be refused:\n%s", testutil.StripANSI(m.View()))
	}
}

func TestSettings_RemovingTheDefaultProfileMovesTheDefault(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "j", "l", "x") // claude, the default
	cfg := m.Config()
	if len(cfg.Profiles) != 1 || cfg.DefaultProgram != "codex" {
		t.Errorf("after removing the default: %+v", cfg)
	}
	m, _ = press(m, "x")
	if len(m.Config().Profiles) != 1 {
		t.Error("the last profile must not be removable")
	}
}

func TestSettings_DefaultAgentCyclesOrPicks(t *testing.T) {
	m, _ := press(newSettings(twoProfiles(), nil), "l", "enter")
	if got := m.Config().DefaultProgram; got != "codex" {
		t.Errorf("enter on two options should cycle, default = %q", got)
	}

	cfg := twoProfiles()
	cfg.Profiles = append(cfg.Profiles, config.Profile{Name: "aider", Program: "aider"}, config.Profile{Name: "gemini", Program: "gemini"})
	m, _ = press(newSettings(cfg, nil), "l", "enter")
	if !m.PickerActive() {
		t.Fatal("enter on more than three options should open the picker")
	}
	m, _ = press(m, "j", "j", "enter")
	if m.PickerActive() || m.Config().DefaultProgram != "aider" {
		t.Errorf("picker left default %q, active %v", m.Config().DefaultProgram, m.PickerActive())
	}
}

func TestSettings_EscStepsBackThenCloses(t *testing.T) {
	m, cmd := press(newSettings(twoProfiles(), nil), "l", "esc")
	if msg := msgOf(cmd); msg != nil {
		t.Fatalf("esc in the field pane should only return to the categories, got %T", msg)
	}
	_, cmd = press(m, "esc")
	if msg := msgOf(cmd); msg != (common.CloseConfigMsg{}) {
		t.Errorf("esc in the category pane produced %T, want CloseConfigMsg", msg)
	}
}

func TestSettings_RefusesToSaveOverAFileThatFailedToLoad(t *testing.T) {
	m, cmd := press(newSettings(twoProfiles(), errors.New("bad toml")), "ctrl+s")
	if msg := msgOf(cmd); msg != nil {
		t.Fatalf("save produced %T despite the load error", msg)
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "failed to load") {
		t.Errorf("the screen should say why it did not save:\n%s", testutil.StripANSI(m.View()))
	}
}

package dialog_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/dialog"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

var profiles = []config.Profile{{Name: "claude", Program: "claude"}, {Name: "codex", Program: "codex"}}

func typeText(d dialog.Dialog, text string) dialog.Dialog {
	for _, r := range text {
		d, _ = d.Update(testutil.Key(string(r)))
	}
	return d
}

// press sends a key and returns the message its command produces, if any.
func press(d dialog.Dialog, k string) (dialog.Dialog, tea.Msg) {
	d, cmd := d.Update(testutil.Key(k))
	if cmd == nil {
		return d, nil
	}
	return d, cmd()
}

func TestNewSession_SubmitsTitleAndProfile(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, false, 60)
	d = typeText(d, "Fix login")
	d, _ = press(d, "tab") // to the agent picker
	d, _ = press(d, "right")
	_, msg := press(d, "enter")

	got, ok := msg.(common.NewSessionMsg)
	if !ok {
		t.Fatalf("enter produced %T, want NewSessionMsg", msg)
	}
	if got.Title != "Fix login" || got.Profile.Name != "codex" || got.Prompt != "" {
		t.Errorf("msg = %+v", got)
	}
}

func TestNewSession_PromptFieldTakesMultilineInput(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles[:1], true, 60)
	d = typeText(d, "Docs")
	d, msg := press(d, "enter")
	if _, created := msg.(common.NewSessionMsg); created {
		t.Fatal("enter on the title should move to the prompt, not create the session")
	}
	d = typeText(d, "line one")
	d, _ = d.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Mod: tea.ModAlt}))
	d = typeText(d, "line two")
	_, msg = press(d, "enter")

	got, ok := msg.(common.NewSessionMsg)
	if !ok {
		t.Fatalf("enter in the prompt produced %T, want NewSessionMsg", msg)
	}
	if got.Prompt != "line one\nline two" {
		t.Errorf("prompt = %q", got.Prompt)
	}
}

func TestNewSession_RequiresTitle(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, false, 60)
	d, msg := press(d, "enter")
	if _, ok := msg.(common.NewSessionMsg); ok {
		t.Fatal("an empty title must not create a session")
	}
	testutil.RequireGolden(t, d.View())
}

func TestNewSession_EscCancels(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, true, 60)
	if _, msg := press(d, "esc"); msg != (common.DialogCancelledMsg{}) {
		t.Errorf("esc produced %T", msg)
	}
}

func TestNewSessionView_WithPrompt(t *testing.T) {
	d, _ := dialog.NewSessionDialog(common.NewTheme(true), profiles, true, 60)
	testutil.RequireGolden(t, d.View())
}

func TestConfirm(t *testing.T) {
	d := dialog.NewConfirm(common.NewTheme(true), "Kill session?", common.ActionKill, "s1")
	testutil.RequireGolden(t, d.View())

	_, msg := press(d, "y")
	if msg != (common.ConfirmedMsg{Action: common.ActionKill, SessionID: "s1"}) {
		t.Errorf("y produced %#v", msg)
	}
	if _, msg := press(d, "n"); msg != (common.DialogCancelledMsg{}) {
		t.Errorf("n produced %#v", msg)
	}
}

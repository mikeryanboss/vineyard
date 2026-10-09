package dialog_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/prompt"
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

var templates = []config.Template{
	{Name: "bug", Text: "Fix #{{.ID}}."},
	{Name: "default", Text: "Do #{{.ID}}."},
	{Name: "research", Text: "Study #{{.ID}}."},
}

// issueDialog opens the dialog for issue #7 with data and returns it with its
// prompt as a session created at once would get it.
func issueDialog(data prompt.Data) *dialog.NewSession {
	data.ID = 7
	d, _ := dialog.NewSessionDialog(common.NewTheme(true), profiles[:1], true, 60)
	return d.ForIssue("Embed grapes", templates, data, 40)
}

func submittedPrompt(t *testing.T, d dialog.Dialog) string {
	t.Helper()
	_, msg := press(d, "enter")
	got, ok := msg.(common.NewSessionMsg)
	if !ok {
		t.Fatalf("enter produced %T, want NewSessionMsg", msg)
	}
	return got.Prompt
}

// The template named after a label is preselected, and left and right on the
// Template field cycle through the templates and none, rendering each.
func TestNewSession_ForIssueChoosesTheTemplate(t *testing.T) {
	var d dialog.Dialog = issueDialog(prompt.Data{Labels: []string{"tui", "bug"}})
	d, _ = press(d, "tab") // to the template
	if got := submittedPrompt(t, d); got != "Fix #7." {
		t.Errorf("an issue labelled bug got %q, want the bug template", got)
	}
	for _, want := range []string{"", "Study #7.", "Do #7.", "Fix #7."} {
		d, _ = press(d, "left")
		if got := submittedPrompt(t, d); got != want {
			t.Errorf("after left, prompt = %q, want %q", got, want)
		}
	}
	d, _ = press(issueDialog(prompt.Data{}), "tab")
	if got := submittedPrompt(t, d); got != "Do #7." {
		t.Errorf("an issue without labels got %q, want the default template", got)
	}
}

func TestNewSession_BuildSubIssuesCheckbox(t *testing.T) {
	text := "{{if .BuildSubIssues}}Build them.{{else}}Leave them.{{end}}"
	open := func(data prompt.Data) dialog.Dialog {
		data.ID = 7
		d, _ := dialog.NewSessionDialog(common.NewTheme(true), profiles[:1], true, 60)
		return d.ForIssue("Embed grapes", []config.Template{{Name: "default", Text: text}}, data, 40)
	}
	if view := open(prompt.Data{}).View(); strings.Contains(view, "Build sub-issues") {
		t.Errorf("an issue without sub-issues should have no checkbox:\n%s", view)
	}

	d := open(prompt.Data{SubIssues: []prompt.SubIssue{{Issue: prompt.Issue{ID: 8}}}})
	d, _ = press(d, "tab") // to the template
	d, _ = press(d, "tab") // to the checkbox
	if got := submittedPrompt(t, d); got != "Leave them." {
		t.Errorf("unticked prompt = %q", got)
	}
	d, _ = press(d, "space")
	if got := submittedPrompt(t, d); got != "Build them." {
		t.Errorf("ticked prompt = %q", got)
	}
	testutil.RequireGolden(t, d.View())
}

func TestNewSession_ShowsTemplateErrors(t *testing.T) {
	d, _ := dialog.NewSessionDialog(common.NewTheme(true), profiles[:1], true, 60)
	d = d.ForIssue("Embed grapes", []config.Template{{Name: "default", Text: "{{.Nope}}"}}, prompt.Data{Issue: prompt.Issue{ID: 7}}, 40)
	if view := testutil.StripANSI(d.View()); !strings.Contains(view, "Template default:") {
		t.Errorf("the dialog should show the template's error:\n%s", view)
	}
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

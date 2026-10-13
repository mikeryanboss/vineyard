package dialog_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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

// N opens on the profile picker: enter moves on to the title, and enter
// there starts the chosen profile.
func TestNewSession_ChoosingSubmitsTitleAndProfile(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, true, 60)
	d, _ = press(d, "right")
	d, _ = d.Update(testutil.Key("enter")) // its command only blinks the cursor
	if view := testutil.StripANSI(d.View()); strings.Contains(view, "A title is required.") {
		t.Fatalf("enter on the profile should move to the title, not create:\n%s", view)
	}
	d = typeText(d, "Fix login")
	_, msg := press(d, "enter")

	got, ok := msg.(common.NewSessionMsg)
	if !ok {
		t.Fatalf("enter produced %T, want NewSessionMsg", msg)
	}
	if got.Title != "Fix login" || got.Profile.Name != "codex" || got.Prompt != "" {
		t.Errorf("msg = %+v", got)
	}
}

// n asks only for a title and starts the first profile, the default.
func TestNewSession_WithoutChoosingRunsTheFirstProfile(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, false, 60)
	if view := testutil.StripANSI(d.View()); strings.Contains(view, "‹") || strings.Contains(view, "Prompt") {
		t.Errorf("the dialog should offer neither a picker nor a prompt:\n%s", view)
	}
	d = typeText(d, "Fix login")
	d, _ = press(d, "tab") // nowhere else to go
	_, msg := press(d, "enter")
	if got, ok := msg.(common.NewSessionMsg); !ok || got.Title != "Fix login" || got.Profile.Name != "claude" {
		t.Errorf("enter produced %#v, want claude titled Fix login", msg)
	}
}

func TestNewSession_PromptFieldTakesMultilineInput(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles[:1], true, 60)
	d, _ = d.(*dialog.NewSession).ForIssue("Docs", nil, prompt.Data{Issue: prompt.Issue{ID: 7}}, 40)
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

func TestNewSessionView_ChoosingAProfile(t *testing.T) {
	d, _ := dialog.NewSessionDialog(common.NewTheme(true), []config.Profile{
		{Name: "terminal", Program: "$SHELL", MainCheckout: true},
		{Name: "claude", Program: "claude"},
	}, true, 60)
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
	d, _ = d.ForIssue("Embed grapes", templates, data, 40)
	return d
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
		d, _ = d.ForIssue("Embed grapes", []config.Template{{Name: "default", Text: text}}, data, 40)
		return d
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
	d, _ = d.ForIssue("Embed grapes", []config.Template{{Name: "default", Text: "{{.Nope}}"}}, prompt.Data{Issue: prompt.Issue{ID: 7}}, 40)
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

func TestConfirm_LinesAreCutToFit(t *testing.T) {
	d := dialog.NewConfirm(common.NewTheme(true), "Kill session anyway?", common.ActionKillDiscardingCommits, "s1",
		"1f08838 Short subject", "9ab12cd "+strings.Repeat("A long subject that goes on ", 4))
	testutil.RequireGolden(t, d.View())

	if _, msg := click(t, d, "confirm"); msg != (common.ConfirmedMsg{Action: common.ActionKillDiscardingCommits, SessionID: "s1"}) {
		t.Errorf("a click on confirm produced %#v", msg)
	}
}

// click clicks the first cell of text in d's view and returns the message
// its command produces, if any.
func click(t *testing.T, d dialog.Dialog, text string) (dialog.Dialog, tea.Msg) {
	t.Helper()
	view := testutil.StripANSI(d.View())
	for y, line := range strings.Split(view, "\n") {
		if i := strings.Index(line, text); i >= 0 {
			d, cmd := d.Update(tea.MouseClickMsg{X: ansi.StringWidth(line[:i]), Y: y})
			if cmd == nil {
				return d, nil
			}
			return d, cmd()
		}
	}
	t.Fatalf("%q is not in the dialog:\n%s", text, view)
	return d, nil
}

func TestNewSession_ClicksFocusFieldsAndStepTheProfile(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, true, 60)
	d, _ = click(t, d, "›")
	if !strings.Contains(testutil.StripANSI(d.View()), "‹ codex ›") {
		t.Errorf("a click on › should show the next profile:\n%s", testutil.StripANSI(d.View()))
	}
	d, _ = click(t, d, "Title")
	d = typeText(d, "Fix login")
	_, msg := press(d, "enter")
	if got, ok := msg.(common.NewSessionMsg); !ok || got.Title != "Fix login" || got.Profile.Name != "codex" {
		t.Errorf("after clicking the title and typing, enter produced %#v", msg)
	}
}

func TestNewSession_OffersTheProfileKeysOnItsFieldOnly(t *testing.T) {
	var d dialog.Dialog
	d, _ = dialog.NewSessionDialog(common.NewTheme(true), profiles, true, 60)
	offers := func() bool {
		for _, h := range d.Hints() {
			if h == [2]string{"←/→", "profile"} {
				return true
			}
		}
		return false
	}
	if !offers() {
		t.Error("on the profile field, the hints should offer ←/→")
	}
	d, _ = press(d, "tab")
	if offers() {
		t.Error("on the title, ←/→ move the cursor; the hints must not offer them for the profile")
	}
}

func TestConfirm_ClicksAnswer(t *testing.T) {
	d := dialog.NewConfirm(common.NewTheme(true), "Kill session?", common.ActionKill, "s1")
	if _, msg := click(t, d, "Kill session?"); msg != nil {
		t.Errorf("a click on the question produced %#v", msg)
	}
	if _, msg := click(t, d, "confirm"); msg != (common.ConfirmedMsg{Action: common.ActionKill, SessionID: "s1"}) {
		t.Errorf("a click on confirm produced %#v", msg)
	}
	if _, msg := click(t, d, "n cancel"); msg != (common.DialogCancelledMsg{}) {
		t.Errorf("a click on cancel produced %#v", msg)
	}
}

func TestPick_ClickChoosesAndWheelMoves(t *testing.T) {
	choices := []dialog.Choice{{Label: "one", Msg: "1"}, {Label: "two", Msg: "2"}, {Label: "three", Msg: "3"}}
	var d dialog.Dialog = dialog.NewPick(common.NewTheme(true), "Pick one", choices, 40)
	if _, msg := click(t, d, "two"); msg != "2" {
		t.Errorf("a click on two chose %#v", msg)
	}
	d, _ = d.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	d, _ = d.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if _, msg := press(d, "enter"); msg != "3" {
		t.Errorf("two wheel steps down chose %#v, want three", msg)
	}
}

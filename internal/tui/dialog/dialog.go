// Package dialog provides the modal dialogs drawn over the main screen:
// creating a session, confirming destructive actions, and picking one of
// several choices.
package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/prompt"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// Dialog is a modal that receives all input while open.
type Dialog interface {
	Update(tea.Msg) (Dialog, tea.Cmd)
	View() string
	// Hints are the key hints the status bar shows while the dialog is open.
	Hints() [][2]string
}

func cancel() tea.Msg { return common.DialogCancelledMsg{} }

// field identifies the focused part of the new-session dialog.
type field int

const (
	fieldTitle field = iota
	fieldTemplate
	fieldSubIssues
	fieldPrompt
	fieldProfile
)

var (
	keyCancel  = key.NewBinding(key.WithKeys("esc"))
	keySubmit  = key.NewBinding(key.WithKeys("enter"))
	keyNext    = key.NewBinding(key.WithKeys("tab"))
	keyPrev    = key.NewBinding(key.WithKeys("shift+tab"))
	keyLeft    = key.NewBinding(key.WithKeys("left", "h"))
	keyRight   = key.NewBinding(key.WithKeys("right", "l"))
	keyNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	keyToggle  = key.NewBinding(key.WithKeys("space"))
)

// NewSession asks for a title, optionally an initial prompt, and an agent.
// For a grapes issue, it also offers prompt templates.
type NewSession struct {
	theme      common.Theme
	width      int
	title      textinput.Model
	prompt     textarea.Model
	withPrompt bool
	issue      int // the grapes issue the session is for, or 0
	profiles   []config.Profile
	profile    int
	focus      field
	err        string

	// For an issue: the templates offered, the one the prompt was rendered
	// from (-1 for none), and what it was rendered with.
	templates []config.Template
	template  int
	data      prompt.Data
}

// NewSessionDialog returns the dialog. withPrompt adds the prompt field.
func NewSessionDialog(theme common.Theme, profiles []config.Profile, withPrompt bool, width int) (*NewSession, tea.Cmd) {
	inner := max(20, width-6) // border and padding
	title := textinput.New()
	title.Placeholder = "What should the agent work on?"
	title.Prompt = ""
	title.CharLimit = 100
	title.SetWidth(inner)
	title.SetStyles(textinput.DefaultStyles(theme.IsDark))

	prompt := textarea.New()
	prompt.Placeholder = "Instructions typed into the agent once it is ready"
	prompt.ShowLineNumbers = false
	prompt.Prompt = ""
	prompt.KeyMap.InsertNewline = keyNewline
	prompt.SetWidth(inner)
	prompt.SetHeight(5)
	prompt.SetStyles(textarea.DefaultStyles(theme.IsDark))

	d := &NewSession{
		theme:      theme,
		width:      width,
		title:      title,
		prompt:     prompt,
		withPrompt: withPrompt,
		profiles:   profiles,
	}
	return d, d.title.Focus()
}

// ForIssue fills the dialog in for a session working on the grapes issue
// data describes: its title, and a prompt rendered from the template among
// templates that prompt.Choose picks. Choosing another template, or toggling
// whether to build the issue's sub-issues, renders the prompt again. The
// title and prompt stay editable. The prompt field grows to show up to
// maxPromptHeight lines while the dialog fits in screenHeight.
func (d *NewSession) ForIssue(title string, templates []config.Template, data prompt.Data, screenHeight int) *NewSession {
	d.issue = data.ID
	d.withPrompt = true
	d.templates = templates
	d.template = prompt.Choose(templates, data.Labels)
	d.data = data
	d.title.SetValue(title)
	d.render()
	others := lipgloss.Height(d.View()) - d.prompt.Height()
	d.prompt.SetHeight(max(d.prompt.Height(), min(maxPromptHeight, screenHeight-others)))
	return d
}

// maxPromptHeight is the most lines the prompt of an issue's dialog shows.
const maxPromptHeight = 20

// render replaces the prompt with the chosen template rendered, or with
// nothing when no template is chosen or rendering fails.
func (d *NewSession) render() {
	d.err = ""
	if d.template < 0 {
		d.prompt.SetValue("")
		return
	}
	t := d.templates[d.template]
	text, err := prompt.Render(t.Text, d.data)
	if err != nil {
		d.err = fmt.Sprintf("Template %s: %v", t.Name, err)
	}
	d.prompt.SetValue(text)
	d.prompt.MoveToBegin() // show the prompt's start, not its end
}

// cycleTemplate chooses the next template, or the previous one for a
// negative step. None comes after the last template.
func (d *NewSession) cycleTemplate(step int) {
	n := len(d.templates) + 1 // the templates, then none at -1
	d.template = (d.template+1+step+n)%n - 1
	d.render()
}

// fields lists the focus stops in order.
func (d *NewSession) fields() []field {
	fields := []field{fieldTitle}
	if len(d.templates) > 0 {
		fields = append(fields, fieldTemplate)
	}
	if len(d.data.SubIssues) > 0 {
		fields = append(fields, fieldSubIssues)
	}
	if d.withPrompt {
		fields = append(fields, fieldPrompt)
	}
	if len(d.profiles) > 1 {
		fields = append(fields, fieldProfile)
	}
	return fields
}

func (d *NewSession) move(step int) tea.Cmd {
	fields := d.fields()
	i := 0
	for j, f := range fields {
		if f == d.focus {
			i = j
		}
	}
	d.focus = fields[(i+step+len(fields))%len(fields)]
	d.title.Blur()
	d.prompt.Blur()
	switch d.focus {
	case fieldTitle:
		return d.title.Focus()
	case fieldPrompt:
		return d.prompt.Focus()
	}
	return nil
}

func (d *NewSession) submit() tea.Cmd {
	title := strings.TrimSpace(d.title.Value())
	if title == "" {
		d.err = "A title is required."
		d.focus = fieldTitle
		d.prompt.Blur()
		return d.title.Focus()
	}
	msg := common.NewSessionMsg{Title: title, Issue: d.issue}
	if d.withPrompt {
		msg.Prompt = strings.TrimSpace(d.prompt.Value())
	}
	if len(d.profiles) > 0 {
		msg.Profile = d.profiles[d.profile]
	}
	return func() tea.Msg { return msg }
}

// Update handles input.
func (d *NewSession) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, keyCancel):
			return d, cancel
		case key.Matches(k, keyNext):
			return d, d.move(1)
		case key.Matches(k, keyPrev):
			return d, d.move(-1)
		case key.Matches(k, keySubmit):
			// Enter on the title moves on to the next field; anywhere else it creates.
			if d.focus == fieldTitle && d.withPrompt && strings.TrimSpace(d.title.Value()) != "" {
				return d, d.move(1)
			}
			return d, d.submit()
		case d.focus == fieldTemplate && key.Matches(k, keyLeft):
			d.cycleTemplate(-1)
			return d, nil
		case d.focus == fieldTemplate && key.Matches(k, keyRight):
			d.cycleTemplate(1)
			return d, nil
		case d.focus == fieldSubIssues && key.Matches(k, keyToggle):
			d.data.BuildSubIssues = !d.data.BuildSubIssues
			d.render()
			return d, nil
		case d.focus == fieldProfile && key.Matches(k, keyLeft):
			d.profile = (d.profile - 1 + len(d.profiles)) % len(d.profiles)
			return d, nil
		case d.focus == fieldProfile && key.Matches(k, keyRight):
			d.profile = (d.profile + 1) % len(d.profiles)
			return d, nil
		}
		d.err = ""
	}
	var cmd tea.Cmd
	switch d.focus {
	case fieldTitle:
		d.title, cmd = d.title.Update(msg)
	case fieldPrompt:
		d.prompt, cmd = d.prompt.Update(msg)
	}
	return d, cmd
}

// View renders the dialog box.
func (d *NewSession) View() string {
	t := d.theme
	label := func(text string, f field) string {
		if d.focus == f {
			return t.StyleTitle.Foreground(t.ColorAccent).Render(text)
		}
		return t.StyleSubtitle.Render(text)
	}
	heading := "New session"
	switch {
	case d.issue > 0:
		heading = fmt.Sprintf("New session for #%d", d.issue)
	case d.withPrompt:
		heading = "New session with prompt"
	}
	parts := []string{
		t.StyleTitle.Render(heading),
		"",
		label("Title", fieldTitle),
		d.title.View(),
	}
	if d.issue > 0 {
		if len(d.templates) > 0 {
			name := "none"
			if d.template >= 0 {
				name = d.templates[d.template].Name
			}
			picker := t.StyleFaint.Render("‹ ") + t.StyleTitle.Render(name) + t.StyleFaint.Render(" ›")
			parts = append(parts, "", label("Template", fieldTemplate), picker)
		} else {
			parts = append(parts, "", t.StyleFaint.Render("Template: none in .vineyard/templates or config.toml"))
		}
	}
	if len(d.data.SubIssues) > 0 {
		box := "[ ]"
		if d.data.BuildSubIssues {
			box = "[x]"
		}
		parts = append(parts, "", label(box+" Build sub-issues", fieldSubIssues))
	}
	if d.withPrompt {
		parts = append(parts, "", label("Prompt", fieldPrompt), d.prompt.View())
	}
	if len(d.profiles) > 1 {
		name := d.profiles[d.profile].Name
		picker := t.StyleFaint.Render("‹ ") + t.StyleTitle.Render(name) + t.StyleFaint.Render(" ›")
		parts = append(parts, "", label("Agent", fieldProfile), picker)
	} else if len(d.profiles) == 1 {
		parts = append(parts, "", t.StyleFaint.Render("Agent: "+d.profiles[0].Program))
	}
	if d.err != "" {
		parts = append(parts, "", t.StyleError.Render(d.err))
	}
	return t.StyleDialog.Width(d.width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// Hints returns the status bar key hints.
func (d *NewSession) Hints() [][2]string {
	hints := [][2]string{{"enter", "create"}}
	if d.withPrompt {
		hints = append(hints, [2]string{"alt+enter", "newline"})
	}
	if len(d.fields()) > 1 {
		hints = append(hints, [2]string{"tab", "next field"})
	}
	switch {
	case d.focus == fieldTemplate:
		hints = append(hints, [2]string{"←/→", "template"})
	case d.focus == fieldSubIssues:
		hints = append(hints, [2]string{"space", "toggle"})
	case len(d.profiles) > 1:
		hints = append(hints, [2]string{"←/→", "agent"})
	}
	return append(hints, [2]string{"esc", "cancel"})
}

// Confirm asks a yes/no question before an action.
type Confirm struct {
	theme     common.Theme
	message   string
	action    common.Action
	sessionID string
}

// NewConfirm returns a confirmation for action on a session.
func NewConfirm(theme common.Theme, message string, action common.Action, sessionID string) *Confirm {
	return &Confirm{theme: theme, message: message, action: action, sessionID: sessionID}
}

// Update handles input.
func (d *Confirm) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}
	switch k.String() {
	case "y", "Y", "enter":
		confirmed := common.ConfirmedMsg{Action: d.action, SessionID: d.sessionID}
		return d, func() tea.Msg { return confirmed }
	case "n", "N", "esc", "q":
		return d, cancel
	}
	return d, nil
}

// View renders the dialog box.
func (d *Confirm) View() string {
	t := d.theme
	body := lipgloss.JoinVertical(lipgloss.Left,
		t.StyleTitle.Render(d.message),
		"",
		t.FormatKeyHint("y", "confirm")+t.StyleFaint.Render("  ·  ")+t.FormatKeyHint("n", "cancel"),
	)
	return t.StyleDialog.Width(min(64, lipgloss.Width(body)+6)).Render(body)
}

// Hints returns the status bar key hints.
func (d *Confirm) Hints() [][2]string {
	return [][2]string{{"y", "confirm"}, {"n", "cancel"}}
}

// Choice is one option of a Pick dialog: its label and the message sent when
// it is chosen.
type Choice struct {
	Label string
	Msg   tea.Msg
}

// Pick asks the user to choose one of several options.
type Pick struct {
	theme   common.Theme
	title   string
	choices []Choice
	cursor  int
	width   int
}

// NewPick returns a picker for choices, which must not be empty.
func NewPick(theme common.Theme, title string, choices []Choice, width int) *Pick {
	return &Pick{theme: theme, title: title, choices: choices, width: width}
}

// Update handles input.
func (d *Pick) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}
	switch k.String() {
	case "j", "down":
		d.cursor = min(d.cursor+1, len(d.choices)-1)
	case "k", "up":
		d.cursor = max(d.cursor-1, 0)
	case "enter":
		chosen := d.choices[d.cursor].Msg
		return d, func() tea.Msg { return chosen }
	case "esc", "q":
		return d, cancel
	}
	return d, nil
}

// View renders the dialog box.
func (d *Pick) View() string {
	t := d.theme
	inner := max(10, d.width-6) // border and padding
	parts := []string{t.StyleTitle.Render(d.title), ""}
	for i, c := range d.choices {
		label := ansi.Truncate(c.Label, inner-2, "…")
		if i == d.cursor {
			parts = append(parts, t.StyleTitle.Foreground(t.ColorAccent).Render("› "+label))
		} else {
			parts = append(parts, t.StyleSubtitle.Render("  "+label))
		}
	}
	return t.StyleDialog.Width(d.width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// Hints returns the status bar key hints.
func (d *Pick) Hints() [][2]string {
	return [][2]string{{"j/k", "move"}, {"enter", "choose"}, {"esc", "cancel"}}
}

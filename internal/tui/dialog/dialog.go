// Package dialog provides the modal dialogs drawn over the main screen:
// creating a session and confirming destructive actions.
package dialog

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
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
)

// NewSession asks for a title, optionally an initial prompt, and an agent.
type NewSession struct {
	theme      common.Theme
	width      int
	title      textinput.Model
	prompt     textarea.Model
	withPrompt bool
	profiles   []config.Profile
	profile    int
	focus      field
	err        string
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

// fields lists the focus stops in order.
func (d *NewSession) fields() []field {
	fields := []field{fieldTitle}
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
	msg := common.NewSessionMsg{Title: title}
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
			// Enter on the title moves on to the prompt; anywhere else it creates.
			if d.focus == fieldTitle && d.withPrompt && strings.TrimSpace(d.title.Value()) != "" {
				return d, d.move(1)
			}
			return d, d.submit()
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
	if d.withPrompt {
		heading = "New session with prompt"
	}
	parts := []string{
		t.StyleTitle.Render(heading),
		"",
		label("Title", fieldTitle),
		d.title.View(),
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
	if len(d.profiles) > 1 {
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

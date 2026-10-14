// Package dialog provides the modal dialogs drawn over the main screen:
// creating a session, renaming one, confirming destructive actions, and
// picking one of several choices.
package dialog

import (
	"fmt"
	"math/rand/v2"
	"slices"
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
	fieldNone // text that is no field
)

// The dialog box's border and padding come before its content.
const (
	contentTop  = 2
	contentLeft = 3
)

// NewSession asks for a title and, when choosing, the profile to run. For a
// grapes issue, it also offers prompt templates and the prompt.
type NewSession struct {
	theme      common.Theme
	width      int
	title      textinput.Model
	prompt     textarea.Model
	withPrompt bool
	issue      int // the grapes issue the session is for, or 0
	profiles   []config.Profile
	choose     bool // whether the profile picker is offered
	profile    int
	focus      field
	err        string

	// For an issue: the templates offered, the one the prompt was rendered
	// from (-1 for none), and what it was rendered with.
	templates []config.Template
	template  int
	data      prompt.Data
}

// NewSessionDialog returns the dialog, which runs the first of profiles.
// choose offers a picker for the others, first and focused.
func NewSessionDialog(theme common.Theme, profiles []config.Profile, choose bool, width int) (*NewSession, tea.Cmd) {
	inner := max(20, width-6) // border and padding
	title := textinput.New()
	title.Placeholder = "What should the agent work on?"
	if choose {
		title.Placeholder = "What is this session for?"
	}
	title.Prompt = ""
	title.CharLimit = 100
	title.SetWidth(inner)
	title.SetStyles(textinput.DefaultStyles(theme.IsDark))

	prompt := textarea.New()
	prompt.Placeholder = "Instructions typed into the agent once it is ready"
	prompt.ShowLineNumbers = false
	prompt.Prompt = ""
	prompt.KeyMap.InsertNewline = common.Keys.Form.Newline
	prompt.SetWidth(inner)
	prompt.SetHeight(5)
	prompt.SetStyles(textarea.DefaultStyles(theme.IsDark))

	d := &NewSession{
		theme:    theme,
		width:    width,
		title:    title,
		prompt:   prompt,
		profiles: profiles,
		choose:   choose,
	}
	if d.picks() {
		return d, d.focusOn(fieldProfile)
	}
	return d, d.focusOn(fieldTitle)
}

// picks reports whether the dialog shows the profile picker.
func (d *NewSession) picks() bool { return d.choose && len(d.profiles) > 1 }

// ForIssue fills the dialog in for a session working on the grapes issue
// data describes: its title, and a prompt rendered from the template among
// templates that prompt.Choose picks. Choosing another template, or toggling
// whether to build the issue's sub-issues, renders the prompt again. The
// title and prompt stay editable, and the title has the focus. The prompt
// field grows to show up to maxPromptHeight lines while the dialog fits in
// screenHeight.
func (d *NewSession) ForIssue(title string, templates []config.Template, data prompt.Data, screenHeight int) (*NewSession, tea.Cmd) {
	d.issue = data.ID
	d.withPrompt = true
	d.templates = templates
	d.template = prompt.Choose(templates, data.Labels)
	d.data = data
	d.title.SetValue(title)
	d.render()
	others := lipgloss.Height(d.View()) - d.prompt.Height()
	d.prompt.SetHeight(max(d.prompt.Height(), min(maxPromptHeight, screenHeight-others)))
	return d, d.focusOn(fieldTitle)
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
	var fields []field
	if d.picks() {
		fields = append(fields, fieldProfile)
	}
	fields = append(fields, fieldTitle)
	if len(d.templates) > 0 {
		fields = append(fields, fieldTemplate)
	}
	if len(d.data.SubIssues) > 0 {
		fields = append(fields, fieldSubIssues)
	}
	if d.withPrompt {
		fields = append(fields, fieldPrompt)
	}
	return fields
}

func (d *NewSession) move(step int) tea.Cmd {
	fields := d.fields()
	i := max(0, slices.Index(fields, d.focus))
	return d.focusOn(fields[(i+step+len(fields))%len(fields)])
}

// focusOn gives field f the focus.
func (d *NewSession) focusOn(f field) tea.Cmd {
	d.focus = f
	d.title.Blur()
	d.prompt.Blur()
	switch f {
	case fieldTitle:
		return d.title.Focus()
	case fieldPrompt:
		return d.prompt.Focus()
	}
	return nil
}

// step shows the next template or agent, or the previous one for a negative
// step, when field f offers them.
func (d *NewSession) step(f field, step int) {
	switch f {
	case fieldTemplate:
		d.cycleTemplate(step)
	case fieldProfile:
		d.profile = (d.profile + step + len(d.profiles)) % len(d.profiles)
	}
}

// click focuses the field drawn at content column x and line y. On a picker,
// ‹ and › also step through its choices; on the sub-issues box, a click
// toggles it.
func (d *NewSession) click(x, y int) tea.Cmd {
	// The box wraps lines wider than its content, so a block may take more
	// lines than it has.
	wrap := lipgloss.NewStyle().Width(d.width - 2*contentLeft)
	for _, b := range d.blocks() {
		if height := lipgloss.Height(wrap.Render(b.text)); y >= height {
			y -= height
			continue
		}
		if b.field == fieldNone {
			return nil
		}
		cmd := d.focusOn(b.field)
		switch {
		case b.picker && x < 2:
			d.step(b.field, -1)
		case b.picker && x >= ansi.StringWidth(b.text)-2:
			d.step(b.field, 1)
		case b.field == fieldSubIssues:
			d.data.BuildSubIssues = !d.data.BuildSubIssues
			d.render()
		}
		return cmd
	}
	return nil
}

func (d *NewSession) submit() tea.Cmd {
	title := strings.TrimSpace(d.title.Value())
	if title == "" {
		title = randomTitle()
	}
	msg := common.NewSessionMsg{Title: title, Issue: d.issue}
	if d.withPrompt {
		msg.Prompt = strings.TrimSpace(d.prompt.Value())
	}
	msg.Profile = d.profiles[d.profile]
	return func() tea.Msg { return msg }
}

// randomTitle names a session the user left untitled: an adjective and a
// grape variety, such as "amber riesling".
func randomTitle() string {
	return adjectives[rand.N(len(adjectives))] + " " + grapes[rand.N(len(grapes))]
}

var (
	adjectives = []string{
		"amber", "bold", "brisk", "bright", "crisp", "dry", "dusky", "early",
		"fresh", "golden", "jammy", "late", "lively", "mellow", "oaky", "plump",
		"ripe", "robust", "rosy", "ruby", "silky", "smoky", "spicy", "sunny",
		"supple", "sweet", "tart", "tawny", "velvet", "young", "zesty",
	}
	grapes = []string{
		"barbera", "cabernet", "carmenere", "chardonnay", "chenin", "gamay",
		"grenache", "malbec", "merlot", "mourvedre", "muscat", "nebbiolo",
		"pinot", "primitivo", "riesling", "sangiovese", "semillon", "syrah",
		"tempranillo", "verdejo", "viognier", "zinfandel",
	}
)

// Update handles input. Clicks are relative to the dialog box.
func (d *NewSession) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if c, ok := msg.(tea.MouseClickMsg); ok {
		return d, d.click(c.X-contentLeft, c.Y-contentTop)
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		keys := common.Keys.Form
		switch {
		case key.Matches(k, keys.Cancel):
			return d, cancel
		case key.Matches(k, keys.NextField):
			return d, d.move(1)
		case key.Matches(k, keys.PrevField):
			return d, d.move(-1)
		case key.Matches(k, keys.Submit):
			// Enter on the profile moves on to the title, and on the title to
			// the next field; anywhere else it creates.
			if d.focus == fieldProfile || d.focus == fieldTitle && d.withPrompt && strings.TrimSpace(d.title.Value()) != "" {
				return d, d.move(1)
			}
			return d, d.submit()
		case (d.focus == fieldTemplate || d.focus == fieldProfile) && key.Matches(k, keys.PrevOption):
			d.step(d.focus, -1)
			return d, nil
		case (d.focus == fieldTemplate || d.focus == fieldProfile) && key.Matches(k, keys.NextOption):
			d.step(d.focus, 1)
			return d, nil
		case d.focus == fieldSubIssues && key.Matches(k, keys.Toggle):
			d.data.BuildSubIssues = !d.data.BuildSubIssues
			d.render()
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

// block is a part of the dialog's content, and the field a click on it
// focuses.
type block struct {
	field field
	text  string
	// picker marks a line of choices between ‹ and ›.
	picker bool
}

// blocks returns the dialog's content, top to bottom.
func (d *NewSession) blocks() []block {
	t := d.theme
	label := func(text string, f field) block {
		if d.focus == f {
			return block{field: f, text: t.StyleTitle.Foreground(t.ColorAccent).Render(text)}
		}
		return block{field: f, text: t.StyleSubtitle.Render(text)}
	}
	picker := func(name string, f field) block {
		text := t.StyleFaint.Render("‹ ") + t.StyleTitle.Render(name) + t.StyleFaint.Render(" ›")
		return block{field: f, text: text, picker: true}
	}
	text := func(s string) block { return block{field: fieldNone, text: s} }
	heading := "New session"
	if d.issue > 0 {
		heading = fmt.Sprintf("New session for #%d", d.issue)
	}
	runs := text(t.StyleFaint.Render(profileSummary(d.profiles[d.profile])))
	blocks := []block{text(t.StyleTitle.Render(heading)), text("")}
	if d.picks() {
		blocks = append(blocks, label("Profile", fieldProfile), picker(d.profiles[d.profile].Name, fieldProfile), runs, text(""))
	}
	blocks = append(blocks, label("Title", fieldTitle), block{field: fieldTitle, text: d.title.View()})
	if d.issue > 0 {
		if len(d.templates) > 0 {
			name := "none"
			if d.template >= 0 {
				name = d.templates[d.template].Name
			}
			blocks = append(blocks, text(""), label("Template", fieldTemplate), picker(name, fieldTemplate))
		} else {
			blocks = append(blocks, text(""), text(t.StyleFaint.Render("Template: none in .vineyard/templates or config.toml")))
		}
	}
	if len(d.data.SubIssues) > 0 {
		box := "[ ]"
		if d.data.BuildSubIssues {
			box = "[x]"
		}
		blocks = append(blocks, text(""), label(box+" Build sub-issues", fieldSubIssues))
	}
	if d.withPrompt {
		blocks = append(blocks, text(""), label("Prompt", fieldPrompt), block{field: fieldPrompt, text: d.prompt.View()})
	}
	if !d.picks() {
		blocks = append(blocks, text(""), runs)
	}
	if d.err != "" {
		blocks = append(blocks, text(""), text(t.StyleError.Render(d.err)))
	}
	return blocks
}

// profileSummary says what a session of profile p runs, where, and in which
// multiplexer.
func profileSummary(p config.Profile) string {
	summary := "Runs " + p.Program + " in a new worktree"
	if p.MainCheckout {
		summary = "Runs " + p.Program + " in the main checkout"
	}
	switch p.Multiplexer {
	case config.MultiplexerTmux:
		summary += ", in tmux"
	case config.MultiplexerVineyard:
		summary += ", on Vineyard's server"
	}
	return summary + "."
}

// View renders the dialog box.
func (d *NewSession) View() string {
	var parts []string
	for _, b := range d.blocks() {
		parts = append(parts, b.text)
	}
	return d.theme.StyleDialog.Width(d.width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// Hints returns the status bar key hints.
func (d *NewSession) Hints() [][2]string {
	keys, hint := common.Keys.Form, common.Hint
	hints := [][2]string{{hint(keys.Submit), "create"}}
	if d.focus == fieldProfile {
		hints = [][2]string{{hint(keys.Submit), "title"}}
	}
	if d.withPrompt {
		hints = append(hints, [2]string{hint(keys.Newline), "newline"})
	}
	if len(d.fields()) > 1 {
		hints = append(hints, [2]string{hint(keys.NextField), "next field"})
	}
	switch {
	case d.focus == fieldTemplate:
		hints = append(hints, [2]string{hint(keys.PrevOption, keys.NextOption), "template"})
	case d.focus == fieldSubIssues:
		hints = append(hints, [2]string{hint(keys.Toggle), "toggle"})
	case d.focus == fieldProfile:
		hints = append(hints, [2]string{hint(keys.PrevOption, keys.NextOption), "profile"})
	}
	return append(hints, [2]string{hint(keys.Cancel), "cancel"})
}

// Rename asks for a session's new title.
type Rename struct {
	theme     common.Theme
	width     int
	heading   string
	title     textinput.Model
	sessionID string
}

// NewRename returns the dialog for the session with sessionID, its title
// filled in and focused.
func NewRename(theme common.Theme, sessionID, title string, width int) (*Rename, tea.Cmd) {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 100
	input.SetWidth(max(20, width-6)) // border and padding
	input.SetStyles(textinput.DefaultStyles(theme.IsDark))
	input.SetValue(title)
	d := &Rename{theme: theme, width: width, heading: fmt.Sprintf("Rename %q", title), title: input, sessionID: sessionID}
	return d, d.title.Focus()
}

// Update handles input. Enter with an empty title does nothing.
func (d *Rename) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		keys := common.Keys.Form
		switch {
		case key.Matches(k, keys.Cancel):
			return d, cancel
		case key.Matches(k, keys.Submit):
			title := strings.TrimSpace(d.title.Value())
			if title == "" {
				return d, nil
			}
			renamed := common.RenameMsg{SessionID: d.sessionID, Title: title}
			return d, func() tea.Msg { return renamed }
		}
	}
	var cmd tea.Cmd
	d.title, cmd = d.title.Update(msg)
	return d, cmd
}

// View renders the dialog box.
func (d *Rename) View() string {
	t := d.theme
	heading := ansi.Truncate(d.heading, d.width-6, "…")
	return t.StyleDialog.Width(d.width).Render(lipgloss.JoinVertical(lipgloss.Left, t.StyleTitle.Render(heading), "", d.title.View()))
}

// Hints returns the status bar key hints.
func (d *Rename) Hints() [][2]string {
	keys := common.Keys.Form
	return [][2]string{{common.Hint(keys.Submit), "rename"}, {common.Hint(keys.Cancel), "cancel"}}
}

// Confirm asks a yes/no question before an action.
type Confirm struct {
	theme     common.Theme
	message   string
	lines     []string
	action    common.Action
	sessionID string
}

// confirmWidth is the widest a Confirm gets, border included.
const confirmWidth = 64

// NewConfirm returns a confirmation for action on a session. Lines, if any,
// are listed under the message, each cut to the dialog's width.
func NewConfirm(theme common.Theme, message string, action common.Action, sessionID string, lines ...string) *Confirm {
	return &Confirm{theme: theme, message: message, lines: lines, action: action, sessionID: sessionID}
}

// Update handles input. Clicks are relative to the dialog box: on the
// answers below the question, they answer it.
func (d *Confirm) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	var yes, no bool
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		yes, no = key.Matches(msg, common.Keys.Confirm.Yes), key.Matches(msg, common.Keys.Confirm.No)
	case tea.MouseClickMsg:
		yesText, noText := d.answers()
		x := msg.X - contentLeft
		if msg.Y == lipgloss.Height(d.View())-contentTop-1 { // the last line of content
			yes = x >= 0 && x < ansi.StringWidth(yesText)
			no = x >= lipgloss.Width(yesText+confirmSeparator) && x < lipgloss.Width(yesText+confirmSeparator+noText)
		}
	}
	switch {
	case yes:
		confirmed := common.ConfirmedMsg{Action: d.action, SessionID: d.sessionID}
		return d, func() tea.Msg { return confirmed }
	case no:
		return d, cancel
	}
	return d, nil
}

// confirmSeparator separates a Confirm's answers.
const confirmSeparator = "  ·  "

// answers returns the answers a Confirm offers, as drawn.
func (d *Confirm) answers() (yes, no string) {
	keys := common.Keys.Confirm
	return d.theme.FormatKeyHint(common.Hint(keys.Yes), "confirm"), d.theme.FormatKeyHint(common.Hint(keys.No), "cancel")
}

// View renders the dialog box.
func (d *Confirm) View() string {
	t := d.theme
	yes, no := d.answers()
	parts := []string{t.StyleTitle.Render(d.message), ""}
	for _, line := range d.lines {
		parts = append(parts, ansi.Truncate(line, confirmWidth-6, "…"))
	}
	if len(d.lines) > 0 {
		parts = append(parts, "")
	}
	body := lipgloss.JoinVertical(lipgloss.Left, append(parts, yes+t.StyleFaint.Render(confirmSeparator)+no)...)
	return t.StyleDialog.Width(min(confirmWidth, lipgloss.Width(body)+6)).Render(body)
}

// Hints returns the status bar key hints.
func (d *Confirm) Hints() [][2]string {
	keys := common.Keys.Confirm
	return [][2]string{{common.Hint(keys.Yes), "confirm"}, {common.Hint(keys.No), "cancel"}}
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

// Update handles input. Clicks are relative to the dialog box: a click on a
// choice chooses it.
func (d *Pick) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	keys := common.Keys.Pick
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Down):
			d.cursor = min(d.cursor+1, len(d.choices)-1)
		case key.Matches(msg, keys.Up):
			d.cursor = max(d.cursor-1, 0)
		case key.Matches(msg, keys.Choose):
			return d, d.choose()
		case key.Matches(msg, keys.Cancel):
			return d, cancel
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			d.cursor = min(d.cursor+1, len(d.choices)-1)
		case tea.MouseWheelUp:
			d.cursor = max(d.cursor-1, 0)
		}
	case tea.MouseClickMsg:
		// The choices are the last lines of content, below the title.
		first := lipgloss.Height(d.View()) - contentTop - len(d.choices)
		if i := msg.Y - first; i >= 0 && i < len(d.choices) {
			d.cursor = i
			return d, d.choose()
		}
	}
	return d, nil
}

// choose sends the message of the choice under the cursor.
func (d *Pick) choose() tea.Cmd {
	chosen := d.choices[d.cursor].Msg
	return func() tea.Msg { return chosen }
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
	keys := common.Keys.Pick
	return [][2]string{{common.Hint(keys.Down, keys.Up), "move"}, {common.Hint(keys.Choose), "choose"}, {common.Hint(keys.Cancel), "cancel"}}
}

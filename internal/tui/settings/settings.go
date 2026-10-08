// Package settings is the config screen: a category pane beside a field pane
// for editing Vineyard's configuration. Adapted from grapes' settings screen.
//
// The screen edits a copy of the configuration. It emits SaveConfigMsg on
// ctrl+s and CloseConfigMsg on esc; the root model writes and applies it.
package settings

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

type pane int

const (
	paneCategories pane = iota
	paneFields
)

type fieldKind int

const (
	// fieldEnum cycles through its options on enter, or opens a picker when
	// there are more than three.
	fieldEnum fieldKind = iota
	// fieldText is edited in place.
	fieldText
	// fieldProfile is one profile; enter edits its command.
	fieldProfile
	// fieldAdd adds a profile.
	fieldAdd
)

type fieldID int

const (
	idDefaultAgent fieldID = iota
	idBranchPrefix
	idAutoYes
	idProfile
	idAddProfile
)

type field struct {
	id      fieldID
	label   string
	kind    fieldKind
	options []string // for fieldEnum
	profile int      // for fieldProfile: index into cfg.Profiles
}

var categories = []string{"General", "Profiles"}

type editMode int

const (
	editNone editMode = iota
	editValue
	// editNewName asks for a new profile's name; its command is asked next.
	editNewName
)

// Model is the config screen.
type Model struct {
	cfg     config.Config
	path    string
	loadErr error
	theme   common.Theme
	width   int
	height  int

	catIdx   int
	fieldIdx int
	focus    pane
	edit     editMode
	input    textinput.Model

	message      string
	messageIsErr bool

	picking     bool
	pickOptions []string
	pickCursor  int
}

// New returns the screen editing cfg, which is saved to path. loadErr is the
// error from loading the file, if it failed; saving is then refused, since
// it would replace the user's file with defaults.
func New(cfg config.Config, path string, loadErr error, theme common.Theme) Model {
	// Without profiles, the default agent is the only one offered. Making it
	// an explicit profile lets the screen edit a single list.
	if len(cfg.Profiles) == 0 {
		cfg.Profiles = cfg.ResolvedProfiles()
	} else {
		cfg.Profiles = slices.Clone(cfg.Profiles)
	}
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 256
	input.SetStyles(textinput.DefaultStyles(theme.IsDark))
	return Model{cfg: cfg, path: path, loadErr: loadErr, theme: theme, input: input}
}

// SetSize sets the size of the screen's area.
func (m Model) SetSize(width, height int) Model {
	m.width, m.height = width, height
	m.input.SetWidth(max(10, width-40))
	return m
}

// SetTheme replaces the theme.
func (m Model) SetTheme(theme common.Theme) Model {
	m.theme = theme
	m.input.SetStyles(textinput.DefaultStyles(theme.IsDark))
	return m
}

// Config returns the edited configuration.
func (m Model) Config() config.Config { return m.cfg }

// fields lists the rows of the selected category.
func (m Model) fields() []field {
	if categories[m.catIdx] == "General" {
		names := make([]string, len(m.cfg.Profiles))
		for i, p := range m.cfg.Profiles {
			names[i] = p.Name
		}
		return []field{
			{id: idDefaultAgent, label: "Default agent", kind: fieldEnum, options: names},
			{id: idBranchPrefix, label: "Branch prefix", kind: fieldText},
			{id: idAutoYes, label: "Auto-yes for new sessions", kind: fieldEnum, options: []string{"off", "on"}},
		}
	}
	fields := make([]field, 0, len(m.cfg.Profiles)+1)
	for i, p := range m.cfg.Profiles {
		fields = append(fields, field{id: idProfile, label: p.Name, kind: fieldProfile, profile: i})
	}
	return append(fields, field{id: idAddProfile, label: "+ Add profile", kind: fieldAdd})
}

func (m Model) currentField() (field, bool) {
	fields := m.fields()
	if m.focus != paneFields || m.fieldIdx >= len(fields) {
		return field{}, false
	}
	return fields[m.fieldIdx], true
}

func (m Model) value(f field) string {
	switch f.id {
	case idDefaultAgent:
		return m.cfg.DefaultProgram
	case idBranchPrefix:
		return m.cfg.BranchPrefix
	case idAutoYes:
		if m.cfg.AutoYes {
			return "on"
		}
		return "off"
	case idProfile:
		return m.cfg.Profiles[f.profile].Program
	}
	return ""
}

func (m *Model) setValue(f field, v string) {
	switch f.id {
	case idDefaultAgent:
		m.cfg.DefaultProgram = v
	case idBranchPrefix:
		m.cfg.BranchPrefix = v
	case idAutoYes:
		m.cfg.AutoYes = v == "on"
	case idProfile:
		m.cfg.Profiles[f.profile].Program = v
	}
}

func (m *Model) setMessage(text string, isErr bool) {
	m.message, m.messageIsErr = text, isErr
}

// --- Update ---

// Update handles a message.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Cursor blinks and the like belong to the input being edited.
		if m.edit != editNone {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	switch {
	case m.picking:
		return m.updatePicking(keyMsg), nil
	case m.edit != editNone:
		return m.updateEditing(keyMsg)
	}
	return m.updateNavigating(keyMsg)
}

func (m Model) updateNavigating(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	keys := common.SettingsKeyMap
	m.setMessage("", false)
	switch {
	case key.Matches(msg, keys.Save):
		if m.loadErr != nil {
			m.setMessage("Not saved: "+m.path+" failed to load, and saving would replace it. Fix the file and restart Vineyard.", true)
			return m, nil
		}
		cfg := m.cfg
		return m, func() tea.Msg { return common.SaveConfigMsg{Config: cfg} }

	case key.Matches(msg, keys.Back):
		if m.focus == paneFields {
			m.focus = paneCategories
			return m, nil
		}
		return m, func() tea.Msg { return common.CloseConfigMsg{} }

	case key.Matches(msg, keys.Right):
		if m.focus == paneCategories {
			m.focus, m.fieldIdx = paneFields, 0
		}
	case key.Matches(msg, keys.Left):
		m.focus = paneCategories
	case key.Matches(msg, keys.Tab):
		if m.focus == paneCategories {
			m.focus = paneFields
		} else {
			m.focus = paneCategories
		}
		m.fieldIdx = 0

	case key.Matches(msg, keys.Up):
		if m.focus == paneCategories && m.catIdx > 0 {
			m.catIdx--
		} else if m.focus == paneFields && m.fieldIdx > 0 {
			m.fieldIdx--
		}
	case key.Matches(msg, keys.Down):
		if m.focus == paneCategories && m.catIdx < len(categories)-1 {
			m.catIdx++
		} else if m.focus == paneFields && m.fieldIdx < len(m.fields())-1 {
			m.fieldIdx++
		}

	case key.Matches(msg, keys.Enter):
		if m.focus == paneCategories {
			m.focus, m.fieldIdx = paneFields, 0
			return m, nil
		}
		if f, ok := m.currentField(); ok {
			return m.activate(f)
		}
	case key.Matches(msg, keys.Remove):
		if f, ok := m.currentField(); ok && f.kind == fieldProfile {
			m.removeProfile(f.profile)
		}
	}
	return m, nil
}

// activate is what enter does on a field.
func (m Model) activate(f field) (Model, tea.Cmd) {
	switch f.kind {
	case fieldEnum:
		if len(f.options) > 3 {
			m.openPicker(f)
			return m, nil
		}
		next := f.options[0]
		if i := slices.Index(f.options, m.value(f)); i >= 0 {
			next = f.options[(i+1)%len(f.options)]
		}
		m.setValue(f, next)
		return m, nil
	case fieldAdd:
		return m.startEdit(editNewName, "", "profile name")
	default:
		return m.startEdit(editValue, m.value(f), "")
	}
}

func (m Model) startEdit(mode editMode, value, placeholder string) (Model, tea.Cmd) {
	m.edit = mode
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m, m.input.Focus()
}

func (m Model) stopEdit() Model {
	m.edit = editNone
	m.input.Blur()
	return m
}

func (m Model) updateEditing(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.stopEdit(), nil
	case "enter":
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	value := strings.TrimSpace(m.input.Value())
	if m.edit == editNewName {
		switch {
		case value == "":
			m.setMessage("A profile needs a name.", true)
			return m, nil
		case slices.ContainsFunc(m.cfg.Profiles, func(p config.Profile) bool { return p.Name == value }):
			m.setMessage("A profile named "+value+" already exists.", true)
			return m, nil
		}
		// Ask for the command next, starting from the name, which is often
		// the command itself.
		m.cfg.Profiles = append(m.cfg.Profiles, config.Profile{Name: value, Program: value})
		m.fieldIdx = len(m.cfg.Profiles) - 1
		m.setMessage("Enter the command that starts "+value+".", false)
		return m.startEdit(editValue, value, "command")
	}

	f, ok := m.currentField()
	if !ok {
		return m.stopEdit(), nil
	}
	if f.kind == fieldProfile && value == "" {
		m.setMessage("A profile needs a command.", true)
		return m, nil
	}
	m.setValue(f, value)
	m.setMessage("", false)
	return m.stopEdit(), nil
}

func (m *Model) removeProfile(i int) {
	if len(m.cfg.Profiles) == 1 {
		m.setMessage("Vineyard needs at least one profile.", true)
		return
	}
	name := m.cfg.Profiles[i].Name
	m.cfg.Profiles = slices.Delete(m.cfg.Profiles, i, i+1)
	m.fieldIdx = min(m.fieldIdx, len(m.cfg.Profiles)-1)
	if m.cfg.DefaultProgram == name {
		m.cfg.DefaultProgram = m.cfg.Profiles[0].Name
		m.setMessage("Removed "+name+". The default agent is now "+m.cfg.DefaultProgram+".", false)
	}
}

// --- Picker ---

// PickerActive reports whether the option picker is open.
func (m Model) PickerActive() bool { return m.picking }

func (m *Model) openPicker(f field) {
	m.picking = true
	m.pickOptions = f.options
	m.pickCursor = max(0, slices.Index(f.options, m.value(f)))
}

func (m Model) updatePicking(msg tea.KeyPressMsg) Model {
	keys := common.SettingsKeyMap
	switch {
	case key.Matches(msg, keys.Up):
		m.pickCursor = max(0, m.pickCursor-1)
	case key.Matches(msg, keys.Down):
		m.pickCursor = min(len(m.pickOptions)-1, m.pickCursor+1)
	case key.Matches(msg, keys.Enter):
		if f, ok := m.currentField(); ok {
			m.setValue(f, m.pickOptions[m.pickCursor])
		}
		m.picking = false
	case key.Matches(msg, keys.Back):
		m.picking = false
	}
	return m
}

// PickerView renders the option picker as a box for the root model to
// overlay on the screen.
func (m Model) PickerView() string {
	t := m.theme
	f, _ := m.currentField()
	current := m.value(f)
	visible := max(3, min(len(m.pickOptions), m.height-8))
	offset := max(0, m.pickCursor-visible+1)

	width := 24
	for _, opt := range m.pickOptions {
		width = max(width, ansi.StringWidth(opt)+4)
	}
	rows := []string{t.StyleTitle.Render(f.label), ""}
	for i := offset; i < offset+visible && i < len(m.pickOptions); i++ {
		opt := m.pickOptions[i]
		prefix := "  "
		switch {
		case i == m.pickCursor:
			prefix = t.StyleTitle.Foreground(t.ColorAccent).Render("›") + " "
		case opt == current:
			prefix = t.StyleSubtitle.Render("✓") + " "
		}
		row := prefix + opt
		row += strings.Repeat(" ", max(0, width-ansi.StringWidth(row)))
		if i == m.pickCursor {
			row = t.StyleSelected.Render(row)
		}
		rows = append(rows, row)
	}
	return t.StyleDialog.Render(strings.Join(rows, "\n"))
}

// --- View ---

// Hints returns the status bar key hints.
func (m Model) Hints() [][2]string {
	switch {
	case m.picking:
		return [][2]string{{"j/k", "navigate"}, {"enter", "select"}, {"esc", "cancel"}}
	case m.edit != editNone:
		return [][2]string{{"enter", "confirm"}, {"esc", "cancel"}}
	}
	hints := [][2]string{{"j/k", "navigate"}, {"tab", "pane"}, {"enter", "edit"}}
	if f, ok := m.currentField(); ok && f.kind == fieldProfile {
		hints = append(hints, [2]string{"x", "remove"})
	}
	return append(hints, [2]string{"ctrl+s", "save"}, [2]string{"esc", "back"})
}

// View renders the screen at exactly its set size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	t := m.theme
	accent := t.StyleTitle.Foreground(t.ColorAccent)
	const catWidth = 16
	sep := t.StyleSeparator.Render("│")

	lines := []string{
		"  " + t.StyleTitle.Render("Config") + "  " + t.StyleFaint.Render(m.path),
		"",
	}

	var left []string
	for i, name := range categories {
		cell := fmt.Sprintf("  %-*s", catWidth-2, name)
		switch {
		case i == m.catIdx && m.focus == paneCategories:
			cell = accent.Render(cell)
		case i == m.catIdx:
			cell = t.StyleTitle.Render(cell)
		default:
			cell = t.StyleSubtitle.Render(cell)
		}
		left = append(left, cell)
	}

	fields := m.fields()
	labelWidth := 0
	for _, f := range fields {
		labelWidth = max(labelWidth, ansi.StringWidth(f.label))
	}
	// Keep the selected field in view when there are more than fit.
	visible := max(1, m.height-len(lines)-2)
	offset := max(0, m.fieldIdx-visible+1)
	var right []string
	for i := offset; i < len(fields) && i < offset+visible; i++ {
		f := fields[i]
		selected := m.focus == paneFields && i == m.fieldIdx
		label := fmt.Sprintf("%-*s", labelWidth+2, f.label)
		value := m.value(f)
		switch {
		case selected && m.edit != editNone:
			label, value = t.StyleTitle.Render(label), m.input.View()
		case selected:
			label, value = accent.Render(label), accent.Render(value)
		case f.kind == fieldAdd:
			label = t.StyleFaint.Render(label)
		default:
			label, value = t.StyleSubtitle.Render(label), t.StyleTitle.UnsetBold().Render(value)
		}
		right = append(right, label+value)
	}

	for i := range max(len(left), len(right)) {
		l := strings.Repeat(" ", catWidth)
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, l+sep+"  "+r)
	}

	if m.message != "" {
		style := t.StyleSubtitle
		if m.messageIsErr {
			style = t.StyleError
		}
		wrapped := style.Width(max(10, m.width-4)).Render(m.message)
		lines = append(lines, "")
		for _, line := range strings.Split(wrapped, "\n") {
			lines = append(lines, "  "+line)
		}
	}

	for i, line := range lines {
		line = ansi.Truncate(line, m.width, "")
		lines[i] = line + strings.Repeat(" ", max(0, m.width-ansi.StringWidth(line)))
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	return strings.Join(lines[:m.height], "\n")
}

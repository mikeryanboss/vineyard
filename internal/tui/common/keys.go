package common

import "charm.land/bubbles/v2/key"

// ListKeys act on the session list, which has focus by default.
type ListKeys struct {
	Up        key.Binding
	Down      key.Binding
	New       key.Binding
	NewPrompt key.Binding
	Attach    key.Binding
	Shell     key.Binding
	Kill      key.Binding
	Pause     key.Binding
	Resume    key.Binding
	Push      key.Binding
	AutoYes   key.Binding
	Tab       key.Binding
	Focus     key.Binding
	Config    key.Binding
	Issues    key.Binding
	Quit      key.Binding
}

var ListKeyMap = ListKeys{
	Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	New:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	NewPrompt: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "new with prompt")),
	Attach:    key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("enter", "attach")),
	Shell:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "shell")),
	Kill:      key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "kill")),
	Pause:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "checkout")),
	Resume:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "resume")),
	Push:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "push")),
	AutoYes:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "auto-yes")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "preview/diff")),
	Focus:     key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "scroll pane")),
	Config:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "config")),
	Issues:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "issues")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// PaneKeys act on the preview or diff pane once it has focus.
type PaneKeys struct {
	Up       key.Binding
	Down     key.Binding
	HalfUp   key.Binding
	HalfDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	NextFile key.Binding
	PrevFile key.Binding
	Toggle   key.Binding
	Collapse key.Binding
	Expand   key.Binding
	Tab      key.Binding
	Back     key.Binding
}

var PaneKeyMap = PaneKeys{
	Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	HalfUp:   key.NewBinding(key.WithKeys("ctrl+u", "pgup"), key.WithHelp("ctrl+u", "half page up")),
	HalfDown: key.NewBinding(key.WithKeys("ctrl+d", "pgdown", "space"), key.WithHelp("ctrl+d", "half page down")),
	Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	NextFile: key.NewBinding(key.WithKeys("]", "n"), key.WithHelp("]", "next file")),
	PrevFile: key.NewBinding(key.WithKeys("[", "p"), key.WithHelp("[", "prev file")),
	Toggle:   key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("enter", "fold file")),
	Collapse: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "collapse all")),
	Expand:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "expand all")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "preview/diff")),
	Back:     key.NewBinding(key.WithKeys("esc", "h", "left", "q"), key.WithHelp("esc", "back")),
}

// SettingsKeys act on the config screen.
type SettingsKeys struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Tab    key.Binding
	Enter  key.Binding
	Remove key.Binding
	Save   key.Binding
	Back   key.Binding
}

var SettingsKeyMap = SettingsKeys{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	Left:   key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "categories")),
	Right:  key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "fields")),
	Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "pane")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
	Remove: key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "remove")),
	Save:   key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

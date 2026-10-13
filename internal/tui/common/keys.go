package common

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/mikeryanboss/vineyard/internal/config"
)

// Keys are the key bindings in effect. NewModel in package tui installs the
// configuration's.
var Keys = NewKeyMap(config.DefaultKeys())

// KeyMap holds the bindings of every mode of config.DefaultKeys.
type KeyMap struct {
	Global   GlobalKeys
	List     ListKeys
	Pane     PaneKeys
	Settings SettingsKeys
	Form     FormKeys
	Confirm  ConfirmKeys
	Pick     PickKeys
}

// GlobalKeys act everywhere: Quit and Back before any other key, Detach and
// StatusBar in an attached terminal.
type GlobalKeys struct {
	Quit      key.Binding
	Back      key.Binding
	Detach    key.Binding
	StatusBar key.Binding
}

// ListKeys act on the session list, which has focus by default.
type ListKeys struct {
	Up             key.Binding
	Down           key.Binding
	New            key.Binding
	NewFromProfile key.Binding
	Terminal       key.Binding
	Open           key.Binding
	Attach         key.Binding
	Shell          key.Binding
	Kill           key.Binding
	Checkout       key.Binding
	Resume         key.Binding
	Push           key.Binding
	AutoYes        key.Binding
	NextTab        key.Binding
	PrevTab        key.Binding
	Expand         key.Binding
	Collapse       key.Binding
	Config         key.Binding
	Issues         key.Binding
	Quit           key.Binding
}

// PaneKeys act on the preview, diff, issue, terminal, or recap pane once it
// has focus.
type PaneKeys struct {
	Up        key.Binding
	Down      key.Binding
	HalfUp    key.Binding
	HalfDown  key.Binding
	Top       key.Binding
	Bottom    key.Binding
	NextFile  key.Binding
	PrevFile  key.Binding
	Fold      key.Binding
	FoldAll   key.Binding
	UnfoldAll key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	Back      key.Binding
}

// SettingsKeys act on the config screen.
type SettingsKeys struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	NextPane key.Binding
	Edit     key.Binding
	Remove   key.Binding
	Where    key.Binding
	// Multiplexer cycles a profile's multiplexer.
	Multiplexer key.Binding
	Save        key.Binding
	Back        key.Binding
}

// FormKeys act while typing into the new-session dialog or a config value.
type FormKeys struct {
	Submit     key.Binding
	Cancel     key.Binding
	NextField  key.Binding
	PrevField  key.Binding
	PrevOption key.Binding
	NextOption key.Binding
	Newline    key.Binding
	Toggle     key.Binding
}

// ConfirmKeys answer a yes/no question.
type ConfirmKeys struct {
	Yes key.Binding
	No  key.Binding
}

// PickKeys choose one of several options.
type PickKeys struct {
	Up     key.Binding
	Down   key.Binding
	Choose key.Binding
	Cancel key.Binding
}

// NewKeyMap returns the bindings of keys, which holds every mode and action
// of config.DefaultKeys, as Config.ResolvedKeys returns. It panics when keys
// and the KeyMap disagree on the actions.
func NewKeyMap(keys config.Keys) KeyMap {
	used := 0
	b := func(mode, action string) key.Binding {
		names, ok := keys[mode][action]
		if !ok {
			panic(fmt.Sprintf("keys: no %s.%s", mode, action))
		}
		used++
		return key.NewBinding(key.WithKeys(names...))
	}
	m := KeyMap{
		Global: GlobalKeys{
			Quit:      b("global", "quit"),
			Back:      b("global", "back"),
			Detach:    b("global", "detach"),
			StatusBar: b("global", "status_bar"),
		},
		List: ListKeys{
			Up:             b("list", "up"),
			Down:           b("list", "down"),
			New:            b("list", "new"),
			NewFromProfile: b("list", "new_from_profile"),
			Terminal:       b("list", "terminal"),
			Open:           b("list", "open"),
			Attach:         b("list", "attach"),
			Shell:          b("list", "shell"),
			Kill:           b("list", "kill"),
			Checkout:       b("list", "checkout"),
			Resume:         b("list", "resume"),
			Push:           b("list", "push"),
			AutoYes:        b("list", "auto_yes"),
			NextTab:        b("list", "next_tab"),
			PrevTab:        b("list", "prev_tab"),
			Expand:         b("list", "expand"),
			Collapse:       b("list", "collapse"),
			Config:         b("list", "config"),
			Issues:         b("list", "issues"),
			Quit:           b("list", "quit"),
		},
		Pane: PaneKeys{
			Up:        b("pane", "up"),
			Down:      b("pane", "down"),
			HalfUp:    b("pane", "half_up"),
			HalfDown:  b("pane", "half_down"),
			Top:       b("pane", "top"),
			Bottom:    b("pane", "bottom"),
			NextFile:  b("pane", "next_file"),
			PrevFile:  b("pane", "prev_file"),
			Fold:      b("pane", "fold"),
			FoldAll:   b("pane", "fold_all"),
			UnfoldAll: b("pane", "unfold_all"),
			NextTab:   b("pane", "next_tab"),
			PrevTab:   b("pane", "prev_tab"),
			Back:      b("pane", "back"),
		},
		Settings: SettingsKeys{
			Up:          b("settings", "up"),
			Down:        b("settings", "down"),
			Left:        b("settings", "left"),
			Right:       b("settings", "right"),
			NextPane:    b("settings", "next_pane"),
			Edit:        b("settings", "edit"),
			Remove:      b("settings", "remove"),
			Where:       b("settings", "where"),
			Multiplexer: b("settings", "multiplexer"),
			Save:        b("settings", "save"),
			Back:        b("settings", "back"),
		},
		Form: FormKeys{
			Submit:     b("form", "submit"),
			Cancel:     b("form", "cancel"),
			NextField:  b("form", "next_field"),
			PrevField:  b("form", "prev_field"),
			PrevOption: b("form", "prev_option"),
			NextOption: b("form", "next_option"),
			Newline:    b("form", "newline"),
			Toggle:     b("form", "toggle"),
		},
		Confirm: ConfirmKeys{
			Yes: b("confirm", "yes"),
			No:  b("confirm", "no"),
		},
		Pick: PickKeys{
			Up:     b("pick", "up"),
			Down:   b("pick", "down"),
			Choose: b("pick", "choose"),
			Cancel: b("pick", "cancel"),
		},
	}
	total := 0
	for _, actions := range keys {
		total += len(actions)
	}
	if used != total {
		panic(fmt.Sprintf("keys: %d actions configured, %d bound", total, used))
	}
	return m
}

// keyGlyphs are how hints show the arrow keys.
var keyGlyphs = map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→"}

// Hint names bindings for a key hint: the first key of each bound one,
// joined by slashes, or "" when none is bound. Modifiers the first key has
// are not repeated: ctrl+d/u.
func Hint(bindings ...key.Binding) string {
	var names []string
	for _, b := range bindings {
		if keys := b.Keys(); len(keys) > 0 {
			name := keys[0]
			if glyph, ok := keyGlyphs[name]; ok {
				name = glyph
			}
			if len(names) > 0 {
				// The last + is the key itself in "+" and "ctrl++".
				if i := strings.LastIndex(strings.TrimSuffix(names[0], "+"), "+"); i > 0 {
					name = strings.TrimPrefix(name, names[0][:i+1])
				}
			}
			names = append(names, name)
		}
	}
	return strings.Join(names, "/")
}

// PressTo returns "Press <key> to <what>." for b's first key, or "" when b
// is unbound.
func PressTo(b key.Binding, what string) string {
	if Hint(b) == "" {
		return ""
	}
	return "Press " + Hint(b) + " to " + what + "."
}

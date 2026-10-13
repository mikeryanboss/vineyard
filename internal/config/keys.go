package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Keys are key bindings: for each mode, such as "list", the keys of each of
// its actions, such as "kill". Keys are named as Bubble Tea names a key
// press: "k", "K", "enter", "ctrl+s", "shift+tab".
type Keys map[string]map[string][]string

// DefaultKeys returns the key bindings of every mode and action. The modes
// are:
//
//   - global: quit and back, to the session list, work everywhere; detach
//     returns from an attached terminal, and status_bar shows or hides its
//     status bar
//   - list: the session list
//   - pane: the preview, diff, issue, terminal, or recap pane once it has focus
//   - settings: the config screen
//   - form: typing into the new-session dialog or a config value
//   - confirm: a yes/no question
//   - pick: choosing one of several options
func DefaultKeys() Keys {
	return Keys{
		"global": {
			"quit":   {"ctrl+c"},
			"back":   {"ctrl+q"},
			"detach": {"ctrl+q"},
			// ctrl+\ is one of the few ctrl keys Claude Code leaves free.
			"status_bar": {"ctrl+\\"},
		},
		"list": {
			"up":               {"k", "up"},
			"down":             {"j", "down"},
			"new":              {"n"},
			"new_from_profile": {"N"},
			"terminal":         {"T"},
			"open":             {"enter"},
			"attach":           {"o"},
			"shell":            {"t"},
			"kill":             {"D"},
			"checkout":         {"c"},
			"resume":           {"r"},
			"push":             {"s"},
			"auto_yes":         {"a"},
			"next_tab":         {"tab"},
			"prev_tab":         {"shift+tab"},
			"expand":           {"l", "right"},
			"collapse":         {"h", "left"},
			"config":           {"C"},
			"issues":           {"i"},
			"quit":             {"q"},
		},
		"pane": {
			"up":         {"k", "up"},
			"down":       {"j", "down"},
			"half_up":    {"ctrl+u", "pgup"},
			"half_down":  {"ctrl+d", "pgdown", "space"},
			"top":        {"g", "home"},
			"bottom":     {"G", "end"},
			"next_file":  {"]", "n"},
			"prev_file":  {"[", "p"},
			"fold":       {"enter", "o"},
			"fold_all":   {"c"},
			"unfold_all": {"e"},
			"next_tab":   {"tab"},
			"prev_tab":   {"shift+tab"},
			"back":       {"esc", "h", "left", "q"},
		},
		"settings": {
			"up":          {"k", "up"},
			"down":        {"j", "down"},
			"left":        {"h", "left"},
			"right":       {"l", "right"},
			"next_pane":   {"tab"},
			"edit":        {"enter"},
			"remove":      {"x", "delete"},
			"where":       {"w"},
			"multiplexer": {"m"},
			"save":        {"ctrl+s"},
			"back":        {"esc"},
		},
		"form": {
			"submit":      {"enter"},
			"cancel":      {"esc"},
			"next_field":  {"tab"},
			"prev_field":  {"shift+tab"},
			"prev_option": {"left", "h"},
			"next_option": {"right", "l"},
			"newline":     {"alt+enter", "ctrl+j"},
			"toggle":      {"space"},
		},
		"confirm": {
			"yes": {"y", "Y", "enter"},
			"no":  {"n", "N", "esc", "q"},
		},
		"pick": {
			"up":     {"k", "up"},
			"down":   {"j", "down"},
			"choose": {"enter"},
			"cancel": {"esc", "q"},
		},
	}
}

// ResolvedKeys returns the default keys with those the configuration sets
// in their place, and an error when the configuration names a mode or
// action that does not exist, a key Bubble Tea never reports, a key two
// actions of a mode share, a key of the global quit or back that a mode also
// binds, or a detach or status bar key that is not ctrl and a letter or
// ctrl+\.
func (c Config) ResolvedKeys() (Keys, error) {
	keys := DefaultKeys()
	for _, mode := range slices.Sorted(maps.Keys(c.Keys)) {
		actions, ok := keys[mode]
		if !ok {
			return nil, fmt.Errorf("keys: no mode %q; the modes are %s", mode, strings.Join(slices.Sorted(maps.Keys(keys)), ", "))
		}
		for _, action := range slices.Sorted(maps.Keys(c.Keys[mode])) {
			if _, ok := actions[action]; !ok {
				return nil, fmt.Errorf("keys.%s: no action %q; the actions are %s", mode, action, strings.Join(slices.Sorted(maps.Keys(actions)), ", "))
			}
			names := c.Keys[mode][action]
			for _, name := range names {
				if err := checkKey(name); err != nil {
					return nil, fmt.Errorf("keys.%s.%s: %w", mode, action, err)
				}
			}
			actions[action] = names
		}
	}
	if len(keys["global"]["detach"]) == 0 {
		return nil, errors.New("keys.global.detach: needs a key, or an attached terminal could not be left")
	}
	for _, action := range []string{"detach", "status_bar"} {
		if err := checkAttached(keys["global"][action]); err != nil {
			return nil, fmt.Errorf("keys.global.%s: %w", action, err)
		}
	}
	for _, name := range keys["global"]["status_bar"] {
		if slices.Contains(keys["global"]["detach"], name) {
			return nil, fmt.Errorf("keys.global.status_bar: %q is bound to global detach, which comes first", name)
		}
	}
	for _, mode := range slices.Sorted(maps.Keys(keys)) {
		if mode == "global" { // detach acts only where quit and back do not
			continue
		}
		bound := map[string]string{} // key to the action bound to it
		for _, action := range slices.Sorted(maps.Keys(keys[mode])) {
			for _, name := range keys[mode][action] {
				if other, ok := bound[name]; ok {
					return nil, fmt.Errorf("keys.%s: %q is bound to both %s and %s", mode, name, other, action)
				}
				for _, global := range []string{"quit", "back"} {
					if slices.Contains(keys["global"][global], name) {
						return nil, fmt.Errorf("keys.%s.%s: %q is bound to global %s, which comes first", mode, action, name, global)
					}
				}
				bound[name] = action
			}
		}
	}
	return keys, nil
}

// modifiers are the modifier prefixes of a key name, in the order Bubble Tea
// writes them.
var modifiers = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// namedKeys are the names of keys that are not a character.
var namedKeys = func() []string {
	names := []string{
		"enter", "tab", "backspace", "esc", "space", "up", "down", "left", "right",
		"begin", "find", "insert", "delete", "select", "pgup", "pgdown", "home", "end",
	}
	for i := 1; i <= 63; i++ {
		names = append(names, "f"+strconv.Itoa(i))
	}
	return names
}()

// checkKey returns an error unless name is a key as Bubble Tea names a key
// press, which is what a binding matches.
func checkKey(name string) error {
	base, mods := name, []string{}
	if i := strings.LastIndex(strings.TrimSuffix(name, "+"), "+"); i >= 0 {
		base, mods = name[i+1:], strings.Split(name[:i], "+")
	}
	last := -1
	for _, mod := range mods {
		i := slices.Index(modifiers, mod)
		if i < 0 {
			return fmt.Errorf("%q: %q is no modifier; the modifiers are %s", name, mod, strings.Join(modifiers, ", "))
		}
		if i <= last {
			return fmt.Errorf("%q: write the modifiers in the order %s", name, strings.Join(modifiers, "+"))
		}
		last = i
	}
	if slices.Contains(namedKeys, base) {
		return nil
	}
	r, size := utf8.DecodeRuneInString(base)
	switch {
	case base == "" || size != len(base) || !unicode.IsGraphic(r) || r == ' ':
		return fmt.Errorf("%q: %q is neither one character nor a key name such as enter, space, up, pgdown, or f1", name, base)
	case len(mods) == 1 && mods[0] == "shift":
		return fmt.Errorf("%q: shift and a character is named by the character it types, such as A", name)
	case len(mods) > 0 && unicode.IsUpper(r):
		return fmt.Errorf("%q: name a modified capital letter by shift and the lowercase letter, such as ctrl+shift+a", name)
	}
	return nil
}

// checkAttached returns an error unless names are keys an attached terminal
// can act on, each ctrl and a letter, or ctrl+\: both multiplexers find those
// in what the user's terminal sends, as one byte. ctrl+i and ctrl+m send tab
// and enter.
func checkAttached(names []string) error {
	for _, name := range names {
		letter, ok := strings.CutPrefix(name, "ctrl+")
		if !ok || len(letter) != 1 || (letter[0] < 'a' || letter[0] > 'z') && letter != "\\" {
			return fmt.Errorf("%q: want ctrl and a letter, such as ctrl+q, or ctrl+\\", name)
		}
		switch letter {
		case "i":
			return fmt.Errorf("%q: the terminal sends it for tab", name)
		case "m":
			return fmt.Errorf("%q: the terminal sends it for enter", name)
		}
	}
	return nil
}

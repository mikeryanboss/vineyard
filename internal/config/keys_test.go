package config

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// A [keys.<mode>] entry replaces the action's default keys, an empty list
// unbinds it, and the actions left out keep their defaults.
func TestResolvedKeys_ReplaceTheActionsTheyName(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
[keys.list]
kill = ["x", "ctrl+k"]
auto_yes = []

[keys.global]
detach = ["ctrl+g"]
`))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := cfg.ResolvedKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		mode, action string
		want         []string
	}{
		{"list", "kill", []string{"x", "ctrl+k"}},
		{"list", "auto_yes", []string{}},
		{"list", "new", []string{"n"}},
		{"global", "detach", []string{"ctrl+g"}},
		{"global", "quit", []string{"ctrl+c"}},
	} {
		if got := keys[c.mode][c.action]; !slices.Equal(got, c.want) || got == nil {
			t.Errorf("keys.%s.%s = %#v, want %#v", c.mode, c.action, got, c.want)
		}
	}
}

// A configuration whose keys could not work is rejected as a whole, as a
// malformed file is, with an error that says what is wrong.
func TestLoad_RejectsKeysThatCannotWork(t *testing.T) {
	for _, c := range []struct{ toml, want string }{
		{"[keys.lists]\nkill = [\"x\"]", `no mode "lists"`},
		{"[keys.list]\nkil = [\"x\"]", `no action "kil"`},
		{"[keys.list]\nkill = [\"ctlr+k\"]", `"ctlr" is no modifier`},
		{"[keys.list]\nkill = [\"shift+ctrl+k\"]", "order ctrl+alt+shift"},
		{"[keys.list]\nkill = [\"shift+k\"]", "named by the character it types"},
		{"[keys.list]\nkill = [\"ctrl+K\"]", "ctrl+shift+a"},
		{"[keys.list]\nkill = [\"kill\"]", "neither one character nor a key name"},
		{"[keys.list]\nkill = [\" \"]", "neither one character nor a key name"},
		{"[keys.list]\nkill = [\"n\"]", `"n" is bound to both kill and new`},
		{"[keys.pane]\nback = [\"ctrl+c\"]", "bound to global quit"},
		{"[keys.global]\ndetach = []", "needs a key"},
		{"[keys.global]\ndetach = [\"q\"]", "ctrl and a letter"},
		{"[keys.global]\ndetach = [\"ctrl+m\"]", "sends it for enter"},
	} {
		cfg, err := Load(writeConfig(t, "auto_yes = true\n"+c.toml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one containing %q", c.toml, err, c.want)
		}
		if cfg.AutoYes {
			t.Errorf("%s: a rejected configuration must give the defaults", c.toml)
		}
	}
}

// Every default key is a key name the checks accept, so the defaults always
// load, and keys that Bubble Tea names in its own ways pass.
func TestResolvedKeys_AcceptsTheDefaultsAndUnusualNames(t *testing.T) {
	if _, err := (Config{}).ResolvedKeys(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"+", "ctrl++", "G", "?", "é", "alt+shift+tab", "f12", "ctrl+alt+delete"} {
		if err := checkKey(name); err != nil {
			t.Errorf("checkKey(%q) = %v", name, err)
		}
	}
}

// The README lists every mode, action, and default key.
func TestREADMEListsTheDefaultKeys(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, block, ok := strings.Cut(string(readme), "<!-- default keys:")
	if !ok {
		t.Fatal("README has no default keys block")
	}
	_, block, _ = strings.Cut(block, "```toml\n")
	block, _, _ = strings.Cut(block, "```")
	var listed struct{ Keys Keys }
	if err := toml.Unmarshal([]byte(block), &listed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listed.Keys, DefaultKeys()) {
		t.Errorf("README lists\n%v\nwant DefaultKeys\n%v", listed.Keys, DefaultKeys())
	}
}

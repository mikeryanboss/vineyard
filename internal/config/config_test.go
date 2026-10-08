package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(Path(home), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestLoad_MissingFileGivesDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProgram != "claude" || cfg.BranchPrefix == "" {
		t.Errorf("defaults = %+v", cfg)
	}
}

func TestLoad_PartialFileKeepsOtherDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, `auto_yes = true`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoYes || cfg.DefaultProgram != "claude" {
		t.Errorf("cfg = %+v, want auto_yes plus default program", cfg)
	}
}

func TestLoad_MalformedFileGivesDefaultsAndError(t *testing.T) {
	cfg, err := Load(writeConfig(t, "auto_yes = true\ndefault_program = [nope"))
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if cfg.AutoYes {
		t.Error("a parse error must discard the partially parsed file")
	}
}

func TestResolvedProfiles_DefaultFirst(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
default_program = "codex"

[[profiles]]
name = "claude"
program = "claude"

[[profiles]]
name = "codex"
program = "codex --full-auto"
`))
	if err != nil {
		t.Fatal(err)
	}
	profiles := cfg.ResolvedProfiles()
	if len(profiles) != 2 || profiles[0].Name != "codex" || profiles[0].Program != "codex --full-auto" {
		t.Errorf("profiles = %+v, want codex first", profiles)
	}
}

func TestResolvedProfiles_WithoutProfilesUsesDefaultProgram(t *testing.T) {
	cfg := Config{DefaultProgram: "aider --model x"}
	profiles := cfg.ResolvedProfiles()
	if len(profiles) != 1 || profiles[0].Program != "aider --model x" {
		t.Errorf("profiles = %+v", profiles)
	}
}

func TestProjectDir_DistinguishesSameNamedRepos(t *testing.T) {
	a := ProjectDir("/home", "/one/app")
	b := ProjectDir("/home", "/two/app")
	if a == b {
		t.Fatal("repos with the same name must get separate project dirs")
	}
	if filepath.Base(filepath.Dir(a)) != "projects" {
		t.Errorf("project dir %q is not under projects/", a)
	}
}

func TestHome_RespectsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VINEYARD_HOME", dir)
	got, err := Home()
	if err != nil || got != dir {
		t.Errorf("Home() = %q, %v; want %q", got, err, dir)
	}
}

func TestSave_RoundTripsThroughLoad(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home") // Save creates the directory
	want := Config{
		DefaultProgram: "yolo",
		BranchPrefix:   "agents/",
		AutoYes:        true,
		Profiles: []Profile{
			{Name: "claude", Program: "claude"},
			{Name: "yolo", Program: "claude --dangerously-skip-permissions"},
		},
	}
	if err := Save(home, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loaded %+v, want %+v", got, want)
	}
}

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

func TestProjectName_DistinguishesSameNamedRepos(t *testing.T) {
	a, b := ProjectName("/one/app"), ProjectName("/two/app")
	if a == b {
		t.Fatal("repos with the same name must get separate project names")
	}
	if !strings.HasPrefix(a, "app-") {
		t.Errorf("project name %q should start with the directory name", a)
	}
}

func TestResolveWorktreeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ dir, want string }{
		{Defaults().WorktreeDir, "/repo/.vineyard/worktrees"},
		{"../wt", "/wt"},
		{"/srv/worktrees/", "/srv/worktrees"},
		{"~/worktrees", filepath.Join(home, "worktrees")},
	} {
		got, err := ResolveWorktreeDir("/repo", c.dir)
		if err != nil || got != c.want {
			t.Errorf("ResolveWorktreeDir(%q) = %q, %v; want %q", c.dir, got, err, c.want)
		}
	}
	// An empty setting would put worktrees among the repository's own files.
	if _, err := ResolveWorktreeDir("/repo", ""); err == nil {
		t.Error("an empty worktree_dir should be an error")
	}
}

// Only the configuration is meant for git; sessions, the lock, and worktrees
// are local. A .gitignore the user edited is theirs.
func TestPrepare_IgnoresAllButConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"*", "!.gitignore", "!config.toml"} {
		if !slices.Contains(strings.Split(string(content), "\n"), line) {
			t.Errorf(".gitignore lacks %q:\n%s", line, content)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, ".gitignore")); string(content) != "mine\n" {
		t.Errorf("Prepare replaced an existing .gitignore: %q", content)
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
		WorktreeDir: "../worktrees",
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

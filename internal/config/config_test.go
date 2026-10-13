package config

import (
	"os"
	"os/exec"
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
	if !cfg.AutoYes || cfg.DefaultProgram != "claude" || !cfg.StatusBar {
		t.Errorf("cfg = %+v, want auto_yes plus default program and status bar", cfg)
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

// names returns the names of profiles, in order.
func names(profiles []Profile) []string {
	out := make([]string, len(profiles))
	for i, p := range profiles {
		out[i] = p.Name
	}
	return out
}

func TestResolvedProfiles_DefaultFirst(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
default_program = "codex"

[[profiles]]
name = "codex"
program = "codex --full-auto"
`))
	if err != nil {
		t.Fatal(err)
	}
	profiles := cfg.ResolvedProfiles()
	if got := names(profiles); !slices.Equal(got, []string{"codex", "claude", "terminal"}) || profiles[0].Program != "codex --full-auto" {
		t.Errorf("profiles = %+v, want codex first, then the built-in ones", profiles)
	}
}

func TestResolvedProfiles_DefaultsOfferClaudeAndAMainCheckoutTerminal(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{
		{Name: "claude", Program: "claude"},
		{Name: "terminal", Program: "$SHELL", MainCheckout: true},
	}
	if got := cfg.ResolvedProfiles(); !reflect.DeepEqual(got, want) {
		t.Errorf("profiles = %+v, want %+v", got, want)
	}
}

// Configs saved before terminal was built in list claude alone; they must
// still be offered terminal.
func TestResolvedProfiles_BuiltinsStayBesideAConfigsOwnProfiles(t *testing.T) {
	cfg, err := Load(writeConfig(t, "[[profiles]]\nname = 'claude'\nprogram = 'claude'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := names(cfg.ResolvedProfiles()); !slices.Equal(got, []string{"claude", "terminal"}) {
		t.Errorf("profiles = %v, want claude and terminal", got)
	}
}

// A profile named like a built-in one replaces it whole, in its place.
func TestResolvedProfiles_ConfiguredProfileReplacesBuiltin(t *testing.T) {
	cfg, err := Load(writeConfig(t, "[[profiles]]\nname = 'terminal'\nprogram = 'zsh'\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{{Name: "claude", Program: "claude"}, {Name: "terminal", Program: "zsh"}}
	if got := cfg.ResolvedProfiles(); !reflect.DeepEqual(got, want) {
		t.Errorf("profiles = %+v, want %+v", got, want)
	}
}

func TestResolvedProfiles_DisabledProfilesAreNotOffered(t *testing.T) {
	cfg, err := Load(writeConfig(t, `disabled_profiles = ["terminal"]`))
	if err != nil {
		t.Fatal(err)
	}
	if got := names(cfg.ResolvedProfiles()); !slices.Equal(got, []string{"claude"}) {
		t.Errorf("profiles = %v, want claude alone", got)
	}

	// A disabled default is not offered as a command either.
	cfg, err = Load(writeConfig(t, "default_program = 'claude'\ndisabled_profiles = ['claude']"))
	if err != nil {
		t.Fatal(err)
	}
	if got := names(cfg.ResolvedProfiles()); !slices.Equal(got, []string{"terminal"}) {
		t.Errorf("profiles = %v, want terminal alone", got)
	}

	if _, err := Load(writeConfig(t, `disabled_profiles = ["claude", "terminal"]`)); err == nil || !strings.Contains(err.Error(), "every profile is disabled") {
		t.Errorf("disabling every profile gave %v, want an error", err)
	}
}

// A file that names its own default program but no profiles predates the
// default profiles; new sessions must still run its program.
func TestResolvedProfiles_UnmatchedDefaultProgramComesFirst(t *testing.T) {
	cfg, err := Load(writeConfig(t, `default_program = "aider --model x"`))
	if err != nil {
		t.Fatal(err)
	}
	profiles := cfg.ResolvedProfiles()
	if len(profiles) != 3 || profiles[0] != (Profile{Name: "aider --model x", Program: "aider --model x"}) {
		t.Errorf("profiles = %+v, want aider first, then the built-in ones", profiles)
	}
}

func TestCustomized_KeepsOnlyWhatDiffersFromTheBuiltins(t *testing.T) {
	all := []Profile{
		{Name: "claude", Program: "claude"},
		{Name: "terminal", Program: "zsh", MainCheckout: true},
		{Name: "codex", Program: "codex"},
	}
	if got := Customized(all); !reflect.DeepEqual(got, all[1:]) {
		t.Errorf("customized = %+v, want %+v", got, all[1:])
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

// Only the configuration and templates are meant for git; sessions, the lock,
// and worktrees are local. A .gitignore the user edited is theirs.
func TestPrepare_SharesOnlyConfigAndTemplates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	dir := Dir(repo)
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.toml", "sessions.json", "worktrees/a/file"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Fields(git("status", "--porcelain", "--untracked-files=all"))
	want := []string{
		"??", ".vineyard/.gitignore", "??", ".vineyard/config.toml",
		"??", ".vineyard/templates/bug.md", "??", ".vineyard/templates/default.md",
		"??", ".vineyard/templates/research.md",
	}
	if !slices.Equal(got, want) {
		t.Errorf("git sees %q, want %q", got, want)
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

// The examples are written once; afterwards the templates are the user's.
func TestPrepare_KeepsTheUsersTemplates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(TemplatesDir(dir), "bug.md")); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(TemplatesDir(dir), "bug.md")); !os.IsNotExist(err) {
		t.Error("Prepare restored a template the user deleted")
	}
}

func TestLoadTemplates_MergesFilesAndConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(TemplatesDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"plan.md": "Plan #{{.ID}}.", "notes.txt": "not a template"} {
		if err := os.WriteFile(filepath.Join(TemplatesDir(dir), name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadTemplates(dir, []Template{{Name: "fix", Text: "Fix #{{.ID}}."}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Template{{Name: "fix", Text: "Fix #{{.ID}}."}, {Name: "plan", Text: "Plan #{{.ID}}."}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("templates = %+v, want %+v", got, want)
	}

	if _, err := LoadTemplates(dir, []Template{{Name: "plan", Text: "again"}}); err == nil || !strings.Contains(err.Error(), `"plan" is defined twice`) {
		t.Errorf("a name in both places gave %v, want an error naming it", err)
	}
	if _, err := LoadTemplates(dir, []Template{{Text: "nameless"}}); err == nil {
		t.Error("a template without a name should be an error")
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
			{Name: "yolo", Program: "claude --dangerously-skip-permissions", Multiplexer: MultiplexerVineyard},
			{Name: "dev server", Program: "npm run dev", MainCheckout: true},
		},
		DisabledProfiles: []string{"terminal"},
		WorktreeDir:      "../worktrees",
		Templates:        []Template{{Name: "fix", Text: "Fix #{{.ID}}.\n\nThen stop.\n"}},
		Multiplexer:      MultiplexerTmux,
		Keys:             Keys{"list": {"kill": {"x"}, "auto_yes": {}}},
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

// New sessions run on Vineyard's server unless the configuration or their
// profile says tmux. An empty multiplexer, which files saved while it meant
// "auto" hold, is Vineyard's server; an unknown one is an error.
func TestMultiplexerFor(t *testing.T) {
	cfg, err := Load(writeConfig(t, "multiplexer = ''\n[[profiles]]\nname = 'old'\nprogram = 'claude'\nmultiplexer = 'tmux'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.MultiplexerFor(Profile{Name: "claude"}); got != MultiplexerVineyard {
		t.Errorf("default multiplexer = %q, want %q", got, MultiplexerVineyard)
	}
	if got := cfg.MultiplexerFor(cfg.Profiles[0]); got != MultiplexerTmux {
		t.Errorf("profile multiplexer = %q, want %q", got, MultiplexerTmux)
	}
	if _, err := Load(writeConfig(t, "multiplexer = 'screen'")); err == nil {
		t.Error("an unknown multiplexer was accepted")
	}
}

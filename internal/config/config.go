// Package config loads a repository's Vineyard configuration and locates its
// data directory.
//
// Everything lives in the repository's main checkout, like grapes' .grapes:
//
//	.vineyard/config.toml      configuration, committed
//	.vineyard/templates/       prompt templates for issue sessions, committed
//	.vineyard/.gitignore       keeps everything else out of git
//	.vineyard/sessions.json    the repository's sessions
//	.vineyard/worktrees/<id>/  one worktree per session, unless worktree_dir says otherwise
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Profile is a named command for launching an agent or another program.
type Profile struct {
	Name    string `toml:"name"`
	Program string `toml:"program"`
	// MainCheckout runs the program in the repository's main checkout instead
	// of a new worktree and branch, as for a terminal beside the agents.
	MainCheckout bool `toml:"main_checkout,omitempty"`
}

// Config is the user configuration.
type Config struct {
	// DefaultProgram names the profile selected for new sessions. When no
	// profile has that name, it is used as the launch command itself.
	DefaultProgram string `toml:"default_program"`
	// BranchPrefix is prepended to the branch name of every new session.
	BranchPrefix string `toml:"branch_prefix"`
	// AutoYes makes new sessions accept agent permission prompts automatically.
	AutoYes bool `toml:"auto_yes"`
	// Profiles are launch commands offered when creating a session, after
	// the built-in ones (BuiltinProfiles). A profile named like a built-in
	// one replaces it.
	Profiles []Profile `toml:"profiles,omitempty"`
	// DisabledProfiles names profiles not to offer, built-in ones included.
	DisabledProfiles []string `toml:"disabled_profiles,omitempty"`
	// WorktreeDir is where new sessions' worktrees go: relative to the
	// repository root, absolute, or starting with "~/".
	WorktreeDir string `toml:"worktree_dir"`
	// DiffCommand shows a session's diff full-screen. It runs through sh in
	// the session's worktree, with {base} replaced by the commit where the
	// session's work begins (git.Base). When it is empty, or its first word is not an installed program,
	// Vineyard shows its own diff full-screen instead.
	DiffCommand string `toml:"diff_command"`
	// Templates are prompt templates for sessions started from an issue,
	// beside those in the templates directory.
	Templates []Template `toml:"templates,omitempty"`
	// Multiplexer runs the agents: MultiplexerTmux or MultiplexerVineyard.
	// Empty means tmux when it is installed, otherwise Vineyard's server. It
	// takes effect when Vineyard starts.
	Multiplexer string `toml:"multiplexer"`
	// Keys replace the default keys of the actions they name
	// (DefaultKeys); an empty list unbinds an action. Use ResolvedKeys.
	Keys Keys `toml:"keys,omitempty"`
}

// The multiplexers that can run agents.
const (
	MultiplexerTmux = "tmux"
	// MultiplexerVineyard is Vineyard's own terminal server.
	MultiplexerVineyard = "vineyard"
)

// Defaults returns the configuration used when no file exists.
func Defaults() Config {
	return Config{
		DefaultProgram: "claude",
		BranchPrefix:   defaultBranchPrefix(),
		WorktreeDir:    filepath.Join(DirName, "worktrees"),
		DiffCommand:    "hunk diff {base} --watch",
	}
}

// BuiltinProfiles returns the profiles every repository is offered unless
// its configuration replaces or disables them.
func BuiltinProfiles() []Profile {
	return []Profile{
		{Name: "claude", Program: "claude"},
		{Name: "terminal", Program: "$SHELL", MainCheckout: true},
	}
}

// IsBuiltin reports whether name names a built-in profile.
func IsBuiltin(name string) bool {
	return slices.ContainsFunc(BuiltinProfiles(), func(p Profile) bool { return p.Name == name })
}

// AllProfiles returns the built-in profiles, each replaced by the configured
// profile of its name, then the other configured profiles. Disabled profiles
// are included.
func (c Config) AllProfiles() []Profile {
	profiles := BuiltinProfiles()
	for _, p := range c.Profiles {
		if i := slices.IndexFunc(profiles, func(b Profile) bool { return b.Name == p.Name }); i >= 0 {
			profiles[i] = p
		} else {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// Disabled reports whether the configuration disables the profile name.
func (c Config) Disabled(name string) bool { return slices.Contains(c.DisabledProfiles, name) }

// Customized returns those of profiles, a list as AllProfiles returns, that
// a configuration must hold to give that list: the profiles that are not
// built-in, and the built-in ones changed.
func Customized(profiles []Profile) []Profile {
	builtins := BuiltinProfiles()
	return slices.DeleteFunc(slices.Clone(profiles), func(p Profile) bool { return slices.Contains(builtins, p) })
}

func defaultBranchPrefix() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "vineyard/"
	}
	return strings.ToLower(u.Username) + "/"
}

// ResolvedProfiles returns the profiles to offer, default first, without
// disabled ones. When no profile is named DefaultProgram, DefaultProgram
// comes first as a command of its own; when the profile it names is
// disabled, the first profile offered is the default.
func (c Config) ResolvedProfiles() []Profile {
	all := c.AllProfiles()
	offered := slices.DeleteFunc(slices.Clone(all), func(p Profile) bool { return c.Disabled(p.Name) })
	if i := slices.IndexFunc(offered, func(p Profile) bool { return p.Name == c.DefaultProgram }); i >= 0 {
		def := offered[i]
		return append([]Profile{def}, slices.Delete(offered, i, i+1)...)
	}
	if slices.ContainsFunc(all, func(p Profile) bool { return p.Name == c.DefaultProgram }) {
		return offered
	}
	return append([]Profile{{Name: c.DefaultProgram, Program: c.DefaultProgram}}, offered...)
}

// DirName is the name of the data directory in a repository's main checkout.
const DirName = ".vineyard"

// gitignore keeps everything in the data directory but the configuration and
// templates out of git.
const gitignore = "# Written by vineyard: only the configuration and templates are shared.\n" +
	"*\n!.gitignore\n!config.toml\n!templates/\n!templates/*\n"

// Dir returns the data directory of the repository whose main checkout is
// repoRoot.
func Dir(repoRoot string) string { return filepath.Join(repoRoot, DirName) }

// Prepare creates the data directory dir, its .gitignore, and its templates
// directory with the example templates, each when missing. An existing
// .gitignore or templates directory is left as the user edited it.
func Prepare(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(gitignore), 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return writeExamples(TemplatesDir(dir))
}

// Path returns the location of the configuration file in dir.
func Path(dir string) string { return filepath.Join(dir, "config.toml") }

// ProjectName names the repository whose main checkout is repoRoot, for tmux
// session names on the shared socket. It combines the directory name, for
// humans, with a hash of the full path, so two checkouts with the same name
// never share a name.
func ProjectName(repoRoot string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(repoRoot)))
	return filepath.Base(repoRoot) + "-" + hex.EncodeToString(sum[:4])
}

// ResolveWorktreeDir returns the absolute directory that worktreeDir names
// for the repository whose main checkout is repoRoot.
func ResolveWorktreeDir(repoRoot, worktreeDir string) (string, error) {
	switch {
	case worktreeDir == "":
		// Not the repository root: worktrees would land among its files.
		return "", errors.New("worktree_dir is empty")
	case strings.HasPrefix(worktreeDir, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, worktreeDir[2:]), nil
	case filepath.IsAbs(worktreeDir):
		return filepath.Clean(worktreeDir), nil
	}
	return filepath.Join(repoRoot, worktreeDir), nil
}

// Save writes cfg to the configuration file in dir. It writes a temporary
// file and renames it into place, so a crash mid-write cannot corrupt the
// file. Comments in a hand-written file are not preserved.
func Save(dir string, cfg Config) error {
	content, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), Path(dir))
}

// Load reads the configuration file in dir. A missing file yields defaults.
// A malformed file yields clean defaults plus the parse error, so the TUI can
// start and report the problem instead of refusing to run.
func Load(dir string) (Config, error) {
	cfg := Defaults()
	content, err := os.ReadFile(Path(dir))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	// Defaults are set before unmarshalling, so omitted fields keep them.
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return Defaults(), fmt.Errorf("%s: %w", Path(dir), err)
	}
	if len(cfg.ResolvedProfiles()) == 0 {
		return Defaults(), fmt.Errorf("%s: every profile is disabled", Path(dir))
	}
	if _, err := cfg.ResolvedKeys(); err != nil {
		return Defaults(), fmt.Errorf("%s: %w", Path(dir), err)
	}
	return cfg, nil
}

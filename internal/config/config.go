// Package config loads a repository's Vineyard configuration and locates its
// data directory.
//
// Everything lives in the repository's main checkout, like grapes' .grapes:
//
//	.vineyard/config.toml      configuration, committed
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
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Profile is a named command for launching an agent.
type Profile struct {
	Name    string `toml:"name"`
	Program string `toml:"program"`
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
	// Profiles are the launch commands offered when creating a session.
	Profiles []Profile `toml:"profiles"`
	// WorktreeDir is where new sessions' worktrees go: relative to the
	// repository root, absolute, or starting with "~/".
	WorktreeDir string `toml:"worktree_dir"`
	// DiffCommand shows a session's diff full-screen. It runs through sh in
	// the session's worktree, with {base} replaced by the session's base
	// commit. When it is empty, or its first word is not an installed program,
	// Vineyard shows its own diff full-screen instead.
	DiffCommand string `toml:"diff_command"`
}

// Defaults returns the configuration used when no file exists.
func Defaults() Config {
	return Config{
		DefaultProgram: "claude",
		BranchPrefix:   defaultBranchPrefix(),
		WorktreeDir:    filepath.Join(DirName, "worktrees"),
		DiffCommand:    "hunk diff {base} --watch",
	}
}

func defaultBranchPrefix() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "vineyard/"
	}
	return strings.ToLower(u.Username) + "/"
}

// ResolvedProfiles returns the profiles to offer, default first. Without
// configured profiles, DefaultProgram becomes the only one.
func (c Config) ResolvedProfiles() []Profile {
	if len(c.Profiles) == 0 {
		return []Profile{{Name: c.DefaultProgram, Program: c.DefaultProgram}}
	}
	profiles := make([]Profile, 0, len(c.Profiles))
	for _, p := range c.Profiles {
		if p.Name == c.DefaultProgram {
			profiles = append([]Profile{p}, profiles...)
		} else {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// DirName is the name of the data directory in a repository's main checkout.
const DirName = ".vineyard"

// gitignore keeps everything in the data directory but the configuration out
// of git.
const gitignore = "# Written by vineyard: only the configuration is shared.\n*\n!.gitignore\n!config.toml\n"

// Dir returns the data directory of the repository whose main checkout is
// repoRoot.
func Dir(repoRoot string) string { return filepath.Join(repoRoot, DirName) }

// Prepare creates the data directory dir, and its .gitignore when missing.
// An existing .gitignore is left as the user edited it.
func Prepare(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(gitignore), 0o644)
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
	return cfg, nil
}

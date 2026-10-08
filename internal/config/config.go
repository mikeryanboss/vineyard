// Package config loads Vineyard's user configuration and locates its data
// directory.
//
// Everything lives under one home directory, ~/.vineyard by default:
//
//	~/.vineyard/config.toml                         user configuration
//	~/.vineyard/projects/<project>/sessions.json    sessions for one repository
//	~/.vineyard/projects/<project>/worktrees/<id>/  one worktree per session
//
// VINEYARD_HOME overrides the home directory.
package config

import (
	"crypto/sha256"
	"encoding/hex"
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
}

// Defaults returns the configuration used when no file exists.
func Defaults() Config {
	return Config{
		DefaultProgram: "claude",
		BranchPrefix:   defaultBranchPrefix(),
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

// Home returns Vineyard's home directory.
func Home() (string, error) {
	if dir := os.Getenv("VINEYARD_HOME"); dir != "" {
		return filepath.Abs(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".vineyard"), nil
}

// Path returns the location of the configuration file.
func Path(home string) string { return filepath.Join(home, "config.toml") }

// ProjectDir returns the data directory for the repository rooted at repoRoot.
// The name combines the directory name, for humans, with a hash of the full
// path, so two checkouts with the same name never share sessions.
func ProjectDir(home, repoRoot string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(repoRoot)))
	name := filepath.Base(repoRoot) + "-" + hex.EncodeToString(sum[:4])
	return filepath.Join(home, "projects", name)
}

// Save writes cfg to the configuration file in home. It writes a temporary
// file and renames it into place, so a crash mid-write cannot corrupt the
// file. Comments in a hand-written file are not preserved.
func Save(home string, cfg Config) error {
	content, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(home, ".config-*.toml")
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
	return os.Rename(tmp.Name(), Path(home))
}

// Load reads the configuration file in home. A missing file yields defaults.
// A malformed file yields clean defaults plus the parse error, so the TUI can
// start and report the problem instead of refusing to run.
func Load(home string) (Config, error) {
	cfg := Defaults()
	content, err := os.ReadFile(Path(home))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	// Defaults are set before unmarshalling, so omitted fields keep them.
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return Defaults(), fmt.Errorf("%s: %w", Path(home), err)
	}
	return cfg, nil
}

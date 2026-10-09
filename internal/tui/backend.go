package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tmux"
)

// Backend is everything the TUI does outside its own process. The root model
// calls it only from commands, never from Update, except for the attach
// commands, which merely build an *exec.Cmd.
//
// Tests substitute a fake to drive the model without tmux or git.
type Backend interface {
	New(opts session.NewOptions) session.Session
	Start(s session.Session, width, height int) (session.Session, error)
	Pause(s session.Session) (session.Session, error)
	Resume(s session.Session, width, height int) (session.Session, error)
	Kill(s session.Session) (session.KillResult, error)
	Push(s session.Session) (session.Session, error)
	EnsureShell(s session.Session, width, height int) error

	Capture(s session.Session) (string, error)
	CaptureHistory(s session.Session, lines int) (string, error)
	Resize(s session.Session, width, height int) error
	SendEnter(s session.Session) error
	Paste(s session.Session, text string) error
	AttachCommand(s session.Session) *exec.Cmd
	ShellAttachCommand(s session.Session) *exec.Cmd
	// DiffTool builds the configured diff command for s. Its error wraps
	// exec.ErrNotFound when the command is empty or its program is missing.
	DiffTool(s session.Session, command string) (*exec.Cmd, error)

	Diff(s session.Session) (string, error)
	DiffStat(s session.Session) (git.Stat, error)
	Branch(s session.Session) (string, error)

	// Recap reads what Claude Code has recorded about s.
	Recap(s session.Session) (recap.Recap, error)

	Save(sessions []session.Session) error
	SaveConfig(cfg config.Config) error
}

// LiveBackend is the Backend backed by tmux, git, and the session store.
type LiveBackend struct {
	*session.Manager
	Tmux  *tmux.Client
	Store session.Store
	// Dir is the repository's .vineyard directory, which holds config.toml.
	Dir string
}

func (b LiveBackend) Capture(s session.Session) (string, error) {
	return b.Tmux.Capture(s.TmuxName)
}

func (b LiveBackend) CaptureHistory(s session.Session, lines int) (string, error) {
	return b.Tmux.CaptureHistory(s.TmuxName, lines)
}

func (b LiveBackend) Resize(s session.Session, width, height int) error {
	return b.Tmux.Resize(s.TmuxName, width, height)
}

func (b LiveBackend) SendEnter(s session.Session) error { return b.Tmux.SendEnter(s.TmuxName) }

func (b LiveBackend) Paste(s session.Session, text string) error {
	return b.Tmux.Paste(s.TmuxName, text)
}

func (b LiveBackend) AttachCommand(s session.Session) *exec.Cmd {
	return b.Tmux.AttachCommand(s.TmuxName)
}

func (b LiveBackend) ShellAttachCommand(s session.Session) *exec.Cmd {
	return b.Tmux.AttachCommand(s.ShellName())
}

func (b LiveBackend) DiffTool(s session.Session, command string) (*exec.Cmd, error) {
	return diffTool(s, command)
}

// diffTool runs command through sh in s's worktree, with {base} replaced by
// s's base commit, once its first word is found on PATH.
func diffTool(s session.Session, command string) (*exec.Cmd, error) {
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil, fmt.Errorf("no diff tool configured: %w", exec.ErrNotFound)
	}
	if _, err := exec.LookPath(words[0]); err != nil {
		return nil, err
	}
	cmd := exec.Command("sh", "-c", strings.ReplaceAll(command, "{base}", s.BaseCommit))
	cmd.Dir = s.WorktreePath
	return cmd, nil
}

func (b LiveBackend) Diff(s session.Session) (string, error) {
	return git.Diff(s.WorktreePath, s.BaseCommit)
}

func (b LiveBackend) DiffStat(s session.Session) (git.Stat, error) {
	return git.DiffStat(s.WorktreePath, s.BaseCommit)
}

func (b LiveBackend) Branch(s session.Session) (string, error) {
	return git.CurrentBranch(s.WorktreePath)
}

func (b LiveBackend) Recap(s session.Session) (recap.Recap, error) {
	dir, err := recap.ProjectsDir()
	if err != nil {
		return recap.Recap{}, err
	}
	return recap.Load(dir, s.WorktreePath)
}

func (b LiveBackend) Save(sessions []session.Session) error { return b.Store.Save(sessions) }

func (b LiveBackend) SaveConfig(cfg config.Config) error { return config.Save(b.Dir, cfg) }

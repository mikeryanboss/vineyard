package tui

import (
	"os/exec"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
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
	Push(s session.Session) error
	EnsureShell(s session.Session, width, height int) error

	Capture(s session.Session) (string, error)
	CaptureHistory(s session.Session, lines int) (string, error)
	Resize(s session.Session, width, height int) error
	SendEnter(s session.Session) error
	Paste(s session.Session, text string) error
	AttachCommand(s session.Session) *exec.Cmd
	ShellAttachCommand(s session.Session) *exec.Cmd

	Diff(s session.Session) (string, error)
	DiffStat(s session.Session) (git.Stat, error)

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

func (b LiveBackend) Diff(s session.Session) (string, error) {
	return git.Diff(s.WorktreePath, s.BaseCommit)
}

func (b LiveBackend) DiffStat(s session.Session) (git.Stat, error) {
	return git.DiffStat(s.WorktreePath, s.BaseCommit)
}

func (b LiveBackend) Save(sessions []session.Session) error { return b.Store.Save(sessions) }

func (b LiveBackend) SaveConfig(cfg config.Config) error { return config.Save(b.Dir, cfg) }

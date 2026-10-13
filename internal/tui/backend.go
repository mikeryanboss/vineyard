package tui

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
)

// Backend is everything the TUI does outside its own process. The root model
// calls it only from commands, never from Update, except for the attach
// commands, which merely build a command and run nothing.
//
// Tests substitute a fake to drive the model without tmux or git.
type Backend interface {
	New(opts session.NewOptions) session.Session
	Start(s session.Session, width, height int) (session.Session, error)
	Pause(s session.Session) (session.Session, error)
	Resume(s session.Session, width, height int) (session.Session, error)
	Kill(s session.Session, discardCommits bool) (session.KillResult, error)
	Push(s session.Session) (session.Session, error)
	EnsureShell(s session.Session, width, height int) error

	Capture(s session.Session) (string, error)
	CaptureHistory(s session.Session, lines int) (string, error)
	Resize(s session.Session, width, height int) error
	SendEnter(s session.Session) error
	Paste(s session.Session, text string) error
	AttachCommand(s session.Session) tea.ExecCommand
	ShellAttachCommand(s session.Session) tea.ExecCommand
	// CaptureShell, CaptureShellHistory, and ResizeShell are Capture,
	// CaptureHistory, and Resize for the session's shell.
	CaptureShell(s session.Session) (string, error)
	CaptureShellHistory(s session.Session, lines int) (string, error)
	ResizeShell(s session.Session, width, height int) error
	// OpenSubagent makes s show the view of the subagent at position in
	// Claude Code's list, which must still hold total subagents.
	OpenSubagent(s session.Session, position, total int) error
	// Subagents reads what Claude Code's transcripts say about the subagents
	// of s's conversation.
	Subagents(s session.Session) (subagent.Conversation, error)
	// SubagentEvents reads the condensed transcript of s's subagent id.
	SubagentEvents(s session.Session, id string) ([]subagent.Event, error)
	// Worktree returns the root of the git worktree holding dir, or "" when
	// none does.
	Worktree(dir string) string
	// TreeDiff and TreeDiffTool are Diff and DiffTool for the worktree at
	// root, which a subagent of s works in: its work begins where its branch
	// forks from the branch of s or from the mainline, whichever is later.
	TreeDiff(root string, s session.Session) (string, error)
	TreeDiffTool(root string, s session.Session, command string) (*exec.Cmd, error)
	// DiffTool builds the configured diff command for s. Its error wraps
	// exec.ErrNotFound when the command is empty or its program is missing.
	DiffTool(s session.Session, command string) (*exec.Cmd, error)

	Diff(s session.Session) (string, error)
	DiffStat(s session.Session) (git.Stat, error)
	Branch(s session.Session) (string, error)

	// Recap reads what Claude Code has recorded about s.
	Recap(s session.Session) (recap.Recap, error)

	// Add, Replace, and Remove change one saved session, leaving the
	// sessions of other Vineyards on the repository alone.
	Add(s session.Session) error
	Replace(s session.Session) error
	Remove(id string) error
	// Load reads the saved sessions and finds their worktrees, falling back
	// to <worktreeDir>/<id>.
	Load(worktreeDir string) ([]session.Session, error)
	// Lead reports whether this process leads the repository's Vineyards,
	// claiming the role if it is free. Only the leader types into agents.
	Lead() (bool, error)
	SaveConfig(cfg config.Config) error
	// Templates reads the prompt templates, together with inline, those in
	// the configuration.
	Templates(inline []config.Template) ([]config.Template, error)
}

// Terminal runs agents and shells in named terminals of a multiplexer: on
// Vineyard's tmux server (*tmux.Client), or on Vineyard's own terminal server
// (*vterm.Client). Each session runs in the multiplexer it names.
type Terminal interface {
	session.Terminal
	// Created returns when the named terminal started its program, to the
	// second or better.
	Created(name string) (time.Time, error)
	Capture(name string) (string, error)
	CaptureHistory(name string, lines int) (string, error)
	Resize(name string, width, height int) error
	SendEnter(name string) error
	// SendKeys presses keys named as tmux names them, such as Down.
	SendKeys(name string, keys ...string) error
	Paste(name, text string) error
	// Attach hands the user's terminal to the named terminal until the user
	// presses one of the detach keys, each ctrl and a letter such as
	// "ctrl+q".
	Attach(name string, detach []string) tea.ExecCommand
}

// LiveBackend is the Backend backed by the multiplexers, git, and the session
// store.
type LiveBackend struct {
	*session.Manager
	// Terminals are the multiplexers available, by name, as in the Manager.
	Terminals map[string]Terminal
	Store     session.Store
	Leader    *session.Leader
	// Transcripts reads Claude Code's transcripts, remembering where it left
	// off in each.
	Transcripts *subagent.Reader
	// Dir is the repository's .vineyard directory, which holds config.toml.
	Dir string
}

// terminal returns the multiplexer that runs s, or one whose every call
// fails when that multiplexer is not available.
func (b LiveBackend) terminal(s session.Session) Terminal {
	if t, ok := b.Terminals[s.Multiplexer]; ok {
		return t
	}
	return unavailable{s}
}

func (b LiveBackend) Add(s session.Session) error     { return b.Store.Add(s) }
func (b LiveBackend) Replace(s session.Session) error { return b.Store.Replace(s) }
func (b LiveBackend) Remove(id string) error          { return b.Store.Remove(id) }
func (b LiveBackend) Lead() (bool, error)             { return b.Leader.Lead() }

func (b LiveBackend) Load(worktreeDir string) ([]session.Session, error) {
	saved, err := b.Store.Load()
	if err != nil {
		return nil, err
	}
	return b.Manager.Locate(saved, worktreeDir)
}

func (b LiveBackend) Capture(s session.Session) (string, error) {
	return b.terminal(s).Capture(s.TmuxName)
}

func (b LiveBackend) CaptureHistory(s session.Session, lines int) (string, error) {
	return b.terminal(s).CaptureHistory(s.TmuxName, lines)
}

func (b LiveBackend) Resize(s session.Session, width, height int) error {
	return b.terminal(s).Resize(s.TmuxName, width, height)
}

func (b LiveBackend) SendEnter(s session.Session) error { return b.terminal(s).SendEnter(s.TmuxName) }

func (b LiveBackend) Paste(s session.Session, text string) error {
	return b.terminal(s).Paste(s.TmuxName, text)
}

func (b LiveBackend) AttachCommand(s session.Session) tea.ExecCommand {
	return b.terminal(s).Attach(s.TmuxName, common.Keys.Global.Detach.Keys())
}

func (b LiveBackend) ShellAttachCommand(s session.Session) tea.ExecCommand {
	return b.terminal(s).Attach(s.ShellName(), common.Keys.Global.Detach.Keys())
}

func (b LiveBackend) CaptureShell(s session.Session) (string, error) {
	return b.terminal(s).Capture(s.ShellName())
}

func (b LiveBackend) CaptureShellHistory(s session.Session, lines int) (string, error) {
	return b.terminal(s).CaptureHistory(s.ShellName(), lines)
}

func (b LiveBackend) ResizeShell(s session.Session, width, height int) error {
	return b.terminal(s).Resize(s.ShellName(), width, height)
}

func (b LiveBackend) OpenSubagent(s session.Session, position, total int) error {
	return subagent.Open(terminalScreen{b.terminal(s), s.TmuxName}, position, total)
}

func (b LiveBackend) Subagents(s session.Session) (subagent.Conversation, error) {
	dir, err := recap.ProjectsDir()
	if err != nil {
		return subagent.Conversation{}, err
	}
	// The terminal runs Claude Code itself, so it started with it. tmux
	// counts whole seconds; an exiting Claude Code, as on a pause, writes
	// until the moment its successor starts, which may be within that
	// second, while the successor needs longer than a second to start and be
	// sent a prompt before it writes. So the second after counts as the start.
	created, err := b.terminal(s).Created(s.TmuxName)
	if err != nil {
		return subagent.Conversation{}, err
	}
	return b.Transcripts.Load(recap.Dir(dir, s.WorktreePath), created.Add(time.Second))
}

func (b LiveBackend) SubagentEvents(s session.Session, id string) ([]subagent.Event, error) {
	dir, err := recap.ProjectsDir()
	if err != nil {
		return nil, err
	}
	return b.Transcripts.Events(recap.Dir(dir, s.WorktreePath), id)
}

func (b LiveBackend) Worktree(dir string) string { return git.Root(dir) }

func (b LiveBackend) TreeDiff(root string, s session.Session) (string, error) {
	base, err := treeBase(root, s)
	if err != nil {
		return "", err
	}
	return git.Diff(root, base)
}

func (b LiveBackend) TreeDiffTool(root string, s session.Session, command string) (*exec.Cmd, error) {
	base, err := treeBase(root, s)
	if err != nil {
		return nil, err
	}
	return diffTool(root, base, command)
}

// treeBase is where the work in the worktree at root, which a subagent of s
// works in, begins. Subagents often create their branches from that of s.
func treeBase(root string, s session.Session) (string, error) {
	onto, err := git.Head(s.WorktreePath)
	if err != nil {
		return "", err
	}
	return git.StackedBase(root, onto)
}

// terminalScreen is a session's terminal as subagent.Open drives it.
type terminalScreen struct {
	terminal Terminal
	name     string
}

func (w terminalScreen) Capture() (string, error)      { return w.terminal.Capture(w.name) }
func (w terminalScreen) SendKeys(keys ...string) error { return w.terminal.SendKeys(w.name, keys...) }

func (b LiveBackend) DiffTool(s session.Session, command string) (*exec.Cmd, error) {
	base, err := git.Base(s.WorktreePath, s.BaseCommit)
	if err != nil {
		return nil, err
	}
	return diffTool(s.WorktreePath, base, command)
}

// diffTool runs command through sh in dir, with {base} replaced by base, once
// its first word is found on PATH.
func diffTool(dir, base, command string) (*exec.Cmd, error) {
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil, fmt.Errorf("no diff tool configured: %w", exec.ErrNotFound)
	}
	if _, err := exec.LookPath(words[0]); err != nil {
		return nil, err
	}
	cmd := exec.Command("sh", "-c", strings.ReplaceAll(command, "{base}", base))
	cmd.Dir = dir
	return cmd, nil
}

func (b LiveBackend) Diff(s session.Session) (string, error) {
	base, err := git.Base(s.WorktreePath, s.BaseCommit)
	if err != nil {
		return "", err
	}
	return git.Diff(s.WorktreePath, base)
}

func (b LiveBackend) DiffStat(s session.Session) (git.Stat, error) {
	base, err := git.Base(s.WorktreePath, s.BaseCommit)
	if err != nil {
		return git.Stat{}, err
	}
	return git.DiffStat(s.WorktreePath, base)
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

func (b LiveBackend) SaveConfig(cfg config.Config) error { return config.Save(b.Dir, cfg) }

func (b LiveBackend) Templates(inline []config.Template) ([]config.Template, error) {
	return config.LoadTemplates(b.Dir, inline)
}

// unavailable is the multiplexer of a session whose multiplexer is not
// available, such as tmux when it is not installed. It runs nothing.
type unavailable struct{ s session.Session }

func (u unavailable) err() error { return session.Unavailable(u.s) }

func (u unavailable) Start(string, string, string, int, int) error { return u.err() }
func (u unavailable) Exists(string) bool                           { return false }
func (u unavailable) Kill(string) error                            { return nil }
func (u unavailable) Created(string) (time.Time, error)            { return time.Time{}, u.err() }
func (u unavailable) Capture(string) (string, error)               { return "", u.err() }
func (u unavailable) CaptureHistory(string, int) (string, error)   { return "", u.err() }
func (u unavailable) Resize(string, int, int) error                { return u.err() }
func (u unavailable) SendEnter(string) error                       { return u.err() }
func (u unavailable) SendKeys(string, ...string) error             { return u.err() }
func (u unavailable) Paste(string, string) error                   { return u.err() }
func (u unavailable) Attach(string, []string) tea.ExecCommand      { return failing{u.err()} }

// failing is a command that fails with err without running anything.
type failing struct{ err error }

func (f failing) Run() error          { return f.err }
func (f failing) SetStdin(io.Reader)  {}
func (f failing) SetStdout(io.Writer) {}
func (f failing) SetStderr(io.Writer) {}

package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mikeryanboss/vineyard/internal/git"
)

// Terminal runs programs in named, detachable terminal sessions.
// *tmux.Client implements it.
type Terminal interface {
	Start(name, dir, program string, width, height int) error
	Exists(name string) bool
	Kill(name string) error
}

// Manager performs session lifecycle operations for one repository.
//
// Its methods take a Session value and return the updated value instead of
// mutating shared state. The TUI runs them in background commands and applies
// the results in its update loop, so they must not touch anything else.
type Manager struct {
	Repo     git.Repo
	Terminal Terminal
	// Project names the repository in tmux session names, which share one
	// socket across repositories.
	Project string

	now func() time.Time
}

// NewManager returns a manager for repo, named project in tmux.
func NewManager(repo git.Repo, terminal Terminal, project string) *Manager {
	return &Manager{
		Repo:     repo,
		Terminal: terminal,
		Project:  project,
		now:      time.Now,
	}
}

// NewOptions describe a session to create.
type NewOptions struct {
	Title   string
	Program string
	Prompt  string
	AutoYes bool
	// BranchPrefix is prepended to the session's branch name.
	BranchPrefix string
	// WorktreeDir is the absolute directory the session's worktree goes in.
	WorktreeDir string
	// Issue is the grapes issue the session works on, or 0. Its branch is
	// then named "<issue>/<slug>" instead of using BranchPrefix.
	Issue int
}

// New describes a session without creating anything, so the TUI can show it
// immediately while Start does the slow work.
func (m *Manager) New(opts NewOptions) Session {
	now := m.now()
	id := newID(opts.Title, now)
	branch := opts.BranchPrefix + Slug(opts.Title, id)
	if opts.Issue > 0 {
		branch = strconv.Itoa(opts.Issue) + "/" + Slug(opts.Title, id)
	}
	return Session{
		ID:            id,
		Title:         opts.Title,
		Program:       opts.Program,
		Branch:        branch,
		Issue:         opts.Issue,
		WorktreePath:  filepath.Join(opts.WorktreeDir, id),
		TmuxName:      "vineyard-" + m.Project + "-" + id,
		Status:        StatusLoading,
		AutoYes:       opts.AutoYes,
		PendingPrompt: opts.Prompt,
		CreatedAt:     now,
	}
}

// Start creates the session's branch and worktree from the current HEAD and
// launches its program. If launching fails, the worktree and branch are
// removed again so a failed start leaves nothing behind.
func (m *Manager) Start(s Session, width, height int) (Session, error) {
	base, err := git.Head(m.Repo.Checkout)
	if err != nil {
		return s, err
	}
	s.BaseCommit = base
	s.Branch = m.freeBranch(s.Branch)
	if err := git.AddWorktree(m.Repo.Root, s.WorktreePath, s.Branch, base); err != nil {
		return s, err
	}
	if err := m.Terminal.Start(s.TmuxName, s.WorktreePath, s.Program, width, height); err != nil {
		cleanup := errors.Join(
			git.RemoveWorktree(m.Repo.Root, s.WorktreePath),
			git.DeleteBranch(m.Repo.Root, s.Branch),
		)
		return s, errors.Join(fmt.Errorf("starting %q: %w", s.Program, err), cleanup)
	}
	s.Status = StatusRunning
	return s, nil
}

// freeBranch returns name, numbered if a branch by that name exists.
func (m *Manager) freeBranch(name string) string {
	branch := name
	for n := 2; git.BranchExists(m.Repo.Root, branch); n++ {
		branch = name + "-" + strconv.Itoa(n)
	}
	return branch
}

// Pause commits the session's work to its branch, stops the agent, and
// removes the worktree. The branch can then be checked out elsewhere.
func (m *Manager) Pause(s Session) (Session, error) {
	if git.IsWorktree(s.WorktreePath) {
		if _, err := git.CommitAll(s.WorktreePath, fmt.Sprintf("vineyard: checkpoint %q", s.Title)); err != nil {
			return s, fmt.Errorf("committing before pause: %w", err)
		}
	}
	if err := m.stopTerminals(s); err != nil {
		return s, err
	}
	if err := git.RemoveWorktree(m.Repo.Root, s.WorktreePath); err != nil {
		return s, err
	}
	s.Status = StatusPaused
	return s, nil
}

// Resume recreates a paused session's worktree from its branch, if needed,
// and relaunches its program. It also restarts stopped sessions, whose
// worktree, and any uncommitted work in it, is still on disk.
func (m *Manager) Resume(s Session, width, height int) (Session, error) {
	if !git.IsWorktree(s.WorktreePath) {
		checkout, err := git.BranchCheckout(m.Repo.Root, s.Branch)
		if err != nil {
			return s, err
		}
		if checkout != "" {
			return s, fmt.Errorf("branch %s is checked out at %s; switch that checkout to another branch first", s.Branch, checkout)
		}
		if err := git.AddWorktreeForBranch(m.Repo.Root, s.WorktreePath, s.Branch); err != nil {
			return s, err
		}
	}
	if !m.Terminal.Exists(s.TmuxName) {
		if err := m.Terminal.Start(s.TmuxName, s.WorktreePath, s.Program, width, height); err != nil {
			return s, err
		}
	}
	s.Status = StatusRunning
	return s, nil
}

// KillResult reports what Kill left behind.
type KillResult struct {
	// KeptBranch is set when the branch has commits and was kept, so that
	// killing a session never throws away committed work.
	KeptBranch bool
}

// Kill stops the agent and removes the worktree, discarding uncommitted
// changes. The branch is deleted only if it has no commits of its own.
func (m *Manager) Kill(s Session) (KillResult, error) {
	if err := m.stopTerminals(s); err != nil {
		return KillResult{}, err
	}
	if err := git.RemoveWorktree(m.Repo.Root, s.WorktreePath); err != nil {
		return KillResult{}, err
	}
	if s.Branch == "" || !git.BranchExists(m.Repo.Root, s.Branch) {
		return KillResult{}, nil
	}
	ahead, err := git.CommitsAhead(m.Repo.Root, s.BaseCommit, s.Branch)
	if err != nil || ahead > 0 {
		return KillResult{KeptBranch: true}, err
	}
	return KillResult{}, git.DeleteBranch(m.Repo.Root, s.Branch)
}

// stopTerminals ends the agent and the session's shell, if one is open.
func (m *Manager) stopTerminals(s Session) error {
	return errors.Join(m.Terminal.Kill(s.TmuxName), m.Terminal.Kill(s.ShellName()))
}

// EnsureShell starts the session's shell in its worktree unless it is running.
func (m *Manager) EnsureShell(s Session, width, height int) error {
	if !git.IsWorktree(s.WorktreePath) {
		return fmt.Errorf("session %q has no worktree; resume it first", s.Title)
	}
	if m.Terminal.Exists(s.ShellName()) {
		return nil
	}
	return m.Terminal.Start(s.ShellName(), s.WorktreePath, "", width, height)
}

// Push commits the session's work and pushes its branch to origin.
func (m *Manager) Push(s Session) error {
	if _, err := git.CommitAll(s.WorktreePath, fmt.Sprintf("vineyard: update from %q", s.Title)); err != nil {
		return err
	}
	return git.Push(s.WorktreePath, s.Branch)
}

// Restore reconciles saved sessions with reality after a restart. A session
// whose tmux session is gone becomes stopped; its worktree is left alone.
func (m *Manager) Restore(sessions []Session) []Session {
	out := make([]Session, len(sessions))
	for i, s := range sessions {
		switch {
		case s.Status == StatusPaused:
		case m.Terminal.Exists(s.TmuxName):
			s.Status = StatusReady
		default:
			s.Status = StatusStopped
		}
		out[i] = s
	}
	return out
}

// Package session defines an agent session — one agent program running in a
// tmux session inside its own git worktree — and manages its lifecycle.
package session

import (
	"fmt"
	"strings"
	"time"
)

// Status is a session's lifecycle state.
type Status string

const (
	// StatusLoading means the worktree or tmux session is being set up.
	StatusLoading Status = "loading"
	// StatusRunning means the agent's screen is changing: it is working.
	StatusRunning Status = "running"
	// StatusReady means the agent's screen is still: it is waiting for input.
	StatusReady Status = "ready"
	// StatusPaused means the worktree was removed and the branch kept.
	StatusPaused Status = "paused"
	// StatusStopped means the tmux session ended (the agent exited, or the
	// tmux server died) while the worktree is still on disk.
	StatusStopped Status = "stopped"
)

// Active reports whether the session has a live tmux session to watch.
func (s Status) Active() bool {
	return s == StatusRunning || s == StatusReady
}

// Session is one agent working in its own worktree.
type Session struct {
	// ID is stable and unique within a project. It names the worktree directory.
	ID string `json:"id"`
	// Title is the human name entered when the session was created.
	Title string `json:"title"`
	// Program is the shell command that launches the agent.
	Program string `json:"program"`
	// Branch is the git branch the session works on.
	Branch string `json:"branch"`
	// BaseCommit is the commit the branch started from. Diffs compare against it.
	BaseCommit string `json:"base_commit"`
	// WorktreePath is where the session's checkout lives.
	WorktreePath string `json:"worktree_path"`
	// TmuxName is the session's name on Vineyard's tmux server.
	TmuxName string `json:"tmux_name"`
	// Status is the last known lifecycle state.
	Status Status `json:"status"`
	// AutoYes accepts the agent's permission prompts automatically.
	AutoYes bool `json:"auto_yes"`
	// PendingPrompt is typed into the agent once it is first ready for input.
	PendingPrompt string `json:"pending_prompt,omitempty"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`
}

// ShellName is the tmux session name of the session's shell, a plain shell in
// its worktree for running commands next to the agent.
func (s Session) ShellName() string { return s.TmuxName + "-shell" }

// newID derives a readable, unique ID from a title and creation time.
func newID(title string, now time.Time) string {
	return fmt.Sprintf("%s-%x", Slug(title, "session"), now.UnixNano()&0xffffff)
}

// Slug reduces s to lowercase letters, digits, and single dashes, which are
// safe in branch names, tmux session names, and paths.
func Slug(s, fallback string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		return fallback
	}
	return slug
}

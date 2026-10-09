package session

import (
	"errors"
	"path/filepath"
)

// Leader is this process's claim to lead a project. Every Vineyard on a
// repository polls every agent's screen, so only the one that leads types
// into agents: it answers auto-yes prompts and delivers pending prompts, and
// no keystroke is sent twice. The role passes to another Vineyard when the
// leader exits and its lock is released.
type Leader struct {
	path string
	// release keeps the lock's file open, and so the lock held, until exit.
	release func()
}

// NewLeader returns the lead role for a project data directory, not yet claimed.
func NewLeader(projectDir string) *Leader {
	return &Leader{path: filepath.Join(projectDir, "lock")}
}

// Lead reports whether this process leads, claiming the role if it is free.
func (l *Leader) Lead() (bool, error) {
	if l.release != nil {
		return true, nil
	}
	release, err := lock(l.path, false)
	if errors.Is(err, errBusy) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	l.release = release
	return true, nil
}

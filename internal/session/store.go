package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const stateVersion = 1

// Store persists a project's sessions as JSON. Several Vineyards may share a
// store: each change reads the file and writes it back under a lock, and
// touches only the sessions it is about.
type Store struct {
	path string
}

type stateFile struct {
	Version  int       `json:"version"`
	Sessions []Session `json:"sessions"`
}

// NewStore returns the store for a project data directory.
func NewStore(projectDir string) Store {
	return Store{path: filepath.Join(projectDir, "sessions.json")}
}

// Path returns the state file's location.
func (s Store) Path() string { return s.path }

// Load reads the saved sessions. A missing file means no sessions.
func (s Store) Load() ([]Session, error) {
	content, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state stateFile
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	if state.Version != stateVersion {
		return nil, fmt.Errorf("%s: unsupported state version %d", s.path, state.Version)
	}
	return state.Sessions, nil
}

// Update changes the saved sessions under the store's lock, so concurrent
// Vineyards never lose each other's changes. change receives the sessions as
// saved now, and what it returns is saved.
func (s Store) Update(change func([]Session) ([]Session, error)) ([]Session, error) {
	release, err := lock(filepath.Join(filepath.Dir(s.path), "sessions.lock"), true)
	if err != nil {
		return nil, err
	}
	defer release()
	sessions, err := s.Load()
	if err != nil {
		return nil, err
	}
	if sessions, err = change(sessions); err != nil {
		return nil, err
	}
	return sessions, s.save(sessions)
}

// Add saves a new session.
func (s Store) Add(session Session) error {
	_, err := s.Update(func(saved []Session) ([]Session, error) {
		return append(saved, session), nil
	})
	return err
}

// Replace saves a changed session. A session no longer saved stays gone:
// another Vineyard has killed it.
func (s Store) Replace(session Session) error {
	_, err := s.Update(func(saved []Session) ([]Session, error) {
		if i := slices.IndexFunc(saved, func(o Session) bool { return o.ID == session.ID }); i >= 0 {
			saved[i] = session
		}
		return saved, nil
	})
	return err
}

// Remove deletes a saved session.
func (s Store) Remove(id string) error {
	_, err := s.Update(func(saved []Session) ([]Session, error) {
		return slices.DeleteFunc(saved, func(o Session) bool { return o.ID == id }), nil
	})
	return err
}

// save replaces the saved sessions. The file is written to a temporary name
// and renamed into place, so a reader never sees half a file.
func (s Store) save(sessions []Session) error {
	if sessions == nil {
		sessions = []Session{}
	}
	content, err := json.MarshalIndent(stateFile{Version: stateVersion, Sessions: sessions}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".sessions-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(append(content, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateVersion = 1

// ErrLocked reports that another Vineyard process owns the project.
var ErrLocked = errors.New("another vineyard is already running for this repository")

// Store persists a project's sessions as JSON.
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

// Save replaces the saved sessions. The file is written to a temporary name
// and renamed into place, so a crash mid-write cannot corrupt it.
func (s Store) Save(sessions []Session) error {
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

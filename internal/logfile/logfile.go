// Package logfile keeps Vineyard's logs, which record failures for later
// diagnosis: the TUI's and Vineyard's server's.
package logfile

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// maxSize is the size past which Open starts a log afresh.
const maxSize = 1 << 20

// Open opens the log at path for appending. When the log has grown past
// maxSize, it first moves it to path.old, replacing the previous one.
func Open(path string) (*os.File, error) {
	if info, err := os.Stat(path); err == nil && info.Size() > maxSize {
		if err := os.Rename(path, path+".old"); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// Tail returns the last n lines of the log at path, or "" when there is no log.
func Tail(path string, n int) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n"), nil
}

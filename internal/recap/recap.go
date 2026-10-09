// Package recap reads what Claude Code has recorded about a session: its
// recaps, its title, and its last prompt.
//
// Claude Code keeps one JSON-lines transcript per conversation, under a
// directory named after the conversation's working directory. It writes a
// recap into the transcript when the user returns after being away. None of
// this is documented; the format was read off Claude Code 2.1.295.
package recap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Entry is one recap.
type Entry struct {
	Time time.Time
	Text string
}

// Recap is what the transcripts of one working directory say.
type Recap struct {
	// Found reports whether there is any transcript at all.
	Found bool
	// Title and LastPrompt come from the most recently written transcript.
	Title      string
	LastPrompt string
	// Entries are the recaps of every transcript, newest first.
	Entries []Entry
}

// ProjectsDir returns where Claude Code keeps its transcripts.
func ProjectsDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "projects"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// dirName is the name of the transcript directory for working directory cwd.
func dirName(cwd string) string { return nonAlphanumeric.ReplaceAllString(cwd, "-") }

// hint is the suffix Claude Code appends to every recap.
const hint = " (disable recaps in /config)"

// line holds the fields of the transcript lines Load reads.
type line struct {
	Type       string    `json:"type"`
	Subtype    string    `json:"subtype"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	AITitle    string    `json:"aiTitle"`
	LastPrompt string    `json:"lastPrompt"`
}

// Markers that a transcript line is one Load reads. Checking for them before
// decoding skips the bulk of a transcript, which is tool output.
var markers = [][]byte{[]byte(`"away_summary"`), []byte(`"ai-title"`), []byte(`"last-prompt"`)}

// Load reads the transcripts Claude Code wrote while working in cwd.
func Load(projectsDir, cwd string) (Recap, error) {
	paths, err := filepath.Glob(filepath.Join(projectsDir, dirName(cwd), "*.jsonl"))
	if err != nil {
		return Recap{}, err
	}
	var r Recap
	var newest time.Time
	for _, path := range paths {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the glob
		}
		if err != nil {
			return Recap{}, err
		}
		title, prompt, entries, err := read(path)
		if err != nil {
			return Recap{}, err
		}
		r.Found = true
		r.Entries = append(r.Entries, entries...)
		if info.ModTime().After(newest) {
			newest = info.ModTime()
			r.Title, r.LastPrompt = title, prompt
		}
	}
	slices.SortFunc(r.Entries, func(a, b Entry) int { return b.Time.Compare(a.Time) })
	return r, nil
}

// read returns one transcript's last title, last prompt, and recaps.
func read(path string) (title, prompt string, entries []Entry, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 64<<20) // a line holds a whole tool result
	for n := 1; scanner.Scan(); n++ {
		raw := scanner.Bytes()
		if !slices.ContainsFunc(markers, func(m []byte) bool { return bytes.Contains(raw, m) }) {
			continue
		}
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			return "", "", nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		switch {
		case l.Type == "system" && l.Subtype == "away_summary":
			entries = append(entries, Entry{Time: l.Timestamp, Text: strings.TrimSuffix(l.Content, hint)})
		case l.Type == "ai-title":
			title = l.AITitle
		case l.Type == "last-prompt":
			prompt = l.LastPrompt
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", nil, fmt.Errorf("%s: %w", path, err)
	}
	return title, prompt, entries, nil
}

// Package recap reads what Claude Code has recorded about a session: its
// title, the user's messages, and its recaps.
//
// Claude Code keeps one JSON-lines transcript per conversation, under a
// directory named after the conversation's working directory. It writes a
// recap into the transcript when the user returns after being away. None of
// this is documented; the format was read off Claude Code 2.1.280 to 2.1.296.
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

// Entry is one message the user typed, or one recap.
type Entry struct {
	Time time.Time
	Text string
	// Recap reports whether Claude Code wrote the entry as a recap, rather
	// than the user typing it.
	Recap bool
}

// Recap is what the transcripts of one working directory say.
type Recap struct {
	// Found reports whether there is any transcript at all.
	Found bool
	// Title comes from the most recently written transcript.
	Title string
	// Entries are the messages and recaps of every transcript, oldest first.
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

// Dir is where Claude Code keeps the transcripts of conversations it had in
// working directory cwd.
func Dir(projectsDir, cwd string) string {
	return filepath.Join(projectsDir, nonAlphanumeric.ReplaceAllString(cwd, "-"))
}

// hint is the suffix Claude Code appends to every recap.
const hint = " (disable recaps in /config)"

// line holds the fields of the transcript lines Load reads.
type line struct {
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	AITitle   string    `json:"aiTitle"`
	// Origin's kind is "human" on the messages the user typed, and empty on
	// tool results, slash commands, and the text Claude Code adds.
	Origin struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		// Content is a string, or a list of blocks.
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// Markers that a transcript line is one Load reads. Checking for them before
// decoding skips the bulk of a transcript, which is tool output.
var markers = [][]byte{[]byte(`"away_summary"`), []byte(`"ai-title"`), []byte(`"origin":{"kind":"human"}`)}

// Load reads the transcripts Claude Code wrote while working in cwd.
func Load(projectsDir, cwd string) (Recap, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(projectsDir, cwd), "*.jsonl"))
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
		title, entries, err := read(path)
		if err != nil {
			return Recap{}, err
		}
		r.Found = true
		r.Entries = append(r.Entries, entries...)
		if info.ModTime().After(newest) {
			newest = info.ModTime()
			r.Title = title
		}
	}
	slices.SortStableFunc(r.Entries, func(a, b Entry) int { return a.Time.Compare(b.Time) })
	return r, nil
}

// read returns one transcript's last title, and its messages and recaps.
func read(path string) (title string, entries []Entry, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
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
			return "", nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		switch {
		case l.Type == "system" && l.Subtype == "away_summary":
			entries = append(entries, Entry{Time: l.Timestamp, Text: strings.TrimSuffix(l.Content, hint), Recap: true})
		case l.Type == "user" && l.Origin.Kind == "human":
			text, err := messageText(l.Message.Content)
			if err != nil {
				return "", nil, fmt.Errorf("%s:%d: %w", path, n, err)
			}
			entries = append(entries, Entry{Time: l.Timestamp, Text: text})
		case l.Type == "ai-title":
			title = l.AITitle
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("%s: %w", path, err)
	}
	return title, entries, nil
}

// messageText returns a message's text: the string it holds, or its text
// blocks, leaving out images.
func messageText(content json.RawMessage) (string, error) {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return "", err
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n\n"), nil
}

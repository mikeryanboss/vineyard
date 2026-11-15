// Package recap reads what Claude Code and Antigravity CLI have recorded about
// a session: its title, the user's prompts, the agent's final responses, and
// Claude Code's away recaps.
//
// Claude Code keeps one JSON-lines transcript per conversation, under a
// directory named after the conversation's working directory, writing an
// away_summary recap when the user returns after being away. Antigravity CLI
// keeps prompt history in history.jsonl, per-step JSON-lines transcripts under
// brain/<id>/.system_generated/logs/, and titles under annotations/<id>.pbtxt.
package recap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Kind distinguishes user prompts, final agent responses, and away recaps.
type Kind int

const (
	KindPrompt Kind = iota
	KindResponse
	KindRecap
)

// Entry is one user prompt, one final agent response, or one away recap.
type Entry struct {
	Time time.Time
	Text string
	Kind Kind
}

// Recap is what the transcripts of one working directory say.
type Recap struct {
	// Found reports whether there is any transcript or history at all.
	Found bool
	// Title comes from the most recently written conversation.
	Title string
	// Entries are the prompts, final responses, and recaps, oldest first.
	Entries []Entry
}

// Equal reports whether r and o have the same presence, title, and entries.
func (r Recap) Equal(o Recap) bool {
	return r.Found == o.Found && r.Title == o.Title &&
		slices.EqualFunc(r.Entries, o.Entries, func(a, b Entry) bool {
			return a.Kind == b.Kind && a.Text == b.Text && a.Time.Equal(b.Time)
		})
}

// ConfigDir returns Claude Code's own directory, which holds its transcripts.
func ConfigDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// ProjectsDir returns where Claude Code keeps its transcripts.
func ProjectsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projects"), nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// Dir is where Claude Code keeps the transcripts of conversations it had in
// working directory cwd.
func Dir(projectsDir, cwd string) string {
	return filepath.Join(projectsDir, nonAlphanumeric.ReplaceAllString(cwd, "-"))
}

// hint is the suffix Claude Code appends to every recap.
const hint = " (disable recaps in /config)"

// line holds the fields of the Claude Code transcript lines Load reads.
type line struct {
	Type        string    `json:"type"`
	Subtype     string    `json:"subtype"`
	IsSidechain bool      `json:"isSidechain"`
	IsAPIError  bool      `json:"isApiErrorMessage"`
	Content     string    `json:"content"`
	Timestamp   time.Time `json:"timestamp"`
	AITitle     string    `json:"aiTitle"`
	// Origin's kind is "human" on the messages the user typed, and empty on
	// tool results, slash commands, and the text Claude Code adds.
	Origin struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		Model      string          `json:"model"`
		StopReason string          `json:"stop_reason"`
		Content    json.RawMessage `json:"content"`
	} `json:"message"`
}

// Markers that a transcript line is one Load reads. Checking for them before
// decoding skips the bulk of a transcript, which is tool output.
var markers = [][]byte{
	[]byte(`"away_summary"`),
	[]byte(`"ai-title"`),
	[]byte(`"origin":{"kind":"human"}`),
	[]byte(`"end_turn"`),
}

// Reader reads Claude Code and Antigravity CLI transcripts incrementally
// across polls, keeping previously parsed entries per file so unchanged logs
// are not re-scanned. It is safe for concurrent use.
type Reader struct {
	mu    sync.Mutex
	files map[string]*transcript
	agy   agyState
}

type transcript struct {
	info         fs.FileInfo
	offset       int64
	lineNo       int
	title        string
	entries      []Entry
	tailTitle    string
	hasTailTitle bool
	tailEntries  []Entry
}

// NewReader returns a Reader with no cached transcript state.
func NewReader() *Reader {
	return &Reader{
		files: map[string]*transcript{},
		agy:   newAgyState(),
	}
}

// Load reads the transcripts Claude Code wrote while working in cwd.
func Load(projectsDir, cwd string) (Recap, error) {
	return NewReader().Load(projectsDir, cwd)
}

// Load reads the transcripts Claude Code wrote while working in cwd, resuming
// each unchanged file from the byte offset of its last complete line.
func (r *Reader) Load(projectsDir, cwd string) (Recap, error) {
	targetDir := Dir(projectsDir, cwd)
	dirEntries, err := os.ReadDir(targetDir)
	if errors.Is(err, fs.ErrNotExist) {
		r.mu.Lock()
		r.pruneDirLocked(targetDir, nil)
		r.mu.Unlock()
		return Recap{}, nil
	}
	if err != nil {
		return Recap{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	present := make(map[string]bool, len(dirEntries))
	var rec Recap
	var newest time.Time
	for _, de := range dirEntries {
		name := de.Name()
		if de.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		path := filepath.Join(targetDir, name)
		info, err := de.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since ReadDir
		}
		if err != nil {
			return Recap{}, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			info, err = os.Stat(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return Recap{}, err
			}
		}
		if !info.Mode().IsRegular() {
			continue
		}
		present[path] = true

		t, err := r.updateLocked(path, info)
		if errors.Is(err, fs.ErrNotExist) {
			delete(r.files, path)
			continue
		}
		if err != nil {
			return Recap{}, err
		}
		rec.Found = true
		rec.Entries = append(rec.Entries, t.entries...)
		rec.Entries = append(rec.Entries, t.tailEntries...)
		title := t.title
		if t.hasTailTitle {
			title = t.tailTitle
		}
		if t.info.ModTime().After(newest) {
			newest = t.info.ModTime()
			rec.Title = title
		}
	}
	r.pruneDirLocked(targetDir, present)
	slices.SortStableFunc(rec.Entries, func(a, b Entry) int { return a.Time.Compare(b.Time) })
	return rec, nil
}

func (r *Reader) pruneDirLocked(dir string, present map[string]bool) {
	for path := range r.files {
		if filepath.Dir(path) == dir && !present[path] {
			delete(r.files, path)
		}
	}
}

func (r *Reader) updateLocked(path string, info fs.FileInfo) (*transcript, error) {
	t := r.files[path]
	if t == nil || t.info == nil || !os.SameFile(t.info, info) || info.Size() < t.info.Size() ||
		(info.Size() == t.info.Size() && !info.ModTime().Equal(t.info.ModTime())) {
		t = &transcript{lineNo: 1}
		r.files[path] = t
	}
	if t.info != nil && info.Size() == t.info.Size() {
		return t, nil
	}
	if info.Size() == 0 && t.offset == 0 {
		t.info = info
		return t, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, fInfo) || fInfo.Size() < t.offset || (t.info != nil && fInfo.Size() < t.info.Size()) {
		t = &transcript{lineNo: 1}
		r.files[path] = t
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, err
	}

	t.tailTitle, t.hasTailTitle = "", false
	t.tailEntries = nil
	reader := bufio.NewReaderSize(f, 64<<10)
	for {
		raw, readErr := reader.ReadBytes('\n')
		if readErr != nil && readErr != io.EOF {
			delete(r.files, path)
			return nil, fmt.Errorf("%s: %w", path, readErr)
		}
		if len(raw) == 0 && readErr == io.EOF {
			break
		}
		var parsedEntry Entry
		var parsedTitle string
		var hasEntry, hasTitle bool
		if slices.ContainsFunc(markers, func(m []byte) bool { return bytes.Contains(raw, m) }) {
			trimmed := bytes.TrimRight(raw, "\r\n")
			var l line
			if err := json.Unmarshal(trimmed, &l); err != nil {
				if readErr == io.EOF {
					break // partial line mid-write at EOF
				}
				lineNo := t.lineNo
				delete(r.files, path)
				return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			switch {
			case l.Type == "system" && l.Subtype == "away_summary":
				parsedEntry = Entry{Time: l.Timestamp, Text: strings.TrimSuffix(l.Content, hint), Kind: KindRecap}
				hasEntry = true
			case l.Type == "user" && l.Origin.Kind == "human":
				text, err := messageText(l.Message.Content)
				if err != nil {
					if readErr == io.EOF {
						break
					}
					lineNo := t.lineNo
					delete(r.files, path)
					return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
				}
				parsedEntry = Entry{Time: l.Timestamp, Text: text, Kind: KindPrompt}
				hasEntry = true
			case l.Type == "assistant" && !l.IsSidechain && !l.IsAPIError && l.Message.StopReason == "end_turn" && l.Message.Model != "<synthetic>":
				text, err := messageText(l.Message.Content)
				if err != nil {
					if readErr == io.EOF {
						break
					}
					lineNo := t.lineNo
					delete(r.files, path)
					return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
				}
				if text = strings.TrimRight(text, "\n"); strings.TrimSpace(text) != "" {
					parsedEntry = Entry{Time: l.Timestamp, Text: text, Kind: KindResponse}
					hasEntry = true
				}
			case l.Type == "ai-title":
				parsedTitle = l.AITitle
				hasTitle = true
			}
		}
		if readErr == io.EOF {
			if hasEntry {
				t.tailEntries = append(t.tailEntries, parsedEntry)
			}
			if hasTitle {
				t.tailTitle, t.hasTailTitle = parsedTitle, true
			}
			break // leave offset before an unterminated EOF line
		}
		if hasEntry {
			t.entries = append(t.entries, parsedEntry)
		}
		if hasTitle {
			t.title = parsedTitle
		}
		t.offset += int64(len(raw))
		t.lineNo++
	}
	t.info = fInfo
	return t, nil
}

// messageText returns a message's text: the string it holds, or its text
// blocks, leaving out images and thinking blocks.
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

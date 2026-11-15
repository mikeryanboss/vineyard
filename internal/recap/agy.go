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
	"slices"
	"strconv"
	"strings"
	"time"
)

// AntigravityDir returns where Antigravity CLI keeps its history, brain
// transcripts, and annotations.
func AntigravityDir() (string, error) {
	if dir := os.Getenv("ANTIGRAVITY_GEMINI_DIR"); dir != "" {
		return filepath.Join(dir, "antigravity-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "antigravity-cli"), nil
}

// LoadAgy reads the prompts, final responses, and conversation title recorded
// by Antigravity CLI for working directory cwd. conversationID is an optional
// known conversation ID for cwd (for example from Session.Conversation).
func LoadAgy(agyDir, cwd, conversationID string) (Recap, error) {
	return NewReader().LoadAgy(agyDir, cwd, conversationID)
}

type agyState struct {
	histPath    string
	histInfo    fs.FileInfo
	histOffset  int64
	histLineNo  int
	histRecords []agyHistoryRecord
	histTail    *agyHistoryRecord

	convs      map[string]*agyTranscript
	firstSteps map[string]agyFirstStep
	titles     map[string]agyTitleCache
}

func newAgyState() agyState {
	return agyState{
		convs:      map[string]*agyTranscript{},
		firstSteps: map[string]agyFirstStep{},
		titles:     map[string]agyTitleCache{},
	}
}

type agyHistoryRecord struct {
	workspace string
	convID    string
	display   string
	isPrompt  bool
	at        time.Time
}

type agyTranscript struct {
	info        fs.FileInfo
	offset      int64
	lineNo      int
	entries     []Entry
	tailEntries []Entry
}

type agyFirstStep struct {
	info  fs.FileInfo
	text  string
	at    time.Time
	valid bool
}

type agyTitleCache struct {
	info  fs.FileInfo
	title string
}

type agyStepLine struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Content   string    `json:"content"`
}

var agyMarkers = [][]byte{
	[]byte(`"USER_INPUT"`),
	[]byte(`"PLANNER_RESPONSE"`),
}

// agyFirstPromptWindow bounds the clock skew between history.jsonl's submit
// timestamp and step 0's RFC3339 created_at in brain/<id>/.../transcript.jsonl.
const agyFirstPromptWindow = 30 * time.Second

// LoadAgy reads the prompts, final responses, and conversation title recorded
// by Antigravity CLI for working directory cwd, reusing cached offsets for
// unchanged files.
func (r *Reader) LoadAgy(agyDir, cwd, conversationID string) (Recap, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.updateAgyHistoryLocked(filepath.Join(agyDir, "history.jsonl")); err != nil {
		return Recap{}, err
	}

	var seenWorkspace bool
	var convIDs []string
	var prompts []agyHistoryRecord
	addConv := func(id string) {
		if id != "" && !slices.Contains(convIDs, id) {
			convIDs = append(convIDs, id)
		}
	}

	visitRecord := func(rec agyHistoryRecord) {
		if rec.workspace != cwd {
			return
		}
		seenWorkspace = true
		addConv(rec.convID)
		if rec.isPrompt {
			prompts = append(prompts, rec)
		}
	}
	for _, rec := range r.agy.histRecords {
		visitRecord(rec)
	}
	if r.agy.histTail != nil {
		visitRecord(*r.agy.histTail)
	}
	addConv(conversationID)

	// Load transcripts of already-known conversation IDs first.
	loaded := map[string][]Entry{}
	var newest time.Time
	var newestID string
	loadConv := func(id string) error {
		if _, done := loaded[id]; done {
			return nil
		}
		path, info, ok, err := statAgyTranscript(agyDir, id)
		if err != nil {
			return err
		}
		if !ok {
			delete(r.agy.convs, agyTranscriptFullPath(agyDir, id))
			delete(r.agy.convs, agyTranscriptCompactPath(agyDir, id))
			loaded[id] = nil
			return nil
		}
		t, err := r.updateAgyTranscriptLocked(path, info)
		if errors.Is(err, fs.ErrNotExist) {
			delete(r.agy.convs, path)
			loaded[id] = nil
			return nil
		}
		if err != nil {
			return err
		}
		entries := make([]Entry, 0, len(t.entries)+len(t.tailEntries))
		entries = append(entries, t.entries...)
		entries = append(entries, t.tailEntries...)
		loaded[id] = entries
		if t.info.ModTime().After(newest) {
			newest = t.info.ModTime()
			newestID = id
		}
		return nil
	}

	for _, id := range convIDs {
		if err := loadConv(id); err != nil {
			return Recap{}, err
		}
	}

	// On the first prompt of a fresh launch, history.jsonl omits conversationId.
	// Match any unlinked prompt for cwd against step 0 of brain/<id> transcripts.
	promptMatched := func(p agyHistoryRecord) bool {
		for _, entries := range loaded {
			for _, e := range entries {
				if e.Kind == KindPrompt && e.Text == p.display && p.at.Sub(e.Time).Abs() <= agyFirstPromptWindow {
					return true
				}
			}
		}
		return false
	}

	needBrainScan := false
	for _, p := range prompts {
		if p.convID == "" && !promptMatched(p) {
			needBrainScan = true
			break
		}
	}
	if needBrainScan {
		matched, err := r.matchUnlinkedPromptsLocked(agyDir, prompts, promptMatched)
		if err != nil {
			return Recap{}, err
		}
		for _, m := range matched {
			addConv(m.convID)
			if err := loadConv(m.convID); err != nil {
				return Recap{}, err
			}
			for i := range r.agy.histRecords {
				rec := &r.agy.histRecords[i]
				if rec.workspace == cwd && rec.convID == "" && rec.isPrompt && rec.display == m.prompt.display && rec.at.Equal(m.prompt.at) {
					rec.convID = m.convID
				}
			}
		}
	}

	var rec Recap
	rec.Found = seenWorkspace
	for _, id := range convIDs {
		entries := loaded[id]
		if entries != nil {
			rec.Found = true
			rec.Entries = append(rec.Entries, entries...)
		}
	}

	// Include any prompt recorded in history.jsonl for cwd whose transcript has
	// not yet flushed that USER_INPUT step to disk.
	for _, p := range prompts {
		if !slices.ContainsFunc(rec.Entries, func(e Entry) bool {
			return e.Kind == KindPrompt && e.Text == p.display && p.at.Sub(e.Time).Abs() <= agyFirstPromptWindow
		}) {
			rec.Entries = append(rec.Entries, Entry{Time: p.at, Text: p.display, Kind: KindPrompt})
		}
	}

	if newestID != "" {
		title, err := r.readAgyTitleLocked(filepath.Join(agyDir, "annotations", newestID+".pbtxt"))
		if err != nil {
			return Recap{}, err
		}
		rec.Title = title
	}

	slices.SortStableFunc(rec.Entries, func(a, b Entry) int { return a.Time.Compare(b.Time) })
	return rec, nil
}

func (r *Reader) resetAgyHistoryLocked(path string) {
	r.agy.histPath = path
	r.agy.histInfo = nil
	r.agy.histOffset = 0
	r.agy.histLineNo = 1
	r.agy.histRecords = nil
	r.agy.histTail = nil
}

func (r *Reader) updateAgyHistoryLocked(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		r.resetAgyHistoryLocked("")
		return nil
	}
	if err != nil {
		return err
	}
	if r.agy.histPath != path || r.agy.histInfo == nil || !os.SameFile(r.agy.histInfo, info) ||
		info.Size() < r.agy.histInfo.Size() ||
		(info.Size() == r.agy.histInfo.Size() && !info.ModTime().Equal(r.agy.histInfo.ModTime())) {
		r.resetAgyHistoryLocked(path)
	}
	if r.agy.histInfo != nil && info.Size() == r.agy.histInfo.Size() {
		return nil
	}
	if info.Size() == 0 && r.agy.histOffset == 0 {
		r.agy.histInfo = info
		return nil
	}

	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		r.resetAgyHistoryLocked("")
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	fInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, fInfo) || fInfo.Size() < r.agy.histOffset || (r.agy.histInfo != nil && fInfo.Size() < r.agy.histInfo.Size()) {
		r.resetAgyHistoryLocked(path)
	}
	if _, err := f.Seek(r.agy.histOffset, io.SeekStart); err != nil {
		return err
	}

	r.agy.histTail = nil
	reader := bufio.NewReaderSize(f, 64<<10)
	for {
		raw, readErr := reader.ReadBytes('\n')
		if readErr != nil && readErr != io.EOF {
			r.resetAgyHistoryLocked("")
			return fmt.Errorf("%s: %w", path, readErr)
		}
		if len(raw) == 0 && readErr == io.EOF {
			break
		}
		trimmed := bytes.TrimSpace(raw)
		var parsed agyHistoryRecord
		var hasRecord bool
		if len(trimmed) > 0 {
			var entry struct {
				Display        string `json:"display"`
				Timestamp      int64  `json:"timestamp"`
				Workspace      string `json:"workspace"`
				ConversationID string `json:"conversationId"`
				Type           string `json:"type"`
			}
			if err := json.Unmarshal(trimmed, &entry); err != nil {
				if readErr == io.EOF {
					break // partial line mid-write at EOF
				}
				lineNo := r.agy.histLineNo
				r.resetAgyHistoryLocked("")
				return fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			if entry.Workspace != "" {
				text := strings.TrimRight(entry.Display, "\n")
				parsed = agyHistoryRecord{
					workspace: entry.Workspace,
					convID:    entry.ConversationID,
					display:   text,
					isPrompt:  entry.Type == "" && strings.TrimSpace(text) != "",
					at:        time.UnixMilli(entry.Timestamp),
				}
				hasRecord = true
			}
		}
		if readErr == io.EOF {
			if hasRecord {
				recCopy := parsed
				r.agy.histTail = &recCopy
			}
			break
		}
		if hasRecord {
			r.agy.histRecords = append(r.agy.histRecords, parsed)
		}
		r.agy.histOffset += int64(len(raw))
		r.agy.histLineNo++
	}
	r.agy.histInfo = fInfo
	return nil
}

func agyTranscriptFullPath(agyDir, id string) string {
	return filepath.Join(agyDir, "brain", id, ".system_generated", "logs", "transcript_full.jsonl")
}

func agyTranscriptCompactPath(agyDir, id string) string {
	return filepath.Join(agyDir, "brain", id, ".system_generated", "logs", "transcript.jsonl")
}

func statAgyTranscript(agyDir, id string) (string, fs.FileInfo, bool, error) {
	for _, path := range []string{agyTranscriptFullPath(agyDir, id), agyTranscriptCompactPath(agyDir, id)} {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", nil, false, err
		}
		if info.Mode().IsRegular() {
			return path, info, true, nil
		}
	}
	return "", nil, false, nil
}

type unlinkedMatch struct {
	prompt agyHistoryRecord
	convID string
}

func (r *Reader) matchUnlinkedPromptsLocked(agyDir string, prompts []agyHistoryRecord, alreadyMatched func(agyHistoryRecord) bool) ([]unlinkedMatch, error) {
	brainDir := filepath.Join(agyDir, "brain")
	entries, err := os.ReadDir(brainDir)
	if errors.Is(err, fs.ErrNotExist) {
		clear(r.agy.firstSteps)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	present := make(map[string]bool, len(entries))
	var matched []unlinkedMatch
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		id := de.Name()
		present[id] = true
		first, err := r.readAgyFirstStepLocked(agyDir, id)
		if err != nil {
			return nil, err
		}
		if !first.valid {
			continue
		}
		for _, p := range prompts {
			if p.convID == "" && !alreadyMatched(p) && p.display == first.text && p.at.Sub(first.at).Abs() <= agyFirstPromptWindow {
				matched = append(matched, unlinkedMatch{prompt: p, convID: id})
				break
			}
		}
	}
	for id := range r.agy.firstSteps {
		if !present[id] {
			delete(r.agy.firstSteps, id)
		}
	}
	return matched, nil
}

func (r *Reader) readAgyFirstStepLocked(agyDir, id string) (agyFirstStep, error) {
	path, info, ok, err := statAgyTranscript(agyDir, id)
	if err != nil || !ok {
		delete(r.agy.firstSteps, id)
		return agyFirstStep{}, err
	}
	if cached, found := r.agy.firstSteps[id]; found && cached.info != nil && os.SameFile(cached.info, info) && cached.valid {
		return cached, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		delete(r.agy.firstSteps, id)
		return agyFirstStep{}, nil
	}
	if err != nil {
		return agyFirstStep{}, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 16<<10)
	raw, readErr := reader.ReadBytes('\n')
	if readErr != nil && readErr != io.EOF {
		return agyFirstStep{}, fmt.Errorf("%s: %w", path, readErr)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return agyFirstStep{}, nil
	}
	var step agyStepLine
	if err := json.Unmarshal(trimmed, &step); err != nil {
		if readErr == io.EOF {
			return agyFirstStep{}, nil // still being written
		}
		return agyFirstStep{}, fmt.Errorf("%s:1: %w", path, err)
	}
	if step.Source != "USER_EXPLICIT" || step.Type != "USER_INPUT" {
		res := agyFirstStep{info: info}
		r.agy.firstSteps[id] = res
		return res, nil
	}
	res := agyFirstStep{
		info:  info,
		text:  extractUserRequest(step.Content),
		at:    step.CreatedAt,
		valid: true,
	}
	r.agy.firstSteps[id] = res
	return res, nil
}

func (r *Reader) updateAgyTranscriptLocked(path string, info fs.FileInfo) (*agyTranscript, error) {
	t := r.agy.convs[path]
	if t == nil || t.info == nil || !os.SameFile(t.info, info) || info.Size() < t.info.Size() ||
		(info.Size() == t.info.Size() && !info.ModTime().Equal(t.info.ModTime())) {
		t = &agyTranscript{lineNo: 1}
		r.agy.convs[path] = t
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
		t = &agyTranscript{lineNo: 1}
		r.agy.convs[path] = t
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, err
	}

	t.tailEntries = nil
	reader := bufio.NewReaderSize(f, 64<<10)
	for {
		raw, readErr := reader.ReadBytes('\n')
		if readErr != nil && readErr != io.EOF {
			delete(r.agy.convs, path)
			return nil, fmt.Errorf("%s: %w", path, readErr)
		}
		if len(raw) == 0 && readErr == io.EOF {
			break
		}
		var parsed Entry
		var hasEntry bool
		if slices.ContainsFunc(agyMarkers, func(m []byte) bool { return bytes.Contains(raw, m) }) {
			trimmed := bytes.TrimRight(raw, "\r\n")
			var step agyStepLine
			if err := json.Unmarshal(trimmed, &step); err != nil {
				if readErr == io.EOF {
					break // partial line mid-write at EOF
				}
				lineNo := t.lineNo
				delete(r.agy.convs, path)
				return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			switch {
			case step.Source == "USER_EXPLICIT" && step.Type == "USER_INPUT":
				if text := extractUserRequest(step.Content); strings.TrimSpace(text) != "" {
					parsed = Entry{Time: step.CreatedAt, Text: text, Kind: KindPrompt}
					hasEntry = true
				}
			case step.Source == "MODEL" && step.Type == "PLANNER_RESPONSE":
				if text := strings.TrimRight(step.Content, "\n"); strings.TrimSpace(text) != "" {
					parsed = Entry{Time: step.CreatedAt, Text: text, Kind: KindResponse}
					hasEntry = true
				}
			}
		}
		if readErr == io.EOF {
			if hasEntry {
				t.tailEntries = append(t.tailEntries, parsed)
			}
			break
		}
		if hasEntry {
			t.entries = append(t.entries, parsed)
		}
		t.offset += int64(len(raw))
		t.lineNo++
	}
	t.info = fInfo
	return t, nil
}

func (r *Reader) readAgyTitleLocked(path string) (string, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		delete(r.agy.titles, path)
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if cached, ok := r.agy.titles[path]; ok && cached.info != nil && os.SameFile(cached.info, info) &&
		cached.info.Size() == info.Size() && cached.info.ModTime().Equal(info.ModTime()) {
		return cached.title, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		delete(r.agy.titles, path)
		return "", nil
	}
	if err != nil {
		return "", err
	}
	title := parseAnnotationTitle(data)
	r.agy.titles[path] = agyTitleCache{info: info, title: title}
	return title, nil
}

// extractUserRequest unwraps Antigravity CLI's <USER_REQUEST>...</USER_REQUEST>
// envelope from a USER_INPUT step, ignoring trailing metadata blocks.
func extractUserRequest(content string) string {
	const openTag = "<USER_REQUEST>"
	const closeTag = "</USER_REQUEST>"
	if i := strings.Index(content, openTag); i >= 0 {
		rest := content[i+len(openTag):]
		if j := strings.LastIndex(rest, closeTag); j >= 0 {
			body := rest[:j]
			body = strings.TrimPrefix(body, "\r\n")
			body = strings.TrimPrefix(body, "\n")
			body = strings.TrimSuffix(body, "\n")
			body = strings.TrimSuffix(body, "\r")
			return body
		}
	}
	return strings.TrimRight(content, "\r\n")
}

// parseAnnotationTitle reads `title:"..."` from an Antigravity CLI .pbtxt file.
func parseAnnotationTitle(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		val, ok := strings.CutPrefix(line, "title:")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		if unquoted, err := strconv.Unquote(val); err == nil {
			return unquoted
		}
		return strings.Trim(val, `"`)
	}
	return ""
}

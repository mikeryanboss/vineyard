package subagent

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

// Claude Code keeps a transcript of each conversation, <dir>/<id>.jsonl, and
// one of each subagent the conversation runs,
// <dir>/<id>/subagents/agent-<agent>.jsonl, next to agent-<agent>.meta.json.
// <dir> is named after the working directory (recap.Dir). A transcript is one
// JSON object per line, appended as the conversation goes. None of this is
// documented; it was read off Claude Code 2.1.296.

// Status says whether a subagent is still at work.
type Status int

const (
	// Running is a subagent at work, or waiting on work of its own.
	Running Status = iota
	// Done is a subagent that completed and reported back.
	Done
	// Stopped is a subagent that stopped without completing: it failed, the
	// user stopped it, or the Claude Code it ran in exited.
	Stopped
)

// Agent is one subagent, as the transcripts record it.
type Agent struct {
	ID          string
	Type        string
	Description string
	// Parent is the ID of the subagent that started it, or "" for one the
	// conversation started.
	Parent string
	// Depth is 0 for a subagent the conversation started, 1 for one that
	// subagent started, and so on.
	Depth  int
	Status Status
	// Started is when it got its prompt. Ended is when Claude Code last
	// reported it stopped, while it is not running.
	Started, Ended time.Time
	// Prompt is what it was asked, and Report what it last said: its report
	// back to its caller, or else its last message.
	Prompt, Report string
	// Edited are the files it edited or wrote, oldest first.
	Edited []string
	// Dir is the directory it last worked in: that of the file it last
	// edited, else the one it last changed to in a shell command, else its
	// working directory.
	Dir string
}

// Conversation is what the transcripts say about a conversation's subagents.
type Conversation struct {
	// ID is Claude Code's ID for the conversation, its transcript's name,
	// which `claude --resume` takes.
	ID string
	// Agents are every subagent the conversation ran, each followed by those
	// it started, in the order they started.
	Agents []Agent
	// Prompted is when the user last sent the conversation a prompt.
	Prompted time.Time
}

// Kept returns the subagents worth showing: those running, those that
// stopped since the user's last prompt, and the subagents that started them.
func (c Conversation) Kept() []Agent {
	keep := map[string]bool{}
	for _, a := range slices.Backward(c.Agents) { // children come after parents
		if keep[a.ID] || a.Status == Running || a.Ended.After(c.Prompted) {
			keep[a.ID] = true
			keep[a.Parent] = true
		}
	}
	var kept []Agent
	for _, a := range c.Agents {
		if keep[a.ID] {
			kept = append(kept, a)
		}
	}
	return kept
}

// EventKind says what an Event records.
type EventKind int

const (
	// Prompt is a message the subagent was sent: its task, or a later one.
	Prompt EventKind = iota
	// Text is a message the subagent wrote.
	Text
	// Tool is a tool the subagent called, Text summarising the call.
	Tool
	// Result is what a tool call returned, cut to its first lines.
	Result
)

// Event is one step of a subagent's transcript.
type Event struct {
	Kind EventKind
	// Name is the tool's, for a Tool event.
	Name string
	Text string
	// Error marks a Result that is an error.
	Error bool
}

// resultLines is how many lines of a tool result an Event keeps.
const resultLines = 3

// Reader reads transcripts, each from where it last stopped: a conversation's
// grows long, and a poll needs only what was appended. It is safe for
// concurrent use.
type Reader struct {
	mu    sync.Mutex
	files map[string]*transcript
}

// NewReader returns a Reader that has read nothing yet.
func NewReader() *Reader { return &Reader{files: map[string]*transcript{}} }

// Load reads the subagents of the conversation last written to in dir, a
// directory of transcripts. It finds none when dir holds no transcript.
//
// started is when the Claude Code that writes to dir started. Subagents run
// inside Claude Code's process and end with it, unreported, so one whose
// transcript was not written to since then ran in an earlier Claude Code and
// counts as stopped, at its last message. The conversation's own transcript
// cannot tell: a Claude Code that resumes the conversation touches it as it
// starts. A Claude Code that exits, as when its session is paused, leaves its
// conversation the newest in dir until its successor is sent a prompt.
func (r *Reader) Load(dir string, started time.Time) (Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	main, err := newest(filepath.Join(dir, "*.jsonl"))
	if main == "" || err != nil {
		return Conversation{}, err
	}
	conv, err := r.update(main, false)
	if err != nil {
		return Conversation{}, err
	}
	paths, err := filepath.Glob(filepath.Join(subagentDir(main), "agent-*.jsonl"))
	if err != nil {
		return Conversation{}, err
	}
	stops := map[string]stop{}
	note := func(t *transcript) {
		for id, s := range t.stops {
			if s.at.After(stops[id].at) {
				stops[id] = s
			}
		}
	}
	note(conv)
	launcher := map[string]string{} // Agent tool use ID -> the subagent that made it
	var read []*transcript
	var agents []Agent
	for _, path := range paths {
		t, err := r.update(path, true)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the glob, or its meta file not written yet
		}
		if err != nil {
			return Conversation{}, err
		}
		note(t)
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "agent-"), ".jsonl")
		for _, use := range t.launched {
			launcher[use] = id
		}
		read = append(read, t)
		agents = append(agents, t.agent(id))
	}
	for i, t := range read {
		a := &agents[i]
		a.Parent = launcher[t.meta.ToolUseID]
		// A resumed subagent writes on after the report that it stopped.
		if s, ok := stops[a.ID]; ok && !s.at.Before(t.last) {
			a.Ended = s.at
			a.Status = Stopped
			if s.status == "completed" {
				a.Status = Done
			}
		}
		// Claude Code reports none of the subagents it stops when the user
		// stops them all.
		if a.Status == Running && !conv.killed.IsZero() && !conv.killed.Before(t.last) {
			a.Status, a.Ended = Stopped, conv.killed
		}
		if a.Status == Running && t.written.Before(started) {
			a.Status, a.Ended = Stopped, t.last
		}
	}
	id := strings.TrimSuffix(filepath.Base(main), ".jsonl")
	return Conversation{ID: id, Agents: tree(agents), Prompted: conv.prompted}, nil
}

// Events returns the condensed transcript of subagent id of the conversation
// last written to in dir.
func (r *Reader) Events(dir, id string) ([]Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	main, err := newest(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	if main == "" {
		return nil, fmt.Errorf("no conversation in %s", dir)
	}
	t, err := r.update(agentPath(main, id), true)
	if err != nil {
		return nil, err
	}
	return slices.Clone(t.events), nil
}

func subagentDir(main string) string {
	return filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents")
}

func agentPath(main, id string) string {
	return filepath.Join(subagentDir(main), "agent-"+id+".jsonl")
}

// newest returns the most recently written file matching pattern, or "".
func newest(pattern string) (string, error) {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	var found string
	var at time.Time
	for _, path := range paths {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the glob
		}
		if err != nil {
			return "", err
		}
		if info.ModTime().After(at) {
			found, at = path, info.ModTime()
		}
	}
	return found, nil
}

// tree orders agents so each is followed by those it started, siblings in
// the order they started, and sets their depths.
func tree(agents []Agent) []Agent {
	children := map[string][]Agent{}
	known := map[string]bool{}
	for _, a := range agents {
		known[a.ID] = true
	}
	for _, a := range agents {
		if !known[a.Parent] {
			a.Parent = "" // started by the conversation, or by a subagent not on record
		}
		children[a.Parent] = append(children[a.Parent], a)
	}
	var ordered []Agent
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		kids := children[parent]
		slices.SortStableFunc(kids, func(a, b Agent) int { return a.Started.Compare(b.Started) })
		for _, a := range kids {
			a.Depth = depth
			ordered = append(ordered, a)
			walk(a.ID, depth+1)
		}
	}
	walk("", 0)
	return ordered
}

// meta is a subagent's meta file.
type meta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	// ToolUseID is the ID of the Agent tool use that started it.
	ToolUseID string `json:"toolUseId"`
}

// stop is Claude Code's report that a task stopped.
type stop struct {
	at     time.Time
	status string
}

// transcript is what Reader has gathered from one transcript file.
type transcript struct {
	read    int64     // bytes read, up to the end of the last whole line
	written time.Time // when the file was last written
	// prompted is when the user last sent a prompt.
	prompted time.Time
	// stops are the last reports of tasks that stopped, by task ID.
	stops map[string]stop
	// killed is when Claude Code last stopped all background agents.
	killed time.Time
	// launched are the IDs of the Agent tool uses that started subagents.
	launched []string

	// The rest describes a subagent's own transcript.
	meta meta
	// started is the time of its first message, and last that of the last
	// that shows it at work: one it wrote, or one it was sent. Tool results
	// and interruption markers do not count: Claude Code writes them as it
	// stops a subagent, after it reports the stop.
	started, last       time.Time
	prompt, report, cwd string
	cd                  string
	edited              []string
	events              []Event
}

// update reads what was appended to the transcript at path since the last
// update. Only a subagent's transcript is read in full; of a conversation's,
// only the lines with prompts and notifications.
func (r *Reader) update(path string, subagent bool) (*transcript, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	t := r.files[path]
	if t == nil || info.Size() < t.read { // new, or rewritten
		t = &transcript{stops: map[string]stop{}}
		if subagent {
			data, err := os.ReadFile(strings.TrimSuffix(path, ".jsonl") + ".meta.json")
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &t.meta); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
		r.files[path] = t
	}
	t.written = info.ModTime()
	if info.Size() == t.read {
		return t, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(t.read, io.SeekStart); err != nil {
		return nil, err
	}
	lines := bufio.NewReaderSize(f, 1<<16)
	for {
		raw, err := lines.ReadBytes('\n')
		if err == io.EOF {
			return t, nil // a line still being written is read next time
		}
		if err != nil {
			return nil, err
		}
		if err := t.add(raw, subagent); err != nil {
			return nil, fmt.Errorf("%s at byte %d: %w", path, t.read, err)
		}
		t.read += int64(len(raw))
	}
}

// line holds the fields of a transcript line that Reader uses.
type line struct {
	Type      string          `json:"type"`
	Operation string          `json:"operation"`
	Subtype   string          `json:"subtype"`
	Content   json.RawMessage `json:"content"`
	Timestamp time.Time       `json:"timestamp"`
	Cwd       string          `json:"cwd"`
	IsMeta    bool            `json:"isMeta"`
	Origin    struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// block is one part of a message's content.
type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// Markers of the conversation lines Reader uses. Checking for them before
// decoding skips the bulk of a conversation, which is tool output.
var markers = [][]byte{[]byte(`"human"`), []byte("<task-notification>"), []byte(`"agents_killed"`)}

// add takes in one line of the transcript.
func (t *transcript) add(raw []byte, subagent bool) error {
	if !subagent && !slices.ContainsFunc(markers, func(m []byte) bool { return bytes.Contains(raw, m) }) {
		return nil
	}
	var l line
	if err := json.Unmarshal(raw, &l); err != nil {
		return err
	}
	switch {
	case l.Type == "queue-operation" && l.Operation == "enqueue":
		t.notice(text(l.Content), l.Timestamp)
	case l.Type == "user" && l.Origin.Kind == "task-notification":
		t.notice(text(l.Message.Content), l.Timestamp)
	case l.Type == "user" && l.Origin.Kind == "human":
		t.prompted = l.Timestamp
	case l.Type == "system" && l.Subtype == "agents_killed":
		t.killed = l.Timestamp
	}
	if !subagent || (l.Type != "user" && l.Type != "assistant") {
		return nil
	}
	if t.started.IsZero() {
		t.started = l.Timestamp
	}
	if l.Type == "assistant" {
		t.last = l.Timestamp
	}
	if l.Cwd != "" {
		t.cwd = l.Cwd
	}
	// Claude Code's own reminders are meta messages from no one; messages
	// the subagent's caller sends while it works are meta too, but from
	// someone.
	sent := l.Type == "user" && (!l.IsMeta || l.Origin.Kind != "")
	var blocks []block
	if err := json.Unmarshal(l.Message.Content, &blocks); err != nil {
		if s := text(l.Message.Content); sent && s != "" { // a plain string
			t.message(s, l.Timestamp)
		}
		return nil
	}
	// A fork's transcript starts with a copy of the call that started it,
	// whose prompt is the fork's task, and of its result, next to
	// instructions for forks in general.
	if slices.ContainsFunc(blocks, func(b block) bool { return b.ToolUseID != "" && b.ToolUseID == t.meta.ToolUseID }) {
		return nil
	}
	for _, b := range blocks {
		switch {
		case b.Type == "tool_use" && b.ID == t.meta.ToolUseID:
			var call struct {
				Prompt string `json:"prompt"`
			}
			_ = json.Unmarshal(b.Input, &call) // a call without a prompt gives none
			t.message(call.Prompt, l.Timestamp)
		case b.Type == "text" && l.Type == "user":
			if sent {
				t.message(b.Text, l.Timestamp)
			}
		case b.Type == "text":
			t.events = append(t.events, Event{Kind: Text, Text: b.Text})
			t.report = b.Text
		case b.Type == "tool_use":
			t.use(b)
		case b.Type == "tool_result":
			t.events = append(t.events, Event{Kind: Result, Text: firstLines(text(b.Content), resultLines), Error: b.IsError})
		}
	}
	return nil
}

// interrupted starts the messages Claude Code adds to a subagent's transcript
// when it interrupts the subagent.
const interrupted = "[Request interrupted by user"

// message takes in a message the subagent was sent at time at.
func (t *transcript) message(s string, at time.Time) {
	if strings.HasPrefix(s, interrupted) {
		return
	}
	t.last = at
	if t.prompt == "" {
		t.prompt = s
	}
	t.events = append(t.events, Event{Kind: Prompt, Text: s})
}

// cdTo matches a shell command that starts by changing to an absolute
// directory.
var cdTo = regexp.MustCompile(`^\s*cd\s+(?:"(/[^"]+)"|'(/[^']+)'|(/[^\s;&|]+))`)

// use takes in a tool call.
func (t *transcript) use(b block) {
	var input map[string]any
	_ = json.Unmarshal(b.Input, &input) // a call without an object input has nothing to summarise
	field := func(name string) string { s, _ := input[name].(string); return s }
	switch b.Name {
	case "Edit", "Write", "NotebookEdit":
		path := field("file_path")
		if path == "" {
			path = field("notebook_path")
		}
		if path != "" {
			t.edited = append(t.edited, path)
		}
	case "Bash":
		if m := cdTo.FindStringSubmatch(field("command")); m != nil {
			t.cd = m[1] + m[2] + m[3]
		}
	case "Agent", "Task":
		t.launched = append(t.launched, b.ID)
	case "SubagentHandback":
		t.report = field("message")
	}
	summary := ""
	for _, name := range []string{"command", "file_path", "notebook_path", "pattern", "description", "url", "query", "to", "message", "prompt"} {
		if summary = field(name); summary != "" {
			break
		}
	}
	t.events = append(t.events, Event{Kind: Tool, Name: b.Name, Text: firstLines(summary, 1)})
}

var (
	taskID     = regexp.MustCompile(`<task-id>([^<]*)</task-id>`)
	taskStatus = regexp.MustCompile(`<status>([^<]*)</status>`)
)

// notice records a task notification, which Claude Code sends when a
// subagent or another background task stops.
func (t *transcript) notice(s string, at time.Time) {
	id, status := taskID.FindStringSubmatch(s), taskStatus.FindStringSubmatch(s)
	if id == nil || status == nil || !strings.Contains(s, "<task-notification>") {
		return
	}
	if at.After(t.stops[id[1]].at) {
		t.stops[id[1]] = stop{at: at, status: status[1]}
	}
}

// agent describes the subagent this transcript records.
func (t *transcript) agent(id string) Agent {
	dir := t.cwd
	if t.cd != "" {
		dir = t.cd
	}
	if n := len(t.edited); n > 0 {
		dir = filepath.Dir(t.edited[n-1])
	}
	return Agent{
		ID:          id,
		Type:        t.meta.AgentType,
		Description: t.meta.Description,
		Started:     t.started,
		Prompt:      t.prompt,
		Report:      t.report,
		Edited:      slices.Clone(t.edited),
		Dir:         dir,
	}
}

// text returns content that is either a string or a list of blocks, of
// which it joins the text.
func text(content json.RawMessage) string {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	var blocks []block
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// firstLines returns the first n lines of s, noting how many it left out.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n… +%d lines", len(lines)-n)
}

// listedStopped is how long Claude Code keeps listing a subagent that failed
// or was stopped (https://code.claude.com/docs/en/sub-agents.md).
const listedStopped = 30 * time.Second

// Match reads Claude Code's list l in terms of agents, all the subagents of
// the conversation as Conversation holds them: it returns the ID of the
// subagent at each position of the list, position 1 at index 0. Claude Code
// lists the subagents the conversation started that run, or that failed in
// the last 30 seconds, in the order they started. It also lists the one
// whose view it shows when that one has finished, titling the view with its
// description, and under that one the subagents it started that run. Match
// reports false when no reading fits the list, or when readings differ.
func Match(agents []Agent, l List, now time.Time) ([]string, bool) {
	listed := func(a Agent) bool {
		return a.Status == Running || a.Status == Stopped && now.Sub(a.Ended) < listedStopped
	}
	var match []string
	found := false
	for _, shown := range append([]Agent{{}}, agents...) {
		if shown.Depth != 0 {
			continue // Claude Code's list for a nested view is not known
		}
		if shown.ID != "" && l.View != "" && !titles(l.View, shown.Description) {
			continue // the view is another's
		}
		var ids []string
		var depths []int
		for _, a := range agents {
			if a.Depth == 0 && (listed(a) || a.ID == shown.ID) ||
				a.Depth == 1 && shown.ID != "" && a.Parent == shown.ID && listed(a) {
				ids = append(ids, a.ID)
				depths = append(depths, a.Depth)
			}
		}
		if len(ids) != l.Len() {
			continue
		}
		position := slices.Index(ids, shown.ID) + 1 // 0, the conversation, for no subagent
		drawn := position > l.Above && position <= l.Above+len(l.Rows)
		if l.Shown != position && (l.Shown != -1 || position == 0 || drawn) {
			continue
		}
		if !slices.EqualFunc(l.Rows, depths[l.Above:l.Above+len(l.Rows)], func(r Row, depth int) bool { return r.Depth == depth }) {
			continue
		}
		if found && !slices.Equal(match, ids) {
			return nil, false
		}
		match, found = ids, true
	}
	return match, found
}

// titles reports whether view, a title Claude Code may have cut short, is
// description.
func titles(view, description string) bool {
	cut, short := strings.CutSuffix(view, "…")
	return view == description || short && strings.HasPrefix(description, cut)
}

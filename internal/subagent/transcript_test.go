package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Transcript lines shaped like those Claude Code 2.1.296 writes. at counts
// seconds from start.

var start = time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)

func at(second int) time.Time { return start.Add(time.Duration(second) * time.Second) }

// jsonLine encodes v as Claude Code does, leaving < and > as they are.
func jsonLine(t *testing.T, v map[string]any) string {
	t.Helper()
	var b strings.Builder
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// human is a prompt the user sent the conversation.
func human(t *testing.T, second int, text string) string {
	return jsonLine(t, map[string]any{"type": "user", "origin": map[string]any{"kind": "human"},
		"message": map[string]any{"role": "user", "content": text}, "timestamp": at(second)})
}

// stopped is Claude Code queueing the notification that task id stopped.
func stopped(t *testing.T, second int, id, status string) string {
	return jsonLine(t, map[string]any{"type": "queue-operation", "operation": "enqueue", "timestamp": at(second),
		"content": fmt.Sprintf("<task-notification>\n<task-id>%s</task-id>\n<status>%s</status>\n<summary>Agent finished</summary>\n</task-notification>", id, status)})
}

// asked is a message a subagent was sent: its prompt, or with from set, one
// its caller sent later.
func asked(t *testing.T, second int, cwd, from, text string) string {
	v := map[string]any{"type": "user", "isSidechain": true, "cwd": cwd,
		"message": map[string]any{"role": "user", "content": text}, "timestamp": at(second)}
	if from != "" {
		v["isMeta"], v["origin"] = true, map[string]any{"kind": from}
	}
	return jsonLine(t, v)
}

// reminder is Claude Code reminding a subagent of something.
func reminder(t *testing.T, second int) string {
	return jsonLine(t, map[string]any{"type": "user", "isMeta": true,
		"message": map[string]any{"role": "user", "content": "<system-reminder>Report back.</system-reminder>"}, "timestamp": at(second)})
}

func said(t *testing.T, second int, text string) string {
	return jsonLine(t, map[string]any{"type": "assistant", "timestamp": at(second),
		"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}}}})
}

func called(t *testing.T, second int, id, name string, input map[string]any) string {
	return jsonLine(t, map[string]any{"type": "assistant", "timestamp": at(second),
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}}}})
}

func returned(t *testing.T, second int, id, text string, isError bool) string {
	return jsonLine(t, map[string]any{"type": "user", "timestamp": at(second),
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": id, "content": text, "is_error": isError}}}})
}

// transcripts is a directory of transcripts holding one conversation.
type transcripts struct {
	t   *testing.T
	dir string
}

func newTranscripts(t *testing.T) transcripts {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "conv", "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	return transcripts{t, dir}
}

func (c transcripts) appendTo(path string, lines ...string) {
	c.t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		c.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "")); err != nil {
		c.t.Fatal(err)
	}
}

// main appends to the conversation's transcript.
func (c transcripts) main(lines ...string) { c.appendTo(filepath.Join(c.dir, "conv.jsonl"), lines...) }

// agent appends to subagent id's transcript, writing its meta file first.
func (c transcripts) agent(id, description, toolUseID string, lines ...string) {
	c.t.Helper()
	path := filepath.Join(c.dir, "conv", "subagents", "agent-"+id)
	meta := fmt.Sprintf(`{"agentType":"general-purpose","description":%q,"toolUseId":%q,"spawnDepth":1}`, description, toolUseID)
	if err := os.WriteFile(path+".meta.json", []byte(meta), 0o644); err != nil {
		c.t.Fatal(err)
	}
	c.appendTo(path+".jsonl", lines...)
}

// load reads dir for a Claude Code that started at started.
func load(t *testing.T, r *Reader, dir string, started time.Time) Conversation {
	t.Helper()
	c, err := r.Load(dir, started)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func byID(c Conversation) map[string]Agent {
	agents := map[string]Agent{}
	for _, a := range c.Agents {
		agents[a.ID] = a
	}
	return agents
}

func TestLoad_ReadsSubagentsFromTheTranscripts(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Review the parser"), stopped(t, 30, "done1", "completed"), stopped(t, 40, "gone3", "killed"))
	c.agent("done1", "review parser", "toolu_1",
		asked(t, 1, "/repo", "", "Review the parser."),
		reminder(t, 1),
		said(t, 10, "Looking at it."),
		called(t, 29, "toolu_h", "SubagentHandback", map[string]any{"message": "The parser is fine."}),
		returned(t, 29, "toolu_h", `{"success":true}`, false))
	c.agent("busy2", "write tests", "toolu_2",
		asked(t, 2, "/repo", "", "Write the tests."),
		called(t, 3, "toolu_cd", "Bash", map[string]any{"command": "cd /trees/tests && go test ./..."}),
		called(t, 4, "toolu_e", "Edit", map[string]any{"file_path": "/trees/tests/parser_test.go"}),
		called(t, 5, "toolu_4", "Agent", map[string]any{"description": "helper"}))
	c.agent("gone3", "update docs", "toolu_3",
		asked(t, 3, "/repo", "", "Update the docs."),
		called(t, 6, "toolu_cd2", "Bash", map[string]any{"command": `cd "/trees/docs" && make`}))
	c.agent("help4", "helper", "toolu_4", asked(t, 6, "/repo", "", "Help."))

	conv := load(t, NewReader(), c.dir, time.Time{})
	var order []string
	for _, a := range conv.Agents {
		order = append(order, strings.Repeat("  ", a.Depth)+a.ID)
	}
	if want := []string{"done1", "busy2", "  help4", "gone3"}; !slices.Equal(order, want) {
		t.Errorf("order = %q, want each subagent followed by those it started: %q", order, want)
	}
	if !conv.Prompted.Equal(at(0)) {
		t.Errorf("Prompted = %v, want the user's prompt at %v", conv.Prompted, at(0))
	}
	agents := byID(conv)
	tests := []struct {
		id     string
		status Status
		ended  time.Time
		dir    string
	}{
		{"done1", Done, at(30), "/repo"},
		{"busy2", Running, time.Time{}, "/trees/tests"},
		{"gone3", Stopped, at(40), "/trees/docs"},
		{"help4", Running, time.Time{}, "/repo"},
	}
	for _, tt := range tests {
		a := agents[tt.id]
		if a.Status != tt.status || !a.Ended.Equal(tt.ended) || a.Dir != tt.dir {
			t.Errorf("%s: status %d, ended %v, dir %q; want %d, %v, %q", tt.id, a.Status, a.Ended, a.Dir, tt.status, tt.ended, tt.dir)
		}
	}
	if a := agents["done1"]; a.Description != "review parser" || a.Prompt != "Review the parser." || a.Report != "The parser is fine." {
		t.Errorf("done1 = %q, prompt %q, report %q", a.Description, a.Prompt, a.Report)
	}
	if a := agents["busy2"]; !slices.Equal(a.Edited, []string{"/trees/tests/parser_test.go"}) || agents["help4"].Parent != "busy2" {
		t.Errorf("busy2 edited %q; help4's parent %q", a.Edited, agents["help4"].Parent)
	}
}

func TestLoad_ReadsOnlyWhatWasAppended(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Go"), stopped(t, 20, "a1", "completed"))
	c.agent("a1", "one", "toolu_1", asked(t, 1, "/repo", "", "Do it."), said(t, 19, "Done."))
	r := NewReader()
	if a := load(t, r, c.dir, time.Time{}).Agents[0]; a.Status != Done {
		t.Fatalf("status = %d, want done", a.Status)
	}

	// Its caller resumes it, and the user is half way through a line.
	c.agent("a1", "one", "toolu_1", asked(t, 25, "/repo", "coordinator", "One more thing."))
	partial := human(t, 26, "Next")
	c.main(partial[:20])
	conv := load(t, r, c.dir, time.Time{})
	if conv.Agents[0].Status != Running || !conv.Prompted.Equal(at(0)) {
		t.Errorf("status %d, prompted %v; want a resumed subagent running, and a half-written line left for later", conv.Agents[0].Status, conv.Prompted)
	}
	c.main(partial[20:])
	if conv := load(t, r, c.dir, time.Time{}); !conv.Prompted.Equal(at(26)) {
		t.Errorf("prompted %v, want the line once complete, at %v", conv.Prompted, at(26))
	}

	// Lines already read are not read again: garbling them changes nothing.
	path := filepath.Join(c.dir, "conv", "subagents", "agent-a1.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", len(data)-1)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Load(c.dir, time.Time{}); err != nil {
		t.Errorf("Load reread lines it had read: %v", err)
	}
	// A file that shrank was rewritten, and is read anew.
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Load(c.dir, time.Time{}); err == nil {
		t.Error("Load should have read the rewritten file anew and failed on its garbage")
	}
}

func TestLoad_ForkStartsWithACopyOfItsOwnCall(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Go"))
	c.agent("fork1", "fix it", "toolu_f",
		called(t, 1, "toolu_f", "Agent", map[string]any{"description": "fix it", "prompt": "Fix the flicker."}),
		jsonLine(t, map[string]any{"type": "user", "timestamp": at(1), "message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_f", "content": "Fork started"},
			map[string]any{"type": "text", "text": "<fork-boilerplate>You are a fork.</fork-boilerplate>"}}}}),
		said(t, 2, "On it."))
	r := NewReader()
	conv := load(t, r, c.dir, time.Time{})
	if len(conv.Agents) != 1 || conv.Agents[0].Parent != "" || conv.Agents[0].Prompt != "Fix the flicker." {
		t.Fatalf("agents = %+v, want the fork, started by the conversation, with its call's prompt", conv.Agents)
	}
	events, err := r.Events(c.dir, "fork1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []Event{{Kind: Prompt, Text: "Fix the flicker."}, {Kind: Text, Text: "On it."}}; !slices.Equal(events, want) {
		t.Errorf("events = %+v, want %+v", events, want)
	}
}

func TestLoad_IDIsTheConversationWrittenLast(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Go"))
	r := NewReader()
	if id := load(t, r, c.dir, time.Time{}).ID; id != "conv" {
		t.Errorf("ID = %q, want conv", id)
	}
	// /clear starts a new conversation, which a restart must resume.
	cleared := filepath.Join(c.dir, "cleared.jsonl")
	c.appendTo(cleared, human(t, 50, "Next task"))
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(cleared, later, later); err != nil {
		t.Fatal(err)
	}
	if id := load(t, r, c.dir, time.Time{}).ID; id != "cleared" {
		t.Errorf("ID = %q, want cleared", id)
	}
}

func TestLoad_NoTranscript(t *testing.T) {
	conv, err := NewReader().Load(filepath.Join(t.TempDir(), "none"), time.Time{})
	if err != nil || len(conv.Agents) != 0 {
		t.Errorf("Load = %+v, %v; want nothing", conv, err)
	}
}

func TestEvents_CondenseTheTranscript(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Go"))
	c.agent("a1", "one", "toolu_1",
		asked(t, 1, "/repo", "", "Run the tests."),
		reminder(t, 1),
		called(t, 2, "toolu_b", "Bash", map[string]any{"command": "go test ./...\necho done", "description": "Run tests"}),
		returned(t, 3, "toolu_b", "ok 1\nok 2\nok 3\nok 4\nok 5", false),
		called(t, 4, "toolu_r", "Read", map[string]any{"file_path": "/repo/missing.go"}),
		returned(t, 4, "toolu_r", "no such file", true),
		asked(t, 5, "/repo", "coordinator", "Hurry up."),
		said(t, 6, "All pass."))
	events, err := NewReader().Events(c.dir, "a1")
	if err != nil {
		t.Fatal(err)
	}
	want := []Event{
		{Kind: Prompt, Text: "Run the tests."},
		{Kind: Tool, Name: "Bash", Text: "go test ./...\n… +1 lines"},
		{Kind: Result, Text: "ok 1\nok 2\nok 3\n… +2 lines"},
		{Kind: Tool, Name: "Read", Text: "/repo/missing.go"},
		{Kind: Result, Text: "no such file", Error: true},
		{Kind: Prompt, Text: "Hurry up."},
		{Kind: Text, Text: "All pass."},
	}
	if !slices.Equal(events, want) {
		t.Errorf("events =\n%+v\nwant\n%+v", events, want)
	}
}

func TestConversation_KeptUntilTheNextPrompt(t *testing.T) {
	c := Conversation{Prompted: at(10), Agents: []Agent{
		{ID: "before", Status: Done, Ended: at(5)},
		{ID: "after", Status: Done, Ended: at(15)},
		{ID: "failed", Status: Stopped, Ended: at(5)},
		{ID: "parent", Status: Done, Ended: at(5)},
		{ID: "child", Parent: "parent", Depth: 1, Status: Running},
		{ID: "running"},
	}}
	var kept []string
	for _, a := range c.Kept() {
		kept = append(kept, a.ID)
	}
	if want := []string{"after", "parent", "child", "running"}; !slices.Equal(kept, want) {
		t.Errorf("kept %q, want %q: stopped since the prompt, running, and the parents of those", kept, want)
	}
}

func TestMatch_ReadsClaudeCodesList(t *testing.T) {
	now := at(100)
	agents := []Agent{
		{ID: "a", Status: Done, Ended: at(50)},
		{ID: "b", Status: Running},
		{ID: "b1", Parent: "b", Depth: 1, Status: Running},
		{ID: "b2", Parent: "b", Depth: 1, Status: Done, Ended: at(90)},
		{ID: "c", Status: Stopped, Ended: at(80)},
		{ID: "d", Status: Running},
		{ID: "e", Status: Stopped, Ended: at(10)},
	}
	rows := func(depths ...int) []Row {
		var r []Row
		for _, d := range depths {
			r = append(r, Row{Type: "general-purpose", Depth: d})
		}
		return r
	}
	tests := []struct {
		name string
		list List
		want []string
	}{
		{"the conversation shown", List{Rows: rows(0, 0, 0), Shown: 0, Focus: -1}, []string{"b", "c", "d"}},
		{"a finished subagent kept in view", List{Rows: rows(0, 0, 0, 0), Shown: 1, Focus: -1}, []string{"a", "b", "c", "d"}},
		{"a running subagent in view, with those it started", List{Rows: rows(0, 1, 0, 0), Shown: 1, Focus: -1}, []string{"b", "b1", "c", "d"}},
		{"out of sight", List{Rows: rows(0, 0), Below: 1, Shown: 0, Focus: -1}, []string{"b", "c", "d"}},
		{"unreadable", List{Rows: rows(0, 0), Shown: 0, Focus: -1}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Match(agents, tt.list, now)
			if ok != (tt.want != nil) || !slices.Equal(got, tt.want) {
				t.Errorf("Match = %q, %v; want %q", got, ok, tt.want)
			}
		})
	}
}

// Any finished subagent could be the one Claude Code keeps in view; the
// view's title tells which.
func TestMatch_TheViewTitleTellsFinishedSubagentsApart(t *testing.T) {
	agents := []Agent{
		{ID: "x", Description: "review the parser", Status: Done, Ended: at(10)},
		{ID: "y", Description: "write the tests for the parser", Status: Done, Ended: at(20)},
		{ID: "z", Description: "update the docs", Status: Running},
	}
	l := List{Rows: []Row{{Type: "general-purpose"}, {Type: "general-purpose"}}, Shown: 1, Focus: -1}
	if ids, ok := Match(agents, l, at(30)); ok {
		t.Errorf("Match = %q without a title; want no reading, as x or y could be in view", ids)
	}
	l.View = "write the tests…"
	if ids, ok := Match(agents, l, at(30)); !ok || !slices.Equal(ids, []string{"y", "z"}) {
		t.Errorf("Match = %q, %v; want y, whose description the title starts, then z", ids, ok)
	}
}

// Subagents end with the Claude Code they run in, which reports nothing then.
// A conversation not written to since its session's Claude Code started was
// left by one that exited.
func TestLoad_SubagentsOfAnExitedClaudeCodeStopped(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Go"), stopped(t, 20, "done1", "completed"))
	c.agent("done1", "one", "toolu_1", asked(t, 1, "/repo", "", "Do it."), said(t, 19, "Done."))
	c.agent("busy2", "two", "toolu_2", asked(t, 2, "/repo", "", "Keep at it."), said(t, 30, "Still going."))
	paths, err := filepath.Glob(filepath.Join(c.dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join(c.dir, "conv", "subagents", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(paths, more...) {
		if err := os.Chtimes(path, at(30), at(30)); err != nil {
			t.Fatal(err)
		}
	}
	if a := byID(load(t, NewReader(), c.dir, at(25)))["busy2"]; a.Status != Running {
		t.Errorf("status = %d, want running in a conversation written to since its Claude Code started", a.Status)
	}
	agents := byID(load(t, NewReader(), c.dir, at(40)))
	if a := agents["busy2"]; a.Status != Stopped || !a.Ended.Equal(at(30)) {
		t.Errorf("busy2: status %d, ended %v; want stopped at its last message, %v", a.Status, a.Ended, at(30))
	}
	if a := agents["done1"]; a.Status != Done {
		t.Errorf("done1: status %d, want done as reported", a.Status)
	}

	// A Claude Code that resumes the conversation touches its transcript as it
	// starts, but not those of the subagents that ended with its predecessor.
	if err := os.Chtimes(filepath.Join(c.dir, "conv.jsonl"), at(45), at(45)); err != nil {
		t.Fatal(err)
	}
	if a := byID(load(t, NewReader(), c.dir, at(40)))["busy2"]; a.Status != Stopped {
		t.Errorf("status = %d, want stopped after its conversation was resumed", a.Status)
	}
}

// killed is Claude Code recording that it stopped all background agents, as
// when the user stops them.
func killed(t *testing.T, second int) string {
	return jsonLine(t, map[string]any{"type": "system", "subtype": "agents_killed", "isMeta": false, "timestamp": at(second)})
}

// rejected is what Claude Code writes to a subagent's transcript as it stops
// it during tool call id: the call's rejection and an interruption marker.
func rejected(t *testing.T, second int, id string) string {
	return returned(t, second, id, "The user doesn't want to proceed with this tool use.", true) +
		jsonLine(t, map[string]any{"type": "user", "isSidechain": true, "timestamp": at(second),
			"message": map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": "[Request interrupted by user for tool use]"}}}})
}

func TestLoad_SubagentsTheUserStoppedStopped(t *testing.T) {
	c := newTranscripts(t)
	c.main(human(t, 0, "Go"), stopped(t, 6, "resumed2", "completed"), killed(t, 20))
	// Claude Code writes to the subagents it stops after it records the stop.
	c.agent("busy1", "one", "toolu_1",
		asked(t, 1, "/repo", "", "Train it."),
		called(t, 2, "toolu_b1", "Bash", map[string]any{"command": "train"}),
		rejected(t, 21, "toolu_b1"))
	// A subagent that completed with work of its own running resumes when
	// that work reports.
	c.agent("resumed2", "two", "toolu_2",
		asked(t, 3, "/repo", "", "Plant it."),
		said(t, 5, "Planted; the run reports later."),
		called(t, 10, "toolu_b2", "Bash", map[string]any{"command": "plant"}),
		rejected(t, 21, "toolu_b2"))
	c.agent("later3", "three", "toolu_3", asked(t, 25, "/repo", "", "Start after the stop."))
	c.agent("messaged4", "four", "toolu_4",
		asked(t, 4, "/repo", "", "Wait."),
		said(t, 5, "Waiting."),
		asked(t, 22, "/repo", "coordinator", "Go on."))

	agents := byID(load(t, NewReader(), c.dir, time.Time{}))
	tests := []struct {
		id     string
		status Status
		ended  time.Time
	}{
		{"busy1", Stopped, at(20)},
		{"resumed2", Stopped, at(20)},
		{"later3", Running, time.Time{}},
		{"messaged4", Running, time.Time{}},
	}
	for _, tt := range tests {
		if a := agents[tt.id]; a.Status != tt.status || !a.Ended.Equal(tt.ended) {
			t.Errorf("%s: status %d, ended %v; want %d, %v", tt.id, a.Status, a.Ended, tt.status, tt.ended)
		}
	}

	events, err := NewReader().Events(c.dir, "busy1")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Kind == Prompt && e.Text != "Train it." {
			t.Errorf("prompt event %q, want only the task: the interruption marker is no message", e.Text)
		}
	}
}

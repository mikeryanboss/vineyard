package recap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTranscript(t *testing.T, dir, name string, modTime time.Time, lines ...string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func formatEntries(entries []Entry) []string {
	var got []string
	for _, e := range entries {
		kind := "user"
		switch e.Kind {
		case KindResponse:
			kind = "response"
		case KindRecap:
			kind = "recap"
		}
		got = append(got, kind+": "+e.Text)
	}
	return got
}

// The user's messages, final assistant responses, and away recaps of every
// transcript of a working directory are merged, oldest first, without Claude
// Code's hint on recaps; the title comes from the most recently written
// transcript. Tool results, slash commands, intermediate tool_use blurbs,
// sidechains, thinking blocks, and synthetic messages are excluded.
func TestLoad(t *testing.T) {
	projects := t.TempDir()
	cwd := "/home/u/repo/.vineyard/worktrees/fix-1"
	dir := filepath.Join(projects, "-home-u-repo--vineyard-worktrees-fix-1")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	writeTranscript(t, dir, "old.jsonl", now.Add(-time.Hour),
		`{"type":"ai-title","aiTitle":"Old title","sessionId":"old"}`,
		`{"type":"user","message":{"role":"user","content":"fix the login"},"origin":{"kind":"human"},"promptSource":"typed","timestamp":"2026-10-09T07:00:00.000Z","isSidechain":false}`,
		`{"type":"assistant","isSidechain":false,"message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"text","text":"Let me inspect login.go first."}]},"timestamp":"2026-10-09T07:00:10.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_1","type":"tool_result","content":"ok"}]},"timestamp":"2026-10-09T07:01:00.000Z"}`,
		`{"type":"assistant","isSidechain":false,"message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"thinking","thinking":"done"}]},"timestamp":"2026-10-09T07:02:00.000Z"}`,
		`{"type":"assistant","isSidechain":false,"message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"Fixed the redirect in login.go."}]},"timestamp":"2026-10-09T07:02:01.000Z"}`,
		`{"type":"assistant","isSidechain":true,"message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"Subagent reply."}]},"timestamp":"2026-10-09T07:03:00.000Z"}`,
		`{"type":"assistant","isSidechain":false,"message":{"model":"<synthetic>","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"No response requested."}]},"timestamp":"2026-10-09T07:04:00.000Z"}`,
		`{"type":"system","subtype":"away_summary","content":"First recap. (disable recaps in /config)","timestamp":"2026-10-09T08:00:00.000Z","isSidechain":false}`,
		`{"type":"system","subtype":"away_summary","content":"Third recap. (disable recaps in /config)","timestamp":"2026-10-09T10:00:00.000Z","isSidechain":false}`,
	)
	writeTranscript(t, dir, "new.jsonl", now,
		`{"type":"user","message":{"role":"user","content":"grep for \"away_summary\" in the logs"},"origin":{"kind":"human"},"promptSource":"queued","timestamp":"2026-10-09T08:30:00.000Z","isSidechain":false}`,
		`{"type":"user","message":{"role":"user","content":"<command-name>/model</command-name>\n            <command-message>model</command-message>\n            <command-args></command-args>"},"timestamp":"2026-10-09T08:40:00.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Base directory for this skill"}]},"isMeta":true,"timestamp":"2026-10-09T08:41:00.000Z"}`,
		`{"type":"system","subtype":"away_summary","content":"Second recap. (disable recaps in /config)","timestamp":"2026-10-09T09:00:00.000Z","isSidechain":false}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}},{"type":"text","text":"why is this red?"}]},"origin":{"kind":"human"},"promptSource":"sdk","timestamp":"2026-10-09T09:30:00.000Z"}`,
		`{"type":"assistant","isSidechain":false,"message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"It uses ColorError."}]},"timestamp":"2026-10-09T09:31:00.000Z"}`,
		`{"type":"ai-title","aiTitle":"First title","sessionId":"new"}`,
		`{"type":"ai-title","aiTitle":"New title","sessionId":"new"}`,
	)

	r, err := Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	got := formatEntries(r.Entries)
	want := []string{
		"user: fix the login",
		"response: Fixed the redirect in login.go.",
		"recap: First recap.",
		`user: grep for "away_summary" in the logs`,
		"recap: Second recap.",
		"user: why is this red?",
		"response: It uses ColorError.",
		"recap: Third recap.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("entries:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !r.Found || r.Title != "New title" {
		t.Errorf("got Found=%v Title=%q, want true and the newest transcript's last title", r.Found, r.Title)
	}
}

func TestLoad_NoTranscript(t *testing.T) {
	r, err := Load(t.TempDir(), "/nowhere")
	if err != nil || r.Found {
		t.Errorf("a directory without transcripts should be no recap, not an error; got %+v, %v", r, err)
	}
}

func TestReader_IncrementalPartialAndDelete(t *testing.T) {
	projects := t.TempDir()
	cwd := "/home/u/repo"
	dir := Dir(projects, cwd)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	r := NewReader()
	convPath := filepath.Join(dir, "conv1.jsonl")
	appendRaw := func(s string) {
		t.Helper()
		f, err := os.OpenFile(convPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(s); err != nil {
			t.Fatal(err)
		}
	}

	line1 := `{"type":"ai-title","aiTitle":"Initial","sessionId":"conv1"}` + "\n" +
		`{"type":"user","message":{"role":"user","content":"step one"},"origin":{"kind":"human"},"timestamp":"2026-10-09T07:00:00.000Z"}` + "\n"
	appendRaw(line1)

	got1, err := r.Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !got1.Found || got1.Title != "Initial" || len(got1.Entries) != 1 || got1.Entries[0].Text != "step one" {
		t.Fatalf("first Load = %+v", got1)
	}
	savedOffset := r.files[convPath].offset

	// Unchanged poll reuses cached state without re-reading.
	gotSame, err := r.Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !gotSame.Equal(got1) {
		t.Fatalf("unchanged Load differed: %+v vs %+v", gotSame, got1)
	}

	// Partial JSON at EOF does not fail or advance offset; completing it delivers the new entry.
	appendRaw(`{"type":"system","subtype":"away_summary","content":"Partial recap`)
	gotPartial, err := r.Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !gotPartial.Equal(got1) || r.files[convPath].offset != savedOffset {
		t.Fatalf("partial EOF Load = %+v (offset %d vs %d)", gotPartial, r.files[convPath].offset, savedOffset)
	}

	appendRaw(`. (disable recaps in /config)","timestamp":"2026-10-09T08:00:00.000Z"}` + "\n")
	got2, err := r.Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2.Entries) != 2 || got2.Entries[1].Kind != KindRecap || got2.Entries[1].Text != "Partial recap." {
		t.Fatalf("completed append Load = %+v", got2)
	}
	if got2.Equal(got1) {
		t.Error("Equal reported true after new entry was added")
	}

	// Same instant across locations compares equal; different Kind does not.
	utc := time.Date(2026, 10, 9, 7, 0, 0, 0, time.UTC)
	fixed := utc.In(time.FixedZone("UTC+2", 2*3600))
	a := Recap{Found: true, Title: "T", Entries: []Entry{{Time: utc, Text: "hi", Kind: KindPrompt}}}
	b := Recap{Found: true, Title: "T", Entries: []Entry{{Time: fixed, Text: "hi", Kind: KindPrompt}}}
	c := Recap{Found: true, Title: "T", Entries: []Entry{{Time: utc, Text: "hi", Kind: KindResponse}}}
	if !a.Equal(b) {
		t.Error("Recap.Equal should treat equivalent instants as equal")
	}
	if a.Equal(c) {
		t.Error("Recap.Equal must distinguish entry kinds")
	}

	// Removing the transcript clears cache entry and Found becomes false.
	if err := os.Remove(convPath); err != nil {
		t.Fatal(err)
	}
	gotDeleted, err := r.Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if gotDeleted.Found || len(gotDeleted.Entries) != 0 || len(r.files) != 0 {
		t.Fatalf("after delete: %+v, cached files=%d", gotDeleted, len(r.files))
	}
}

func TestLoadAgy_FirstTurnIncrementalAndWorkspaceIsolation(t *testing.T) {
	gemini := t.TempDir()
	t.Setenv("ANTIGRAVITY_GEMINI_DIR", gemini)
	agyDir, err := AntigravityDir()
	if err != nil {
		t.Fatal(err)
	}
	if agyDir != filepath.Join(gemini, "antigravity-cli") {
		t.Fatalf("AntigravityDir = %q", agyDir)
	}

	cwd := "/home/u/repo/.vineyard/worktrees/recap-1"
	sibling := "/home/u/repo/.vineyard/worktrees/other-2"
	r := NewReader()

	// Empty state: Found is false.
	empty, err := r.LoadAgy(agyDir, cwd, "")
	if err != nil || empty.Found {
		t.Fatalf("empty LoadAgy = %+v, %v", empty, err)
	}

	histPath := filepath.Join(agyDir, "history.jsonl")
	if err := os.MkdirAll(agyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appendHistory := func(ws, convID, typ, display string, ts time.Time) {
		t.Helper()
		rec := map[string]any{
			"display":   display,
			"timestamp": ts.UnixMilli(),
			"workspace": ws,
		}
		if convID != "" {
			rec["conversationId"] = convID
		}
		if typ != "" {
			rec["type"] = typ
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(histPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	appendStep := func(convID string, step map[string]any) {
		t.Helper()
		p := filepath.Join(agyDir, "brain", convID, ".system_generated", "logs", "transcript_full.jsonl")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(step)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	writeTitle := func(convID, raw string) {
		t.Helper()
		p := filepath.Join(agyDir, "annotations", convID+".pbtxt")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t0 := time.Date(2026, 11, 15, 9, 12, 33, 407_000_000, time.UTC)

	// Sibling workspace conversation must never bleed into cwd.
	appendHistory(sibling, "conv-other", "", "other worktree prompt", t0)
	appendStep("conv-other", map[string]any{
		"step_index": 0, "source": "USER_EXPLICIT", "type": "USER_INPUT",
		"created_at": t0.Truncate(time.Second).Format(time.RFC3339),
		"content":    "<USER_REQUEST>\nother worktree prompt\n</USER_REQUEST>",
	})
	writeTitle("conv-other", `title:"Other Session"`)

	// Slash command alone marks Found=true with zero entries.
	appendHistory(cwd, "", "slash_command", "/model", t0)
	slashOnly, err := r.LoadAgy(agyDir, cwd, "")
	if err != nil || !slashOnly.Found || len(slashOnly.Entries) != 0 {
		t.Fatalf("after slash command: %+v, %v", slashOnly, err)
	}

	// First prompt submitted before brain/<id> flushes to disk: visible right away,
	// without conversationId in history.jsonl.
	firstPrompt := "check recap tab\nwith two lines"
	appendHistory(cwd, "", "", firstPrompt, t0.Add(time.Second))
	beforeBrain, err := r.LoadAgy(agyDir, cwd, "")
	if err != nil || len(beforeBrain.Entries) != 1 || beforeBrain.Entries[0].Text != firstPrompt {
		t.Fatalf("before brain transcript: %+v, %v", beforeBrain, err)
	}

	// Brain transcript and annotation appear for turn 1 (still no conversationId in history.jsonl).
	convID := "conv-recap-1"
	t1 := t0.Add(time.Second).Truncate(time.Second)
	appendStep(convID, map[string]any{
		"step_index": 0, "source": "USER_EXPLICIT", "type": "USER_INPUT",
		"created_at": t1.Format(time.RFC3339),
		"content":    "<USER_REQUEST>\n" + firstPrompt + "\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nnow\n</ADDITIONAL_METADATA>",
	})
	// Intermediate tool-calling PLANNER_RESPONSE (empty content) and GENERIC tool output (even mentioning PLANNER_RESPONSE) are ignored.
	appendStep(convID, map[string]any{
		"step_index": 1, "source": "MODEL", "type": "PLANNER_RESPONSE",
		"created_at": t1.Add(time.Second).Format(time.RFC3339),
		"tool_calls": []any{map[string]any{"name": "read_file"}},
	})
	appendStep(convID, map[string]any{
		"step_index": 2, "source": "MODEL", "type": "GENERIC",
		"created_at": t1.Add(2 * time.Second).Format(time.RFC3339),
		"content":    `grep output mentioning "PLANNER_RESPONSE" and "USER_INPUT"`,
	})
	writeTitle(convID, "title:\"Recap \\\"Tab\\\" Support\"\n")

	turn1, err := r.LoadAgy(agyDir, cwd, "")
	if err != nil {
		t.Fatal(err)
	}
	if turn1.Title != `Recap "Tab" Support` {
		t.Errorf("Title = %q, want unquoted pbtxt title", turn1.Title)
	}
	// Must not duplicate the first prompt despite sub-second history vs RFC3339 created_at difference.
	if got := formatEntries(turn1.Entries); strings.Join(got, "|") != "user: "+firstPrompt {
		t.Fatalf("turn 1 entries = %v", got)
	}

	// Agent finishes turn 1 and appends its final response; history.jsonl has not changed.
	appendStep(convID, map[string]any{
		"step_index": 3, "source": "MODEL", "type": "PLANNER_RESPONSE",
		"created_at": t1.Add(5 * time.Second).Format(time.RFC3339),
		"content":    "Here is what `agy` stores on disk.\n",
	})
	turn1Done, err := r.LoadAgy(agyDir, cwd, "")
	if err != nil {
		t.Fatal(err)
	}
	wantTurn1 := []string{
		"user: " + firstPrompt,
		"response: Here is what `agy` stores on disk.",
	}
	if got := formatEntries(turn1Done.Entries); strings.Join(got, "\n") != strings.Join(wantTurn1, "\n") {
		t.Fatalf("turn 1 done:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantTurn1, "\n"))
	}

	// Second turn includes conversationId in history.jsonl.
	t2 := t1.Add(20 * time.Second)
	appendHistory(cwd, convID, "", "ship it", t2)
	appendStep(convID, map[string]any{
		"step_index": 4, "source": "USER_EXPLICIT", "type": "USER_INPUT",
		"created_at": t2.Format(time.RFC3339),
		"content":    "<USER_REQUEST>\nship it\n</USER_REQUEST>",
	})
	appendStep(convID, map[string]any{
		"step_index": 5, "source": "MODEL", "type": "PLANNER_RESPONSE",
		"created_at": t2.Add(3 * time.Second).Format(time.RFC3339),
		"content":    "Opened PR #90.",
	})

	turn2, err := r.LoadAgy(agyDir, cwd, "")
	if err != nil {
		t.Fatal(err)
	}
	wantTurn2 := []string{
		"user: " + firstPrompt,
		"response: Here is what `agy` stores on disk.",
		"user: ship it",
		"response: Opened PR #90.",
	}
	if got := formatEntries(turn2.Entries); strings.Join(got, "\n") != strings.Join(wantTurn2, "\n") {
		t.Fatalf("turn 2:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantTurn2, "\n"))
	}
}

func BenchmarkReader_IdleLoad(b *testing.B) {
	projects := b.TempDir()
	cwd := "/home/u/repo"
	dir := Dir(projects, cwd)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	toolLine := `{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_1","type":"tool_result","content":"` + strings.Repeat("x", 200) + `"}]},"timestamp":"2026-10-09T07:01:00.000Z"}` + "\n"
	var content strings.Builder
	content.WriteString(`{"type":"ai-title","aiTitle":"Big","sessionId":"big"}` + "\n")
	content.WriteString(`{"type":"user","message":{"role":"user","content":"start"},"origin":{"kind":"human"},"timestamp":"2026-10-09T07:00:00.000Z"}` + "\n")
	for range 10000 { // ~3 MiB transcript
		content.WriteString(toolLine)
	}
	if err := os.WriteFile(filepath.Join(dir, "big.jsonl"), []byte(content.String()), 0o600); err != nil {
		b.Fatal(err)
	}
	r := NewReader()
	if _, err := r.Load(projects, cwd); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		rec, err := r.Load(projects, cwd)
		if err != nil || !rec.Found {
			b.Fatalf("Load = %+v, %v", rec, err)
		}
	}
}

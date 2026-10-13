package recap

import (
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

// The user's messages and the recaps of every transcript of a working
// directory are merged, oldest first, without Claude Code's hint on recaps;
// the title comes from the most recently written transcript. Tool results,
// slash commands, and the text Claude Code adds are not messages. Lines are
// as Claude Code 2.1.282 to 2.1.295 writes them.
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
		`{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_1","type":"tool_result","content":"ok"}]},"timestamp":"2026-10-09T07:01:00.000Z"}`,
		`{"type":"system","subtype":"away_summary","content":"First recap. (disable recaps in /config)","timestamp":"2026-10-09T08:00:00.000Z","isSidechain":false}`,
		`{"type":"system","subtype":"away_summary","content":"Third recap. (disable recaps in /config)","timestamp":"2026-10-09T10:00:00.000Z","isSidechain":false}`,
	)
	writeTranscript(t, dir, "new.jsonl", now,
		`{"type":"user","message":{"role":"user","content":"grep for \"away_summary\" in the logs"},"origin":{"kind":"human"},"promptSource":"queued","timestamp":"2026-10-09T08:30:00.000Z","isSidechain":false}`,
		`{"type":"user","message":{"role":"user","content":"<command-name>/model</command-name>\n            <command-message>model</command-message>\n            <command-args></command-args>"},"timestamp":"2026-10-09T08:40:00.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Base directory for this skill"}]},"isMeta":true,"timestamp":"2026-10-09T08:41:00.000Z"}`,
		`{"type":"system","subtype":"away_summary","content":"Second recap. (disable recaps in /config)","timestamp":"2026-10-09T09:00:00.000Z","isSidechain":false}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}},{"type":"text","text":"why is this red?"}]},"origin":{"kind":"human"},"promptSource":"sdk","timestamp":"2026-10-09T09:30:00.000Z"}`,
		`{"type":"ai-title","aiTitle":"First title","sessionId":"new"}`,
		`{"type":"ai-title","aiTitle":"New title","sessionId":"new"}`,
	)

	r, err := Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range r.Entries {
		kind := "user"
		if e.Recap {
			kind = "recap"
		}
		got = append(got, kind+": "+e.Text)
	}
	want := []string{
		"user: fix the login",
		"recap: First recap.",
		`user: grep for "away_summary" in the logs`,
		"recap: Second recap.",
		"user: why is this red?",
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

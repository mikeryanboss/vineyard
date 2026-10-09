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

// Recaps of every transcript of a working directory are merged, newest first,
// without Claude Code's hint; the title and last prompt come from the most
// recently written transcript. Lines are as Claude Code 2.1.295 writes them.
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
		`{"type":"system","subtype":"away_summary","content":"First recap. (disable recaps in /config)","timestamp":"2026-10-09T08:00:00.000Z","isSidechain":false}`,
		`{"type":"system","subtype":"away_summary","content":"Third recap. (disable recaps in /config)","timestamp":"2026-10-09T10:00:00.000Z","isSidechain":false}`,
	)
	writeTranscript(t, dir, "new.jsonl", now,
		`{"type":"user","message":{"role":"user","content":"grep for \"away_summary\" in the logs"}}`,
		`{"type":"system","subtype":"away_summary","content":"Second recap. (disable recaps in /config)","timestamp":"2026-10-09T09:00:00.000Z","isSidechain":false}`,
		`{"type":"ai-title","aiTitle":"First title","sessionId":"new"}`,
		`{"type":"ai-title","aiTitle":"New title","sessionId":"new"}`,
		`{"type":"last-prompt","lastPrompt":"fix the login","sessionId":"new"}`,
	)

	r, err := Load(projects, cwd)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, e := range r.Entries {
		texts = append(texts, e.Text)
	}
	if got, want := strings.Join(texts, " | "), "Third recap. | Second recap. | First recap."; got != want {
		t.Errorf("recaps = %q, want %q", got, want)
	}
	if !r.Found || r.Title != "New title" || r.LastPrompt != "fix the login" {
		t.Errorf("got Found=%v Title=%q LastPrompt=%q, want true, the newest transcript's last title and prompt", r.Found, r.Title, r.LastPrompt)
	}
}

func TestLoad_NoTranscript(t *testing.T) {
	r, err := Load(t.TempDir(), "/nowhere")
	if err != nil || r.Found {
		t.Errorf("a directory without transcripts should be no recap, not an error; got %+v, %v", r, err)
	}
}

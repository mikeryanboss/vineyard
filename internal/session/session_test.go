package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mikeryanboss/vineyard/internal/git"
)

// fakeTerminal records sessions instead of running tmux.
type fakeTerminal struct {
	live     map[string]string // name -> dir
	startErr error
}

func newFakeTerminal() *fakeTerminal { return &fakeTerminal{live: map[string]string{}} }

func (f *fakeTerminal) Start(name, dir, program string, width, height int) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.live[name] = dir
	return nil
}

func (f *fakeTerminal) Exists(name string) bool { _, ok := f.live[name]; return ok }

func (f *fakeTerminal) Kill(name string) error { delete(f.live, name); return nil }

func newTestManager(t *testing.T) (*Manager, *fakeTerminal) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "README.md", "hello\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "initial")

	repo, err := git.FindRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	term := newFakeTerminal()
	m := NewManager(repo, term, filepath.Join(t.TempDir(), "project"))
	m.now = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }
	return m, term
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func startSession(t *testing.T, m *Manager, title string) Session {
	t.Helper()
	s, err := m.Start(m.New(NewOptions{Title: title, Program: "claude", BranchPrefix: "test/"}), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStart_CreatesBranchWorktreeAndTerminal(t *testing.T) {
	m, term := newTestManager(t)
	s := startSession(t, m, "Fix the login bug!")

	if s.Status != StatusRunning {
		t.Errorf("status = %s, want running", s.Status)
	}
	if s.Branch != "test/fix-the-login-bug" {
		t.Errorf("branch = %q", s.Branch)
	}
	if !git.IsWorktree(s.WorktreePath) {
		t.Error("worktree not created")
	}
	if term.live[s.TmuxName] != s.WorktreePath {
		t.Error("program must run inside the worktree")
	}
	if head, _ := git.Head(m.Repo.Root); s.BaseCommit != head {
		t.Errorf("base commit = %q, want HEAD %q", s.BaseCommit, head)
	}
}

func TestStart_NumbersTakenBranchNames(t *testing.T) {
	m, _ := newTestManager(t)
	first := startSession(t, m, "same")
	m.now = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 1, 0, time.UTC) }
	second := startSession(t, m, "same")
	if first.Branch == second.Branch || second.Branch != "test/same-2" {
		t.Errorf("branches = %q, %q; want a numbered second branch", first.Branch, second.Branch)
	}
}

func TestStart_FailedLaunchLeavesNothingBehind(t *testing.T) {
	m, term := newTestManager(t)
	term.startErr = errors.New("no such program")
	s, err := m.Start(m.New(NewOptions{Title: "broken", Program: "nope", BranchPrefix: "test/"}), 80, 24)
	if err == nil {
		t.Fatal("expected start to fail")
	}
	if _, statErr := os.Stat(s.WorktreePath); !os.IsNotExist(statErr) {
		t.Error("worktree should be removed after a failed launch")
	}
	if git.BranchExists(m.Repo.Root, s.Branch) {
		t.Error("branch should be deleted after a failed launch")
	}
}

func TestPauseResume_PreservesWork(t *testing.T) {
	m, term := newTestManager(t)
	s := startSession(t, m, "work")
	writeFile(t, s.WorktreePath, "work.txt", "progress\n")

	paused, err := m.Pause(s)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != StatusPaused || term.Exists(s.TmuxName) {
		t.Error("pause should stop the terminal and mark the session paused")
	}
	if _, err := os.Stat(s.WorktreePath); !os.IsNotExist(err) {
		t.Error("pause should remove the worktree")
	}
	if n, _ := git.CommitsAhead(m.Repo.Root, s.BaseCommit, s.Branch); n != 1 {
		t.Errorf("pause should checkpoint the work as one commit, branch is %d ahead", n)
	}

	resumed, err := m.Resume(paused, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != StatusRunning || !term.Exists(s.TmuxName) {
		t.Error("resume should restart the terminal")
	}
	if content, err := os.ReadFile(filepath.Join(s.WorktreePath, "work.txt")); err != nil || string(content) != "progress\n" {
		t.Errorf("resumed worktree lost the work: %q, %v", content, err)
	}
}

func TestResume_RefusesBranchCheckedOutElsewhere(t *testing.T) {
	m, _ := newTestManager(t)
	paused, err := m.Pause(startSession(t, m, "busy"))
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, m.Repo.Root, "checkout", "-q", paused.Branch)

	if _, err := m.Resume(paused, 80, 24); err == nil || !strings.Contains(err.Error(), "checked out") {
		t.Errorf("err = %v, want a checked-out-elsewhere error", err)
	}
}

func TestResume_StoppedSessionKeepsUncommittedWork(t *testing.T) {
	m, term := newTestManager(t)
	s := startSession(t, m, "crashy")
	writeFile(t, s.WorktreePath, "draft.txt", "unsaved\n")
	delete(term.live, s.TmuxName) // the tmux server died
	s = m.Restore([]Session{s})[0]
	if s.Status != StatusStopped {
		t.Fatalf("restored status = %s, want stopped", s.Status)
	}

	if _, err := m.Resume(s, 80, 24); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.WorktreePath, "draft.txt")); err != nil {
		t.Error("restarting a stopped session must not recreate its worktree")
	}
}

func TestShell_StartsInWorktreeAndStopsWithSession(t *testing.T) {
	m, term := newTestManager(t)
	s := startSession(t, m, "shelly")
	if err := m.EnsureShell(s, 80, 24); err != nil {
		t.Fatal(err)
	}
	if term.live[s.ShellName()] != s.WorktreePath {
		t.Fatal("shell should run in the session's worktree")
	}
	if _, err := m.Pause(s); err != nil {
		t.Fatal(err)
	}
	if term.Exists(s.ShellName()) {
		t.Error("pausing must stop the shell, whose directory is about to be removed")
	}
	paused := s
	paused.Status = StatusPaused
	if err := m.EnsureShell(paused, 80, 24); err == nil {
		t.Error("a paused session has no worktree to open a shell in")
	}
}

func TestKill_DeletesBranchWithoutCommits(t *testing.T) {
	m, _ := newTestManager(t)
	s := startSession(t, m, "scratch")
	result, err := m.Kill(s)
	if err != nil {
		t.Fatal(err)
	}
	if result.KeptBranch || git.BranchExists(m.Repo.Root, s.Branch) {
		t.Error("a branch without commits should be deleted")
	}
}

func TestKill_KeepsBranchWithCommits(t *testing.T) {
	m, _ := newTestManager(t)
	s := startSession(t, m, "valuable")
	writeFile(t, s.WorktreePath, "keep.txt", "x\n")
	runGit(t, s.WorktreePath, "add", "-A")
	runGit(t, s.WorktreePath, "commit", "-q", "-m", "agent work")

	result, err := m.Kill(s)
	if err != nil {
		t.Fatal(err)
	}
	if !result.KeptBranch || !git.BranchExists(m.Repo.Root, s.Branch) {
		t.Error("a branch with commits must be kept")
	}
}

func TestRestore_MarksLiveSessionsReadyAndKeepsPaused(t *testing.T) {
	m, term := newTestManager(t)
	term.live["alive"] = "/x"
	got := m.Restore([]Session{
		{TmuxName: "alive", Status: StatusRunning},
		{TmuxName: "gone", Status: StatusReady},
		{TmuxName: "gone", Status: StatusPaused},
	})
	want := []Status{StatusReady, StatusStopped, StatusPaused}
	for i := range want {
		if got[i].Status != want[i] {
			t.Errorf("session %d status = %s, want %s", i, got[i].Status, want[i])
		}
	}
}

func TestStore_RoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	if sessions, err := store.Load(); err != nil || len(sessions) != 0 {
		t.Fatalf("empty store = %v, %v", sessions, err)
	}
	saved := []Session{{ID: "a", Title: "A", Status: StatusPaused, PendingPrompt: "go"}}
	if err := store.Save(saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Title != "A" || loaded[0].PendingPrompt != "go" {
		t.Errorf("loaded = %+v", loaded)
	}
}

func TestLock_IsExclusive(t *testing.T) {
	dir := t.TempDir()
	release, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(dir); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock err = %v, want ErrLocked", err)
	}
	release()
	again, err := Lock(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	again()
}

func TestScreenDetection(t *testing.T) {
	if !AwaitingPermission("/usr/local/bin/claude --verbose", "1. Yes\n3. No, and tell Claude what to do differently") {
		t.Error("claude permission prompt not detected")
	}
	if AwaitingPermission("codex", "No, and tell Claude what to do differently") {
		t.Error("prompts must be matched per program")
	}
	if !AwaitingTrust("Do you trust the files in this folder?") {
		t.Error("trust prompt not detected")
	}
	if ScreenHash("a") == ScreenHash("b") {
		t.Error("different screens should hash differently")
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Fix the Login Bug!":  "fix-the-login-bug",
		"  --weird__name--  ": "weird-name",
		"???":                 "fallback",
		"Ünïcode café":        "n-code-caf",
	}
	for in, want := range cases {
		if got := Slug(in, "fallback"); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

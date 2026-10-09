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
	m := NewManager(repo, term, "shop-1a2b3c4d")
	m.now = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }
	return m, term
}

// worktrees is the default worktree directory, inside the main checkout.
func worktrees(m *Manager) string { return filepath.Join(m.Repo.Root, ".vineyard", "worktrees") }

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
	s, err := m.Start(m.New(NewOptions{Title: title, Program: "claude", BranchPrefix: "test/", WorktreeDir: worktrees(m)}), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// restore restores sessions as a restart does: through the store, which keeps
// only what is saved, and with the default worktree directory.
func restore(t *testing.T, m *Manager, sessions ...Session) []Session {
	t.Helper()
	store := NewStore(t.TempDir())
	if err := store.Save(sessions); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := m.Restore(loaded, worktrees(m))
	if err != nil {
		t.Fatal(err)
	}
	return restored
}

// moveRepo renames the repository directory, as moving or copying a project
// does, and returns a manager for it at the new location.
func moveRepo(t *testing.T, m *Manager) *Manager {
	t.Helper()
	moved := m.Repo.Root + "-moved"
	if err := os.Rename(m.Repo.Root, moved); err != nil {
		t.Fatal(err)
	}
	repo, err := git.FindRepo(moved)
	if err != nil {
		t.Fatal(err)
	}
	next := NewManager(repo, m.Terminal, m.Project)
	next.now = m.now
	return next
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

// Worktrees go where the caller says, named by session ID, and tmux names
// carry the project name so repositories sharing the socket never collide.
func TestNew_PlacesWorktreeAndNamesTmuxSession(t *testing.T) {
	m, _ := newTestManager(t)
	dir := t.TempDir()
	s := m.New(NewOptions{Title: "Fix login", Program: "claude", WorktreeDir: dir})
	if s.WorktreePath != filepath.Join(dir, s.ID) {
		t.Errorf("worktree path = %q, want under %q", s.WorktreePath, dir)
	}
	if s.TmuxName != "vineyard-shop-1a2b3c4d-"+s.ID {
		t.Errorf("tmux name = %q", s.TmuxName)
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

// A session started for a grapes issue follows the repository's branch
// convention, "<id>/<slug>", whatever the configured prefix.
func TestStart_IssueSessionBranchesByIssue(t *testing.T) {
	m, _ := newTestManager(t)
	s, err := m.Start(m.New(NewOptions{Title: "Embed grapes", Program: "claude", BranchPrefix: "test/", WorktreeDir: worktrees(m), Issue: 12}), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if s.Branch != "12/embed-grapes" || s.Issue != 12 {
		t.Errorf("branch, issue = %q, %d; want 12/embed-grapes, 12", s.Branch, s.Issue)
	}
}

func TestStart_FailedLaunchLeavesNothingBehind(t *testing.T) {
	m, term := newTestManager(t)
	term.startErr = errors.New("no such program")
	s, err := m.Start(m.New(NewOptions{Title: "broken", Program: "nope", BranchPrefix: "test/", WorktreeDir: worktrees(m)}), 80, 24)
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
	s = restore(t, m, s)[0]
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
	if result.KeptBranch != "" || git.BranchExists(m.Repo.Root, s.Branch) {
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
	if result.KeptBranch != s.Branch || !git.BranchExists(m.Repo.Root, s.Branch) {
		t.Error("a branch with commits must be kept")
	}
}

func TestRestore_MarksLiveSessionsReadyAndKeepsPaused(t *testing.T) {
	m, term := newTestManager(t)
	term.live["alive"] = "/x"
	got := restore(t, m,
		Session{TmuxName: "alive", Status: StatusRunning},
		Session{TmuxName: "gone", Status: StatusReady},
		Session{TmuxName: "gone", Status: StatusPaused},
	)
	want := []Status{StatusReady, StatusStopped, StatusPaused}
	for i := range want {
		if got[i].Status != want[i] {
			t.Errorf("session %d status = %s, want %s", i, got[i].Status, want[i])
		}
	}
}

func TestRestore_FindsWorktreesAfterRepositoryMoves(t *testing.T) {
	m, _ := newTestManager(t)
	relative := startSession(t, m, "relative")
	if link, _ := os.ReadFile(filepath.Join(relative.WorktreePath, ".git")); strings.HasPrefix(string(link), "gitdir: /") {
		t.Errorf("new worktrees must link to the repository relatively: %s", link)
	}
	// A worktree created before Vineyard used relative links.
	legacy := m.New(NewOptions{Title: "legacy", Program: "claude", WorktreeDir: worktrees(m)})
	runGit(t, m.Repo.Root, "worktree", "add", "-q", "-b", "legacy", legacy.WorktreePath)
	paused, err := m.Pause(startSession(t, m, "paused"))
	if err != nil {
		t.Fatal(err)
	}

	m = moveRepo(t, m)
	got := restore(t, m, relative, legacy, paused)

	for _, s := range got {
		if want := filepath.Join(worktrees(m), s.ID); s.WorktreePath != want {
			t.Errorf("%s: worktree path = %q, want %q", s.Title, s.WorktreePath, want)
		}
	}
	for _, s := range got[:2] {
		if !git.IsWorktree(s.WorktreePath) {
			t.Errorf("%s: worktree unusable after the move", s.Title)
		}
	}
	if _, err := m.Resume(got[2], 80, 24); err != nil {
		t.Fatal(err)
	}
	if !git.IsWorktree(got[2].WorktreePath) {
		t.Error("a paused session should resume inside the moved repository")
	}
}

func TestBranch_FollowsTheAgentsCheckout(t *testing.T) {
	m, _ := newTestManager(t)
	s := startSession(t, m, "switcher")
	runGit(t, s.WorktreePath, "switch", "-q", "-c", "19/real-work")

	if got := restore(t, m, s)[0].Branch; got != "19/real-work" {
		t.Errorf("restored branch = %q, want the checked-out 19/real-work", got)
	}
	paused, err := m.Pause(s)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Branch != "19/real-work" {
		t.Errorf("paused branch = %q, want the checked-out 19/real-work", paused.Branch)
	}
	resumed, err := m.Resume(paused, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if branch, _ := git.CurrentBranch(resumed.WorktreePath); branch != "19/real-work" {
		t.Errorf("resumed on %q, want 19/real-work", branch)
	}
}

func TestPushAndKill_UseTheCheckedOutBranch(t *testing.T) {
	m, _ := newTestManager(t)
	remote := t.TempDir()
	runGit(t, remote, "init", "-q", "--bare")
	runGit(t, m.Repo.Root, "remote", "add", "origin", remote)
	s := startSession(t, m, "pusher")
	runGit(t, s.WorktreePath, "switch", "-q", "-c", "19/real-work")
	writeFile(t, s.WorktreePath, "work.txt", "x\n")

	pushed, err := m.Push(s)
	if err != nil {
		t.Fatal(err)
	}
	if pushed.Branch != "19/real-work" {
		t.Errorf("pushed branch = %q, want 19/real-work", pushed.Branch)
	}
	runGit(t, remote, "rev-parse", "--verify", "refs/heads/19/real-work")

	result, err := m.Kill(s)
	if err != nil {
		t.Fatal(err)
	}
	if result.KeptBranch != "19/real-work" {
		t.Errorf("kept branch = %q, want the one with the commit, 19/real-work", result.KeptBranch)
	}
}

func TestLifecycle_RefusesDetachedHead(t *testing.T) {
	m, _ := newTestManager(t)
	s := startSession(t, m, "detached")
	runGit(t, s.WorktreePath, "switch", "-q", "--detach")

	if _, err := m.Pause(s); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Errorf("pause err = %v, want a detached HEAD error", err)
	}
	if _, err := m.Push(s); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Errorf("push err = %v, want a detached HEAD error", err)
	}
	if _, err := m.Kill(s); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Errorf("kill err = %v, want a detached HEAD error", err)
	}
	if !git.IsWorktree(s.WorktreePath) {
		t.Error("a refused operation must leave the worktree in place")
	}
}

// The worktree path belongs to the machine and git records it; saving it
// breaks sessions when the repository moves.
func TestStore_DoesNotSaveWorktreePath(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Save([]Session{{ID: "a", WorktreePath: "/home/someone/shop/.vineyard/worktrees/a"}}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "/home/someone") {
		t.Errorf("sessions.json holds a machine path:\n%s", content)
	}
}

func TestStore_RoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	if sessions, err := store.Load(); err != nil || len(sessions) != 0 {
		t.Fatalf("empty store = %v, %v", sessions, err)
	}
	saved := []Session{{ID: "a", Title: "A", Status: StatusPaused, PendingPrompt: "go", Issue: 12}}
	if err := store.Save(saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Title != "A" || loaded[0].PendingPrompt != "go" || loaded[0].Issue != 12 {
		t.Errorf("loaded = %+v", loaded)
	}
}

// Sessions saved before sessions recorded issues have no "issue" key.
func TestStore_LoadsSessionsWithoutIssue(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sessions.json", `{"version": 1, "sessions": [{"id": "a", "title": "A", "status": "paused"}]}`)
	loaded, err := NewStore(dir).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Issue != 0 {
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

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// newRepo creates a repository with one commit and returns its path. Git runs
// with an isolated configuration so the user's settings cannot change results.
func newRepo(t *testing.T) string {
	t.Helper()
	isolateGit(t)
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "README.md", "hello\n")
	mustGit(t, dir, "add", "-A")
	mustGit(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

func mustGit(t *testing.T, dir string, args ...string) string {
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
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindRepo_FromLinkedWorktreeNamesMainCheckout(t *testing.T) {
	root := newRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, root, "worktree", "add", "-q", "-b", "side", linked)

	repo, err := FindRepo(linked)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, _ := filepath.EvalSymlinks(root)
	gotRoot, _ := filepath.EvalSymlinks(repo.Root)
	if gotRoot != wantRoot {
		t.Errorf("Root = %q, want main checkout %q", repo.Root, root)
	}
	wantCheckout, _ := filepath.EvalSymlinks(linked)
	gotCheckout, _ := filepath.EvalSymlinks(repo.Checkout)
	if gotCheckout != wantCheckout {
		t.Errorf("Checkout = %q, want %q", repo.Checkout, linked)
	}
}

func TestFindRepo_RejectsRepositoryWithoutCommits(t *testing.T) {
	isolateGit(t)
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q")
	if _, err := FindRepo(dir); err != ErrNoCommits {
		t.Fatalf("err = %v, want ErrNoCommits", err)
	}
}

func TestFindRepo_RejectsPlainDirectory(t *testing.T) {
	isolateGit(t)
	if _, err := FindRepo(t.TempDir()); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}

func TestWorktreeLifecycle_KeepsBranchAfterRemove(t *testing.T) {
	root := newRepo(t)
	base, _ := Head(root)
	path := filepath.Join(t.TempDir(), "wt")

	if err := AddWorktree(root, path, "feature", base); err != nil {
		t.Fatal(err)
	}
	if !IsWorktree(path) {
		t.Fatal("worktree was not created")
	}
	if got, _ := BranchCheckout(root, "feature"); got == "" {
		t.Error("branch should be checked out in the new worktree")
	}

	writeFile(t, path, "new.txt", "one\ntwo\n")
	committed, err := CommitAll(path, "checkpoint")
	if err != nil || !committed {
		t.Fatalf("CommitAll = %v, %v; want a commit", committed, err)
	}
	if n, _ := CommitsAhead(root, base, "feature"); n != 1 {
		t.Errorf("CommitsAhead = %d, want 1", n)
	}

	if err := RemoveWorktree(root, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("worktree directory should be gone")
	}
	if !BranchExists(root, "feature") {
		t.Fatal("branch must survive worktree removal")
	}
	if got, _ := BranchCheckout(root, "feature"); got != "" {
		t.Errorf("branch still checked out at %q", got)
	}

	if err := AddWorktreeForBranch(root, path, "feature"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "new.txt")); err != nil {
		t.Error("re-added worktree should contain the committed work")
	}
}

func TestRemoveWorktree_ToleratesDeletedDirectory(t *testing.T) {
	root := newRepo(t)
	base, _ := Head(root)
	path := filepath.Join(t.TempDir(), "wt")
	if err := AddWorktree(root, path, "gone", base); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWorktree(root, path); err != nil {
		t.Fatalf("removing a hand-deleted worktree should succeed, got %v", err)
	}
	if err := AddWorktreeForBranch(root, path, "gone"); err != nil {
		t.Fatalf("branch should be reusable after prune: %v", err)
	}
}

func TestCommitAll_CleanTreeIsNoop(t *testing.T) {
	root := newRepo(t)
	committed, err := CommitAll(root, "nothing")
	if err != nil || committed {
		t.Fatalf("CommitAll on a clean tree = %v, %v; want false, nil", committed, err)
	}
}

func TestDiff_IncludesCommittedUncommittedAndUntracked(t *testing.T) {
	root := newRepo(t)
	base, _ := Head(root)

	writeFile(t, root, "committed.txt", "a\n")
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "work")
	writeFile(t, root, "README.md", "hello\nworld\n")
	writeFile(t, root, "untracked.txt", "x\ny\nz\n")
	writeFile(t, root, ".gitignore", "ignored.txt\n")
	writeFile(t, root, "ignored.txt", "secret\n")

	out, err := Diff(root, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"b/committed.txt", "+world", "b/untracked.txt", "+z"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Error("diff must not include ignored files")
	}

	stat, err := DiffStat(root, base)
	if err != nil {
		t.Fatal(err)
	}
	// committed.txt +1, README +1, untracked +3, .gitignore +1
	if stat.Added != 6 || stat.Removed != 0 {
		t.Errorf("DiffStat = %+v, want +6 -0", stat)
	}
	for _, want := range []string{"committed.txt", "README.md", "untracked.txt", ".gitignore"} {
		if !slices.Contains(stat.Paths, want) {
			t.Errorf("DiffStat paths %v missing %q", stat.Paths, want)
		}
	}
}

// A branch's work stays the same as the mainline moves on, the agent rebases
// onto it, and a merge commit takes the branch in, as on GitHub.
func TestBase_FindsTheBranchsOwnWorkThroughRebaseAndMerge(t *testing.T) {
	root := newRepo(t)
	start, _ := Head(root)
	agent := filepath.Join(t.TempDir(), "agent")
	mustGit(t, root, "worktree", "add", "-q", "-b", "agent", agent)
	own := func() []string {
		t.Helper()
		base, err := Base(agent, start)
		if err != nil {
			t.Fatal(err)
		}
		stat, err := DiffStat(agent, base)
		if err != nil {
			t.Fatal(err)
		}
		return stat.Paths
	}

	writeFile(t, agent, "agent.txt", "work\n")
	mustGit(t, agent, "add", "-A")
	mustGit(t, agent, "commit", "-q", "-m", "agent's work")
	writeFile(t, root, "other.txt", "someone else's work\n")
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "other work")
	if got := own(); !slices.Equal(got, []string{"agent.txt"}) {
		t.Errorf("without origin/HEAD the work runs from start: got %v, want [agent.txt]", got)
	}

	publish := func() { mustGit(t, root, "update-ref", "refs/remotes/origin/main", "main") }
	publish()
	mustGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	mustGit(t, agent, "rebase", "-q", "main")
	if got := own(); !slices.Equal(got, []string{"agent.txt"}) {
		t.Errorf("rebased onto the mainline: got %v, want [agent.txt]", got)
	}

	mustGit(t, root, "merge", "-q", "--no-ff", "-m", "merge agent", "agent")
	writeFile(t, root, "later.txt", "after the merge\n")
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "later work")
	publish()
	if got := own(); !slices.Equal(got, []string{"agent.txt"}) {
		t.Errorf("merged into the mainline: got %v, want [agent.txt]", got)
	}

	fresh := filepath.Join(t.TempDir(), "fresh")
	mustGit(t, root, "worktree", "add", "-q", "-b", "fresh", fresh, "origin/main")
	head, _ := Head(fresh)
	if base, err := Base(fresh, start); err != nil || base != head {
		t.Errorf("Base of a branch at the mainline's tip = %v, %v; want HEAD %s", base, err, head)
	}
}

func TestDiff_LeavesIndexUntouched(t *testing.T) {
	root := newRepo(t)
	base, _ := Head(root)
	writeFile(t, root, "untracked.txt", "x\n")

	if _, err := Diff(root, base); err != nil {
		t.Fatal(err)
	}
	if status := mustGit(t, root, "status", "--porcelain"); status != "?? untracked.txt" {
		t.Errorf("status after Diff = %q; the file must stay untracked", status)
	}
}

func TestParseNumstat_SkipsBinaryCountsAndKeepsRenamedPaths(t *testing.T) {
	got := parseNumstat("3\t1\ta.go\x00-\t-\timage.png\x002\t0\t\x00old.go\x00new.go\x0010\t0\tb.go\x00")
	want := Stat{Added: 15, Removed: 1, Paths: []string{"a.go", "image.png", "old.go", "new.go", "b.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseNumstat = %+v, want %+v", got, want)
	}
}

// Package git wraps the git command line for the operations Vineyard needs:
// repository discovery, per-session worktrees, checkpoint commits, pushes, and
// diffs against a session's base commit.
//
// Everything shells out to git. The CLI is the one interface every repository
// layout, hook, and credential helper already works with.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Repo identifies the repository Vineyard was started in.
type Repo struct {
	// Root is the main checkout. When Vineyard starts inside a linked worktree,
	// Root still names the main checkout so every launch sees the same sessions.
	Root string
	// Checkout is the checkout Vineyard was started in. New sessions branch
	// from its HEAD.
	Checkout string
}

// Name returns the repository's directory name.
func (r Repo) Name() string { return filepath.Base(r.Root) }

// ErrNotRepo reports that a directory is not inside a git repository.
var ErrNotRepo = errors.New("not a git repository")

// ErrNoCommits reports a repository without an initial commit. Worktrees need
// a commit to branch from.
var ErrNoCommits = errors.New("repository has no commits yet; create an initial commit first")

// commandError keeps git's stderr, which carries the actual reason a command failed.
type commandError struct {
	args   []string
	err    error
	stderr string
}

func (e *commandError) Error() string {
	msg := strings.TrimSpace(e.stderr)
	if msg == "" {
		msg = e.err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.args, " "), msg)
}

func (e *commandError) Unwrap() error { return e.err }

// run executes git in dir and returns stdout.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &commandError{args: args, err: err, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

// exitCode returns the exit status carried by err, or -1.
func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// FindRepo locates the repository containing dir.
func FindRepo(dir string) (Repo, error) {
	checkout, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, fmt.Errorf("%w: %s", ErrNotRepo, dir)
	}
	commonDir, err := run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Repo{}, err
	}
	if _, err := run(dir, "rev-parse", "--verify", "HEAD"); err != nil {
		return Repo{}, ErrNoCommits
	}
	root := filepath.Dir(strings.TrimSpace(commonDir))
	return Repo{Root: filepath.Clean(root), Checkout: filepath.Clean(strings.TrimSpace(checkout))}, nil
}

// Head returns the commit checked out in dir.
func Head(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// HasPath reports whether commit rev of the repository at dir contains path,
// a file or directory relative to the repository root.
func HasPath(dir, rev, path string) bool {
	_, err := run(dir, "cat-file", "-e", rev+":"+filepath.ToSlash(path))
	return err == nil
}

// AddWorktree creates a worktree at path on a new branch that starts at base.
// Its links to the repository are relative, so they survive moving the
// repository together with the worktree.
func AddWorktree(repoRoot, path, branch, base string) error {
	_, err := run(repoRoot, "worktree", "add", "--relative-paths", "-b", branch, path, base)
	return err
}

// AddWorktreeForBranch creates a worktree at path for an existing branch,
// with relative links like AddWorktree.
func AddWorktreeForBranch(repoRoot, path, branch string) error {
	_, err := run(repoRoot, "worktree", "add", "--relative-paths", path, branch)
	return err
}

// RepairWorktree reconnects the worktree at path with the repository after
// either was moved, and rewrites their links as relative paths.
func RepairWorktree(repoRoot, path string) error {
	_, err := run(repoRoot, "worktree", "repair", "--relative-paths", path)
	return err
}

// Worktree is one checkout of the repository, as git records it.
type Worktree struct {
	Path string
	// Branch is the checked-out branch, or "" for a detached HEAD.
	Branch string
	// Prunable means git's record points at a directory that is gone, for
	// example because the repository or the worktree was moved.
	Prunable bool
}

// Worktrees lists the repository's checkouts, the main checkout first.
func Worktrees(repoRoot string) ([]Worktree, error) {
	out, err := run(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var worktrees []Worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			worktrees = append(worktrees, Worktree{Path: strings.TrimPrefix(line, "worktree ")})
		case strings.HasPrefix(line, "branch refs/heads/"):
			worktrees[len(worktrees)-1].Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "prunable" || strings.HasPrefix(line, "prunable "):
			worktrees[len(worktrees)-1].Prunable = true
		}
	}
	return worktrees, nil
}

// CurrentBranch returns the branch checked out in dir. A detached HEAD is an
// error: there is no branch to commit to, push, or check out again.
func CurrentBranch(dir string) (string, error) {
	out, err := run(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if exitCode(err) == 1 {
		return "", fmt.Errorf("%s has a detached HEAD; check out a branch first", dir)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RemoveWorktree deletes the worktree at path, discarding uncommitted changes,
// and prunes git's record of it. The branch is kept.
func RemoveWorktree(repoRoot, path string) error {
	_, removeErr := run(repoRoot, "worktree", "remove", "--force", path)
	// Prune even when remove failed: a directory deleted by hand leaves a
	// stale registration that would block re-adding the branch later.
	_, pruneErr := run(repoRoot, "worktree", "prune")
	if removeErr != nil && !IsWorktree(path) {
		removeErr = nil
	}
	return errors.Join(removeErr, pruneErr)
}

// IsWorktree reports whether path is the top of a usable git checkout.
func IsWorktree(path string) bool {
	out, err := run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	return filepath.Clean(strings.TrimSpace(out)) == filepath.Clean(path)
}

// BranchExists reports whether a local branch exists.
func BranchExists(repoRoot, branch string) bool {
	_, err := run(repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// DeleteBranch force-deletes a local branch.
func DeleteBranch(repoRoot, branch string) error {
	_, err := run(repoRoot, "branch", "-D", branch)
	return err
}

// BranchCheckout returns the path of the worktree that has branch checked
// out, or "" when no worktree does.
func BranchCheckout(repoRoot, branch string) (string, error) {
	worktrees, err := Worktrees(repoRoot)
	if err != nil {
		return "", err
	}
	for _, w := range worktrees {
		if w.Branch == branch {
			return w.Path, nil
		}
	}
	return "", nil
}

// CommitsAhead counts the commits on branch that base does not have.
func CommitsAhead(repoRoot, base, branch string) (int, error) {
	out, err := run(repoRoot, "rev-list", "--count", base+".."+branch)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// IsDirty reports whether dir has uncommitted changes, untracked files included.
func IsDirty(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// CommitAll stages every change in dir and commits it. It reports whether a
// commit was made; a clean worktree is not an error.
func CommitAll(dir, message string) (bool, error) {
	dirty, err := IsDirty(dir)
	if err != nil || !dirty {
		return false, err
	}
	if _, err := run(dir, "add", "-A"); err != nil {
		return false, err
	}
	if _, err := run(dir, "commit", "-m", message); err != nil {
		return false, err
	}
	return true, nil
}

// Push pushes branch to origin and sets it as the upstream.
func Push(dir, branch string) error {
	_, err := run(dir, "push", "-u", "origin", branch)
	return err
}

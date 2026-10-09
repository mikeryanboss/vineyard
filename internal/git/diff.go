package git

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxUntrackedFiles bounds the untracked files a diff includes. An agent that
// generates a dependency directory without ignoring it would otherwise make
// every refresh read thousands of files.
const maxUntrackedFiles = 200

// maxUntrackedBytes skips untracked files too large to be worth showing.
const maxUntrackedBytes = 1 << 20

// Stat summarises a diff: the lines it adds and removes, and the paths it
// changes, both sides of a rename.
type Stat struct {
	Added   int
	Removed int
	Paths   []string
}

// IsZero reports whether the diff is empty.
func (s Stat) IsZero() bool { return s.Added == 0 && s.Removed == 0 }

// Base returns the commit where the work of the branch checked out in dir
// begins, so that a diff from it shows that work and nothing the branch took
// in from the mainline, origin/HEAD. Without origin/HEAD there is no
// mainline to tell apart, and the work begins at start, the commit the branch
// was created from.
//
// Before the branch merges, its work begins where it forks from the mainline.
// Once a merge commit on the mainline's first-parent history brings it in, its
// work begins where it forks from that merge's first parent: the mainline as
// the merge found it. A fast-forward leaves no merge commit, and the branch's
// commits are then indistinguishable from the mainline's, so its work begins
// at HEAD.
func Base(dir, start string) (string, error) {
	if _, err := run(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/HEAD"); err != nil {
		if exitCode(err) == 1 {
			return start, nil
		}
		return "", err
	}
	fork, err := run(dir, "merge-base", "origin/HEAD", "HEAD")
	if err != nil {
		return "", err
	}
	head, err := Head(dir)
	if err != nil {
		return "", err
	}
	if fork = strings.TrimSpace(fork); fork != head {
		return fork, nil
	}
	after, err := run(dir, "rev-list", "--first-parent", "--ancestry-path", "--reverse", "HEAD..origin/HEAD")
	if err != nil {
		return "", err
	}
	merge, _, _ := strings.Cut(after, "\n")
	if merge == "" {
		return head, nil // HEAD is the mainline's tip
	}
	fork, err = run(dir, "merge-base", merge+"^1", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fork), nil
}

// Diff returns the unified diff from base to the working tree in dir,
// including untracked files that are not ignored.
//
// It never touches the index. Git's own trick for showing untracked files,
// `git add -N`, would write into the worktree's index while the agent works in it.
func Diff(dir, base string) (string, error) {
	tracked, err := run(dir, "diff", "--no-color", "--no-ext-diff", "-M", base)
	if err != nil {
		return "", err
	}
	untracked, err := untrackedFiles(dir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(tracked)
	for _, path := range untracked {
		out, err := run(dir, "diff", "--no-color", "--no-ext-diff", "--no-index", "--", os.DevNull, path)
		// --no-index exits 1 when the files differ, which they always do here.
		if err != nil && exitCode(err) != 1 {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

// DiffStat summarises the diff from base to the working tree, including
// untracked files. It is much cheaper than Diff for sessions whose full diff
// is not on screen.
func DiffStat(dir, base string) (Stat, error) {
	out, err := run(dir, "diff", "--numstat", "-z", "--no-ext-diff", "-M", base)
	if err != nil {
		return Stat{}, err
	}
	stat := parseNumstat(out)
	untracked, err := untrackedFiles(dir)
	if err != nil {
		return Stat{}, err
	}
	for _, path := range untracked {
		stat.Paths = append(stat.Paths, path)
		content, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || bytes.IndexByte(content, 0) >= 0 {
			continue // unreadable or binary: no line count to report
		}
		stat.Added += countLines(content)
	}
	return stat, nil
}

// untrackedFiles lists untracked, non-ignored files small enough to show.
func untrackedFiles(dir string) ([]string, error) {
	out, err := run(dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, path := range strings.Split(out, "\x00") {
		if path == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, path))
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxUntrackedBytes {
			continue
		}
		files = append(files, path)
		if len(files) == maxUntrackedFiles {
			break
		}
	}
	return files, nil
}

// parseNumstat summarises `git diff --numstat -z` output. Each file is
// "added TAB removed TAB path NUL"; a rename leaves the path empty and follows
// with "old NUL new NUL". Binary files report "-" for both counts.
func parseNumstat(out string) Stat {
	var stat Stat
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		fields := strings.SplitN(records[i], "\t", 3)
		if len(fields) < 3 {
			continue
		}
		if n, err := strconv.Atoi(fields[0]); err == nil {
			stat.Added += n
		}
		if n, err := strconv.Atoi(fields[1]); err == nil {
			stat.Removed += n
		}
		if fields[2] != "" {
			stat.Paths = append(stat.Paths, fields[2])
		} else {
			stat.Paths = append(stat.Paths, records[i+1], records[i+2])
			i += 2
		}
	}
	return stat
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := bytes.Count(content, []byte("\n"))
	if content[len(content)-1] != '\n' {
		n++
	}
	return n
}

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

// Stat summarises a diff as added and removed line counts.
type Stat struct {
	Added   int
	Removed int
}

// IsZero reports whether the diff is empty.
func (s Stat) IsZero() bool { return s.Added == 0 && s.Removed == 0 }

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

// DiffStat counts the lines added and removed from base to the working tree,
// including untracked files. It is much cheaper than Diff for sessions whose
// full diff is not on screen.
func DiffStat(dir, base string) (Stat, error) {
	out, err := run(dir, "diff", "--numstat", "--no-ext-diff", "-M", base)
	if err != nil {
		return Stat{}, err
	}
	stat := parseNumstat(out)
	untracked, err := untrackedFiles(dir)
	if err != nil {
		return Stat{}, err
	}
	for _, path := range untracked {
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

// parseNumstat sums `git diff --numstat` output. Binary files report "-".
func parseNumstat(out string) Stat {
	var stat Stat
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if n, err := strconv.Atoi(fields[0]); err == nil {
			stat.Added += n
		}
		if n, err := strconv.Atoi(fields[1]); err == nil {
			stat.Removed += n
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

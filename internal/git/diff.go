package git

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
const maxCachedBases = 4096

var baseCache struct {
	mu        sync.Mutex
	bases     map[[2]string]string
	ancestors map[[2]string]bool
}

func Base(dir, start string) (string, error) {
	out, err := run(dir, "rev-parse", "HEAD", "refs/remotes/origin/HEAD")
	if err == nil {
		if fields := strings.Fields(out); len(fields) == 2 {
			return baseOnHashes(dir, fields[0], fields[1])
		}
	}
	if _, verifyErr := run(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/HEAD"); verifyErr != nil {
		if exitCode(verifyErr) == 1 {
			return start, nil
		}
		return "", verifyErr
	}
	return "", err
}

// StackedBase is Base for a branch that may have been created from another
// branch, whose tip is onto, instead of from the mainline. Its work begins
// where it forks from that branch or from the mainline, whichever is later,
// and a merge commit on that branch's first-parent history takes it in as one
// on the mainline's does. Without origin/HEAD its work begins where it forks
// from that branch.
func StackedBase(dir, onto string) (string, error) {
	stacked, err := baseOn(dir, onto)
	if err != nil {
		return "", err
	}
	mainline, err := Base(dir, stacked)
	if err != nil {
		return "", err
	}
	if mainline == stacked {
		return stacked, nil
	}
	anc, err := isAncestor(dir, mainline, stacked)
	if err != nil {
		return "", err
	}
	if !anc {
		return mainline, nil
	}
	return stacked, nil
}

// baseOn is Base with the mainline's tip at mainline.
func baseOn(dir, mainline string) (string, error) {
	out, err := run(dir, "rev-parse", "HEAD", mainline)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return "", fmt.Errorf("unexpected rev-parse output %q", out)
	}
	return baseOnHashes(dir, fields[0], fields[1])
}

func baseOnHashes(dir, head, target string) (string, error) {
	key := [2]string{head, target}
	baseCache.mu.Lock()
	fork, ok := baseCache.bases[key]
	baseCache.mu.Unlock()
	if ok {
		return fork, nil
	}

	fork, err := run(dir, "merge-base", target, head)
	if err != nil {
		return "", err
	}
	if fork = strings.TrimSpace(fork); fork == head {
		after, err := run(dir, "rev-list", "--first-parent", "--ancestry-path", "--reverse", head+".."+target)
		if err != nil {
			return "", err
		}
		if merge, _, _ := strings.Cut(after, "\n"); merge != "" {
			fork, err = run(dir, "merge-base", merge+"^1", head)
			if err != nil {
				return "", err
			}
			fork = strings.TrimSpace(fork)
		}
	}

	baseCache.mu.Lock()
	if baseCache.bases == nil || len(baseCache.bases) >= maxCachedBases {
		baseCache.bases = map[[2]string]string{}
	}
	baseCache.bases[key] = fork
	baseCache.mu.Unlock()
	return fork, nil
}

func isAncestor(dir, ancestor, commit string) (bool, error) {
	key := [2]string{ancestor, commit}
	baseCache.mu.Lock()
	cached, ok := baseCache.ancestors[key]
	baseCache.mu.Unlock()
	if ok {
		return cached, nil
	}

	var result bool
	if _, err := run(dir, "merge-base", "--is-ancestor", ancestor, commit); err != nil {
		if exitCode(err) != 1 {
			return false, err
		}
	} else {
		result = true
	}

	baseCache.mu.Lock()
	if baseCache.ancestors == nil || len(baseCache.ancestors) >= maxCachedBases {
		baseCache.ancestors = map[[2]string]bool{}
	}
	baseCache.ancestors[key] = result
	baseCache.mu.Unlock()
	return result, nil
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
		if err := appendUntrackedDiff(&b, dir, path); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func appendUntrackedDiff(b *strings.Builder, dir, path string) error {
	fullPath := filepath.Join(dir, path)
	info, err := os.Lstat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}

	var mode string
	var data []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		mode = "120000"
		data = []byte(target)
	} else {
		mode = "100644"
		if info.Mode()&0o111 != 0 {
			mode = "100755"
		}
		data, err = os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
	}

	aPath := formatGitPath("a/" + path)
	bPath := formatGitPath("b/" + path)
	b.WriteString("diff --git ")
	b.WriteString(aPath)
	b.WriteByte(' ')
	b.WriteString(bPath)
	b.WriteString("\nnew file mode ")
	b.WriteString(mode)
	b.WriteString("\nindex 0000000..0000000\n")

	if len(data) == 0 {
		return nil
	}
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		b.WriteString("Binary files /dev/null and ")
		b.WriteString(bPath)
		b.WriteString(" differ\n")
		return nil
	}

	b.WriteString("--- /dev/null\n+++ ")
	b.WriteString(bPath)
	b.WriteByte('\n')

	hasNewline := data[len(data)-1] == '\n'
	lines := bytes.Split(data, []byte("\n"))
	if hasNewline {
		lines = lines[:len(lines)-1]
	}
	b.WriteString("@@ -0,0 +1")
	if len(lines) != 1 {
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(len(lines)))
	}
	b.WriteString(" @@\n")
	for _, line := range lines {
		b.WriteByte('+')
		b.Write(line)
		b.WriteByte('\n')
	}
	if !hasNewline {
		b.WriteString("\\ No newline at end of file\n")
	}
	return nil
}

// formatGitPath C-quotes paths that contain characters git quotes in headers,
// matching internal/diff.Parse's unquote expectations.
func formatGitPath(path string) string {
	quote := false
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c < 0x20 || c == 0x7f || c >= 0x80 || c == '"' || c == '\\' {
			quote = true
			break
		}
	}
	if !quote {
		return path
	}
	var b strings.Builder
	b.Grow(len(path) + 2)
	b.WriteByte('"')
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\v':
			b.WriteString(`\v`)
		default:
			if c < 0x20 || c == 0x7f || c >= 0x80 {
				fmt.Fprintf(&b, `\%03o`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
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

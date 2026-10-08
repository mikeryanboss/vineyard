// Package diff parses git's unified diff output into files, hunks, and
// numbered lines, and locates the changed part of edited lines.
package diff

import (
	"strconv"
	"strings"
	"unicode"
)

// FileStatus describes what happened to a file.
type FileStatus int

const (
	Modified FileStatus = iota
	Added
	Deleted
	Renamed
)

// LineKind is the role of a line within a hunk.
type LineKind int

const (
	Context LineKind = iota
	Add
	Delete
)

// File is one file's changes.
type File struct {
	OldPath string
	NewPath string
	Status  FileStatus
	Binary  bool
	Hunks   []Hunk
	Added   int
	Removed int
}

// Path is the file's name after the change, or before it for deletions.
func (f File) Path() string {
	if f.Status == Deleted {
		return f.OldPath
	}
	return f.NewPath
}

// Hunk is a contiguous block of changes.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	// Section is the enclosing function or heading git prints after the
	// hunk range, if any.
	Section string
	Lines   []Line
}

// Line is one line of a hunk. OldNo and NewNo are 1-based line numbers in
// the old and new file, and 0 where the line does not exist.
type Line struct {
	Kind  LineKind
	Text  string
	OldNo int
	NewNo int
	// NoNewline marks a last line without a trailing newline.
	NoNewline bool
}

// Parse reads `git diff` output. Unrecognised lines are ignored, so partial
// or unusual output still yields whatever files it can.
func Parse(text string) []File {
	var files []File
	var file *File
	var hunk *Hunk
	oldLeft, newLeft := 0, 0 // lines still expected in the current hunk
	oldNo, newNo := 0, 0

	flushHunk := func() {
		if file != nil && hunk != nil {
			file.Hunks = append(file.Hunks, *hunk)
		}
		hunk = nil
	}
	flushFile := func() {
		flushHunk()
		if file != nil {
			files = append(files, *file)
		}
		file = nil
	}

	for _, line := range strings.Split(text, "\n") {
		// Hunk bodies come first: a removed line reading "-- x" must not be
		// mistaken for a "--- a/x" file header.
		if hunk != nil && (oldLeft > 0 || newLeft > 0) {
			switch {
			case strings.HasPrefix(line, "+"):
				hunk.Lines = append(hunk.Lines, Line{Kind: Add, Text: line[1:], NewNo: newNo})
				newNo++
				newLeft--
				file.Added++
				continue
			case strings.HasPrefix(line, "-"):
				hunk.Lines = append(hunk.Lines, Line{Kind: Delete, Text: line[1:], OldNo: oldNo})
				oldNo++
				oldLeft--
				file.Removed++
				continue
			case strings.HasPrefix(line, " ") || line == "":
				text := line
				if text != "" {
					text = text[1:]
				}
				hunk.Lines = append(hunk.Lines, Line{Kind: Context, Text: text, OldNo: oldNo, NewNo: newNo})
				oldNo++
				newNo++
				oldLeft--
				newLeft--
				continue
			}
		}
		if strings.HasPrefix(line, `\`) { // "\ No newline at end of file"
			if hunk != nil && len(hunk.Lines) > 0 {
				hunk.Lines[len(hunk.Lines)-1].NoNewline = true
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
			oldPath, newPath := splitGitHeader(strings.TrimPrefix(line, "diff --git "))
			file = &File{OldPath: oldPath, NewPath: newPath}
		case file == nil:
			continue
		case strings.HasPrefix(line, "@@ "):
			flushHunk()
			h, oldCount, newCount, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			hunk = &h
			oldLeft, newLeft = oldCount, newCount
			oldNo, newNo = h.OldStart, h.NewStart
		case strings.HasPrefix(line, "new file mode"):
			file.Status = Added
		case strings.HasPrefix(line, "deleted file mode"):
			file.Status = Deleted
		case strings.HasPrefix(line, "rename from "):
			file.Status = Renamed
			file.OldPath = unquote(strings.TrimPrefix(line, "rename from "))
		case strings.HasPrefix(line, "rename to "):
			file.Status = Renamed
			file.NewPath = unquote(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch":
			file.Binary = true
		case strings.HasPrefix(line, "--- "):
			if path := headerPath(strings.TrimPrefix(line, "--- ")); path != "" {
				file.OldPath = path
			} else {
				file.Status = Added
			}
		case strings.HasPrefix(line, "+++ "):
			if path := headerPath(strings.TrimPrefix(line, "+++ ")); path != "" {
				file.NewPath = path
			} else {
				file.Status = Deleted
			}
		}
	}
	flushFile()
	return files
}

// parseHunkHeader reads "@@ -a,b +c,d @@ section".
func parseHunkHeader(line string) (h Hunk, oldCount, newCount int, ok bool) {
	rest := strings.TrimPrefix(line, "@@ ")
	end := strings.Index(rest, " @@")
	if end < 0 {
		return h, 0, 0, false
	}
	ranges := strings.Fields(rest[:end])
	if len(ranges) != 2 {
		return h, 0, 0, false
	}
	var okOld, okNew bool
	h.OldStart, oldCount, okOld = parseRange(strings.TrimPrefix(ranges[0], "-"))
	h.NewStart, newCount, okNew = parseRange(strings.TrimPrefix(ranges[1], "+"))
	h.OldLines, h.NewLines = oldCount, newCount
	h.Section = strings.TrimSpace(rest[end+3:])
	return h, oldCount, newCount, okOld && okNew
}

// parseRange reads "start,count", where a missing count means 1.
func parseRange(s string) (start, count int, ok bool) {
	startText, countText, hasCount := strings.Cut(s, ",")
	start, err := strconv.Atoi(startText)
	if err != nil {
		return 0, 0, false
	}
	count = 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil {
			return 0, 0, false
		}
	}
	return start, count, true
}

// splitGitHeader splits "a/x b/y" from a diff --git line. Paths containing
// " b/" are ambiguous there, which is why --- and +++ lines override it.
func splitGitHeader(s string) (oldPath, newPath string) {
	if strings.HasPrefix(s, `"`) {
		if end := strings.Index(s[1:], `" `); end >= 0 {
			return stripPrefix(unquote(s[:end+2])), stripPrefix(unquote(s[end+3:]))
		}
	}
	if i := strings.Index(s, " b/"); i >= 0 {
		return stripPrefix(s[:i]), stripPrefix(s[i+1:])
	}
	return s, s
}

// headerPath reads the path from a ---/+++ line, or "" for /dev/null.
func headerPath(s string) string {
	s = strings.TrimSuffix(s, "\t")
	if s == "/dev/null" {
		return ""
	}
	return stripPrefix(unquote(s))
}

func stripPrefix(path string) string {
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

// unquote decodes paths git C-quotes for unusual characters.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return s
}

// Range is a half-open span of rune indices within a line.
type Range struct{ Start, End int }

// Empty reports whether the range covers nothing.
func (r Range) Empty() bool { return r.End <= r.Start }

// Pairs matches each removed line with the added line that replaced it, so
// edits can be shown word by word. A run of removals followed directly by
// a run of additions pairs up in order. It returns index pairs into lines.
func Pairs(lines []Line) [][2]int {
	var pairs [][2]int
	for i := 0; i < len(lines); {
		if lines[i].Kind != Delete {
			i++
			continue
		}
		delStart := i
		for i < len(lines) && lines[i].Kind == Delete {
			i++
		}
		addStart := i
		for i < len(lines) && lines[i].Kind == Add {
			i++
		}
		for k := 0; k < addStart-delStart && addStart+k < i; k++ {
			pairs = append(pairs, [2]int{delStart + k, addStart + k})
		}
	}
	return pairs
}

// maxChangedShare is the fraction of a line above which the whole line is
// considered rewritten, and marking a "changed part" would just be noise.
const maxChangedShare = 0.7

// ChangedRanges finds the part of old and new that differs, by trimming their
// common prefix and suffix and widening the result to whole words. It reports
// ok=false when the lines are identical or mostly rewritten.
func ChangedRanges(old, new string) (oldRange, newRange Range, ok bool) {
	a, b := []rune(old), []rune(new)
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	oldRange = widenToWords(a, Range{prefix, len(a) - suffix})
	newRange = widenToWords(b, Range{prefix, len(b) - suffix})
	if oldRange.Empty() && newRange.Empty() {
		return oldRange, newRange, false
	}
	if share(oldRange, len(a)) > maxChangedShare || share(newRange, len(b)) > maxChangedShare {
		return oldRange, newRange, false
	}
	return oldRange, newRange, true
}

func share(r Range, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(r.End-r.Start) / float64(total)
}

// widenToWords extends r so it neither starts nor ends inside a word.
func widenToWords(s []rune, r Range) Range {
	for r.Start > 0 && r.Start <= len(s) && isWord(s[r.Start-1]) && (r.Start == len(s) || isWord(s[r.Start])) {
		r.Start--
	}
	for r.End < len(s) && r.End > 0 && isWord(s[r.End]) && isWord(s[r.End-1]) {
		r.End++
	}
	return r
}

func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

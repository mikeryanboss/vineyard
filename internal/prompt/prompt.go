// Package prompt renders the first prompt of a session started from a grapes
// issue, from a template the repository keeps (see config.LoadTemplates).
package prompt

import (
	"regexp"
	"strings"
	"text/template"

	"github.com/mikeryanboss/vineyard/internal/config"
)

// Issue names an issue and its status.
type Issue struct {
	ID     int
	Title  string
	Status string
}

// SubIssue is a sub-issue of the issue a prompt is for.
type SubIssue struct {
	Issue
	// Branch is the branch of a session working on it, or empty.
	Branch string
}

// Data is what a template can use.
type Data struct {
	Issue
	Labels []string
	// Parent is nil for a top-level issue.
	Parent    *Issue
	SubIssues []SubIssue
	// Blockers are the issues it is blocked by that are neither done nor
	// cancelled.
	Blockers []Issue
	// BuildSubIssues asks the agent to build the sub-issues no session works
	// on; the new-session dialog has a checkbox for it.
	BuildSubIssues bool
}

// DefaultTemplate names the template used when none is named after a label.
const DefaultTemplate = "default"

// Choose returns the index in templates of the one named after the first of
// labels that has one, else of the default template, else -1.
func Choose(templates []config.Template, labels []string) int {
	index := func(name string) int {
		for i, t := range templates {
			if t.Name == name {
				return i
			}
		}
		return -1
	}
	for _, label := range labels {
		if i := index(label); i >= 0 {
			return i
		}
	}
	return index(DefaultTemplate)
}

var blankLines = regexp.MustCompile(`\n{3,}`)

// Render executes the template text with data. Trailing spaces and repeated
// blank lines are removed, so templates need no whitespace control around
// their conditional paragraphs.
func Render(text string, data Data) (string, error) {
	t, err := template.New("prompt").Parse(text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")), nil
}

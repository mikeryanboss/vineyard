package prompt

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikeryanboss/vineyard/internal/config"
)

// examples returns the example templates a new repository starts with.
func examples(t *testing.T) []config.Template {
	t.Helper()
	dir := filepath.Join(t.TempDir(), config.DirName)
	if err := config.Prepare(dir); err != nil {
		t.Fatal(err)
	}
	templates, err := config.LoadTemplates(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	return templates
}

func render(t *testing.T, templates []config.Template, name string, data Data) string {
	t.Helper()
	for _, tmpl := range templates {
		if tmpl.Name == name {
			text, err := Render(tmpl.Text, data)
			if err != nil {
				t.Fatalf("rendering %s: %v", name, err)
			}
			return text
		}
	}
	t.Fatalf("no template %q", name)
	return ""
}

func TestChoose_PrefersTheFirstLabelWithATemplate(t *testing.T) {
	templates := examples(t)
	name := func(labels ...string) string {
		if i := Choose(templates, labels); i >= 0 {
			return templates[i].Name
		}
		return "none"
	}
	if got := name("tui", "bug", "research"); got != "bug" {
		t.Errorf("labels tui, bug, research chose %s, want bug", got)
	}
	if got := name("tui"); got != "default" {
		t.Errorf("label tui chose %s, want default", got)
	}
	if got := Choose(templates[:1], nil); got != -1 { // bug alone: no default
		t.Errorf("without a default template, Choose = %d, want -1", got)
	}
}

// A todo issue without relations gets the ownership and worktree paragraphs,
// and nothing about relations it does not have.
func TestExamples_PlainIssue(t *testing.T) {
	got := render(t, examples(t), "default", Data{Issue: Issue{ID: 7, Title: "Embed grapes", Status: "todo"}})
	want := "You own grapes issue #7: Embed grapes. Owning it means you keep its files current: status, acceptance criteria, and comment log. Read .grapes/7/, its specification and comments, before you start.\n\n" +
		"Vineyard created this worktree and branch for this issue. Work and commit here; do not create another worktree."
	if got != want {
		t.Errorf("prompt =\n%s\n\nwant\n%s", got, want)
	}
}

func TestExamples_StateTheIssuesSituation(t *testing.T) {
	data := Data{
		Issue:    Issue{ID: 31, Title: "Parse templates", Status: "in_progress"},
		Parent:   &Issue{ID: 30, Title: "Prompt templates", Status: "in_progress"},
		Blockers: []Issue{{ID: 29, Title: "Expose labels", Status: "todo"}},
		SubIssues: []SubIssue{
			{Issue: Issue{ID: 32, Title: "Load files", Status: "todo"}},
			{Issue: Issue{ID: 33, Title: "Pick by label", Status: "in_progress"}, Branch: "33/pick-by-label"},
		},
	}
	for _, name := range []string{"default", "bug", "research"} {
		got := render(t, examples(t), name, data)
		for _, want := range []string{
			"You own grapes issue #31: Parse templates.",
			"do not create another worktree",
			"It is already in progress.",
			"It is a sub-issue of #30: Prompt templates.",
			"- #29 Expose labels (todo)",
			"- #32 Load files (todo)\n- #33 Pick by label (in_progress): another session works on it in branch 33/pick-by-label; do not edit its files",
			"Do not build its sub-issues here",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s prompt lacks %q:\n%s", name, want, got)
			}
		}
		if strings.Contains(got, "\n\n\n") {
			t.Errorf("%s prompt has repeated blank lines:\n%q", name, got)
		}

		data.BuildSubIssues = true
		got = render(t, examples(t), name, data)
		data.BuildSubIssues = false
		if !strings.Contains(got, "Build the open sub-issues no other session works on yourself") || strings.Contains(got, "Do not build") {
			t.Errorf("%s prompt with BuildSubIssues should ask to build them:\n%s", name, got)
		}
	}
}

func TestExamples_SayWhatKindOfWork(t *testing.T) {
	data := Data{Issue: Issue{ID: 5, Title: "Crash", Status: "backlog"}}
	templates := examples(t)
	if got := render(t, templates, "bug", data); !strings.Contains(got, "It is a bug. Reproduce it first") {
		t.Errorf("bug prompt:\n%s", got)
	}
	if got := render(t, templates, "research", data); !strings.Contains(got, "It is a research issue") {
		t.Errorf("research prompt:\n%s", got)
	}
	if got := render(t, templates, "default", data); !strings.Contains(got, "Its status is backlog. Confirm with the user") {
		t.Errorf("a backlog issue should be confirmed first:\n%s", got)
	}
}

func TestRender_ReportsTemplateErrors(t *testing.T) {
	if _, err := Render("{{.Nope}}", Data{}); err == nil {
		t.Error("an unknown field should be an error")
	}
	if _, err := Render("{{if}}", Data{}); err == nil {
		t.Error("a malformed template should be an error")
	}
}

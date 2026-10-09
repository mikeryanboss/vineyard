package tui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Mibokess/grapes/embedded"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/dialog"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

// writeIssue writes a minimal grapes issue into checkout's .grapes directory.
func writeIssue(t *testing.T, checkout string, id int, title string) {
	t.Helper()
	dir := filepath.Join(checkout, ".grapes", fmt.Sprint(id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := fmt.Sprintf("title = %q\nstatus = 'todo'\npriority = 'medium'\n"+
		"created = 2026-01-01T00:00:00Z\nupdated = 2026-01-01T00:00:00Z\n", title)
	if err := os.WriteFile(filepath.Join(dir, "meta.toml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// appendMeta adds lines, such as labels or a parent, to issue id's meta.toml.
func appendMeta(t *testing.T, checkout string, id int, lines string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(checkout, ".grapes", fmt.Sprint(id), "meta.toml"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(lines); err != nil {
		t.Fatal(err)
	}
}

// exampleTemplates returns the prompt templates a new repository starts with.
func exampleTemplates(t *testing.T) []config.Template {
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

// withGrapes gives m the issue tracker in root's .grapes directory. Its Init is
// never run: the file watcher would block the test helpers.
func withGrapes(t *testing.T, m Model, root string) Model {
	t.Helper()
	g, err := embedded.New(filepath.Join(root, ".grapes"))
	if err != nil {
		t.Fatal(err)
	}
	m.grapes, m.grapesErr = g, nil
	m = update(m, tea.WindowSizeMsg{Width: 110, Height: 26})
	m.syncList()
	return m
}

// grapesModel returns a model whose repository has issues #7 and #8.
func grapesModel(t *testing.T, sessions []session.Session) Model {
	t.Helper()
	m, backend := newTestModel(t, sessions)
	backend.templates = exampleTemplates(t)
	root := t.TempDir()
	writeIssue(t, root, 7, "Embed grapes")
	writeIssue(t, root, 8, "Pick a session")
	return withGrapes(t, m, root)
}

func screen(m Model) string { return testutil.StripANSI(m.View().Content) }

func selectedID(m Model) string {
	s, _ := m.selected()
	return s.ID
}

func TestIssues_OpenAndCloseKeepsSelection(t *testing.T) {
	m := grapesModel(t, []session.Session{
		{ID: "a", Title: "fix login", Status: session.StatusReady},
		{ID: "b", Title: "add search", Status: session.StatusReady},
	})
	m = keys(m, "j", "i")

	if !m.issuesOpen || !strings.Contains(screen(m), "Embed grapes") {
		t.Fatalf("i should show the grapes board:\n%s", screen(m))
	}
	if strings.Contains(screen(m), "fix login") {
		t.Error("grapes should own the whole screen")
	}

	m = keys(m, "q")
	if m.issuesOpen {
		t.Fatal("q in grapes should return to the sessions screen")
	}
	if selectedID(m) != "b" {
		t.Errorf("selection = %q, want b", selectedID(m))
	}
}

func TestIssues_EnterOnTheIssueTabOpensGrapes(t *testing.T) {
	m := grapesModel(t, []session.Session{{ID: "a", Title: "fix login", Status: session.StatusReady}})
	m = keys(m, "tab", "tab", "enter")

	if !m.issuesOpen {
		t.Errorf("enter on the issue tab should open grapes, as i does:\n%s", screen(m))
	}
}

func TestIssues_UnavailableWithoutGrapes(t *testing.T) {
	m, _ := newTestModel(t, nil)
	m.grapesErr = fmt.Errorf("~/shop has no .grapes directory")

	m = keys(m, "i")

	if m.issuesOpen {
		t.Error("i opened an issues screen without grapes")
	}
	if !strings.Contains(m.status, "has no .grapes directory") {
		t.Errorf("status = %q, want the reason", m.status)
	}
}

// Grapes' commands report back through vineyard's update loop: a write, then
// the reload it triggers. Vineyard must hand those results to grapes, or
// grapes never sees its own changes.
func TestIssues_GrapesSeesItsOwnWrites(t *testing.T) {
	m, _ := newTestModel(t, []session.Session{{ID: "a", Title: "embed", Status: session.StatusReady, Issue: 7}})
	root := t.TempDir()
	writeIssue(t, root, 7, "Embed grapes")
	m = withGrapes(t, m, root)

	m = keys(m, "i", "s") // #7's detail, then its status picker
	m = keys(m, "j", "enter")

	meta, err := os.ReadFile(filepath.Join(root, ".grapes", "7", "meta.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(meta), "in_progress") {
		t.Fatalf("status change was not written:\n%s", meta)
	}
	// Grapes reloads when its file watcher reports the write. The watcher has
	// buffered the event since embedded.New; starting grapes delivers it.
	for _, msg := range collect(m.grapes.Init()) {
		m = send(m, msg)
	}
	if issue, _ := m.grapes.Issue(7); issue.Status != "in_progress" {
		t.Errorf("grapes shows #7 as %q after reloading, want in_progress", issue.Status)
	}
}

func TestIssues_SessionsKeyWithNoSessionStartsOne(t *testing.T) {
	m := grapesModel(t, []session.Session{{ID: "a", Title: "fix login", Status: session.StatusReady}})
	m = keys(m, "i")

	m = send(m, embedded.SessionsMsg{IssueID: 7})
	if _, ok := m.dialog.(*dialog.NewSession); !ok {
		t.Fatalf("dialog = %T, want the new-session dialog", m.dialog)
	}
	view := screen(m)
	if !strings.Contains(view, "New session for #7") || !strings.Contains(view, "Embed grapes") {
		t.Errorf("dialog should be filled in for #7:\n%s", view)
	}

	m = keys(m, "enter", "enter")
	i := m.find("embed-grapes")
	if i < 0 {
		t.Fatalf("no session created: %+v", m.sessions)
	}
	s := m.sessions[i]
	if s.Issue != 7 || !strings.HasPrefix(s.PendingPrompt, "You own grapes issue #7: Embed grapes.") {
		t.Errorf("session = %+v, want issue 7 and the default template's prompt", s)
	}
	if m.issuesOpen || selectedID(m) != s.ID {
		t.Error("the new session should be shown and selected")
	}
}

func TestIssues_SessionsKeyJumpsToTheOnlySession(t *testing.T) {
	m := grapesModel(t, []session.Session{
		{ID: "a", Title: "embed", Status: session.StatusReady, Issue: 7},
		{ID: "b", Title: "other", Status: session.StatusReady},
	})
	m = keys(m, "j", "i")

	m = send(m, embedded.SessionsMsg{IssueID: 7})

	if m.issuesOpen || m.dialog != nil || selectedID(m) != "a" {
		t.Errorf("open=%v dialog=%T selected=%q; want session a selected", m.issuesOpen, m.dialog, selectedID(m))
	}
}

func TestIssues_SessionsKeyPicksAmongSeveral(t *testing.T) {
	m := grapesModel(t, []session.Session{
		{ID: "a", Title: "embed first try", Status: session.StatusReady, Issue: 7},
		{ID: "b", Title: "other", Status: session.StatusReady},
		{ID: "c", Title: "embed second try", Status: session.StatusReady, Issue: 7},
	})
	m = keys(m, "i")
	m = send(m, embedded.SessionsMsg{IssueID: 7})
	if _, ok := m.dialog.(*dialog.Pick); !ok {
		t.Fatalf("dialog = %T, want a picker", m.dialog)
	}
	view := screen(m)
	if !strings.Contains(view, "embed first try") || !strings.Contains(view, "embed second try") || strings.Contains(view, "other") {
		t.Errorf("picker should list exactly the sessions on #7:\n%s", view)
	}

	cancelled := keys(m, "esc")
	if cancelled.dialog != nil || !cancelled.issuesOpen {
		t.Error("esc should close the picker and stay in grapes")
	}

	m = keys(m, "j", "enter")
	if m.issuesOpen || selectedID(m) != "c" {
		t.Errorf("open=%v selected=%q; want session c", m.issuesOpen, selectedID(m))
	}
}

func TestIssues_SessionOpensItsIssue(t *testing.T) {
	m := grapesModel(t, []session.Session{{ID: "a", Title: "embed", Status: session.StatusReady, Issue: 8}})

	if !strings.Contains(screen(m), "#8") {
		t.Errorf("the list should tag the session with #8:\n%s", screen(m))
	}
	m = keys(m, "i")
	if !m.issuesOpen || !strings.Contains(screen(m), "Pick a session") || strings.Contains(screen(m), "Embed grapes") {
		t.Errorf("i should open grapes on #8's detail:\n%s", screen(m))
	}
}

// A session counts as working on every issue its branch changed, recorded or
// not. With several, i asks which one to open.
func TestIssues_TouchedIssuesLinkSessions(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(root, "init", "-q", "-b", "main")
	git(root, "config", "user.email", "test@example.com")
	git(root, "config", "user.name", "Test")
	writeIssue(t, root, 7, "Embed grapes")
	writeIssue(t, root, 8, "Pick a session")
	git(root, "add", "-A")
	git(root, "commit", "-q", "-m", "issues")
	worktree := filepath.Join(t.TempDir(), "agent")
	git(root, "worktree", "add", "-q", "-b", "agent", worktree)
	writeIssue(t, worktree, 8, "Pick a session, in progress")

	m, _ := newTestModel(t, []session.Session{
		{ID: "a", Title: "embed", Status: session.StatusReady, Issue: 7, WorktreePath: worktree},
	})
	m = withGrapes(t, m, root)

	if got := m.issuesOf(m.sessions[0]); !slices.Equal(got, []int{7, 8}) {
		t.Errorf("issuesOf = %v, want [7 8]", got)
	}
	m = keys(m, "i")
	if _, ok := m.dialog.(*dialog.Pick); !ok {
		t.Fatalf("dialog = %T, want a picker of the session's issues", m.dialog)
	}
	m = keys(m, "enter")
	if !m.issuesOpen || !strings.Contains(screen(m), "Embed grapes") {
		t.Errorf("choosing #7 should open its detail:\n%s", screen(m))
	}

	m = send(m, embedded.SessionsMsg{IssueID: 8})
	if m.issuesOpen || selectedID(m) != "a" {
		t.Error("a session that only touched #8 should still be found for #8")
	}
}

// Vineyard starts light until the terminal reports its background, and grapes
// must too, or a terminal that never reports gets pale grapes text on a light
// background.
func TestIssues_GrapesStartsLightLikeVineyard(t *testing.T) {
	m, _ := newTestModel(t, nil)
	root := t.TempDir()
	writeIssue(t, root, 7, "Embed grapes")
	g, err := embedded.New(filepath.Join(root, ".grapes"))
	if err != nil {
		t.Fatal(err)
	}
	m = NewModel(m.backend, nil, Options{Grapes: &g})
	m = keys(update(m, tea.WindowSizeMsg{Width: 110, Height: 26}), "i")

	initial := m.View().Content
	light := update(m, tea.BackgroundColorMsg{Color: color.White}).View().Content
	dark := update(m, tea.BackgroundColorMsg{Color: color.Black}).View().Content
	if initial != light {
		t.Error("grapes should start in the light theme")
	}
	if light == dark {
		t.Error("the theme does not show in grapes' view; this test proves nothing")
	}
}

// The issue tab shows the selected session's issue as grapes renders it, and
// follows the selection.
func TestIssueTab_ShowsTheSelectedSessionsIssue(t *testing.T) {
	m := grapesModel(t, []session.Session{
		{ID: "a", Title: "embed", Status: session.StatusReady, Issue: 7},
		{ID: "b", Title: "pick", Status: session.StatusReady, Issue: 8},
		{ID: "c", Title: "loose", Status: session.StatusReady},
	})

	m = keys(m, "tab", "tab")
	if m.tab != tabIssue {
		t.Fatalf("two tabs from preview should reach the issue tab, got %v", m.tab)
	}
	if got := screen(m); !strings.Contains(got, "Issue #7") || !strings.Contains(got, "Embed grapes") {
		t.Fatalf("the issue tab should show #7:\n%s", got)
	}

	m = keys(m, "j")
	if got := screen(m); !strings.Contains(got, "Pick a session") || strings.Contains(got, "Embed grapes") {
		t.Errorf("selecting b should show #8 instead of #7:\n%s", got)
	}
	m = keys(m, "j")
	if got := screen(m); !strings.Contains(got, "No issue linked") {
		t.Errorf("a session without an issue should say so:\n%s", got)
	}

	m = keys(m, "tab")
	if m.tab != tabRecap {
		t.Errorf("tab from the issue tab should reach the recap tab, got %v", m.tab)
	}
}

// The issue tab renders only when something changes, so a grapes reload must
// re-render it, or it keeps showing the issue as it was.
func TestIssueTab_FollowsGrapesReloads(t *testing.T) {
	m, _ := newTestModel(t, []session.Session{{ID: "a", Title: "embed", Status: session.StatusReady, Issue: 7}})
	root := t.TempDir()
	writeIssue(t, root, 7, "Embed grapes")
	m = withGrapes(t, m, root)
	m = keys(m, "tab", "tab")

	writeIssue(t, root, 7, "Embed grapes as a tab")
	// The watcher has buffered the write since embedded.New; starting grapes
	// delivers it, and the reload follows.
	for _, msg := range collect(m.grapes.Init()) {
		m = send(m, msg)
	}
	if got := screen(m); !strings.Contains(got, "Embed grapes as a tab") {
		t.Errorf("the issue tab should show the reloaded title:\n%s", got)
	}
}

// The prompt describes the issue as grapes and Vineyard know it: its label
// picks the template, and a sub-issue shows the branch of its session.
func TestIssues_PromptDescribesTheIssue(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{
		{ID: "b", Title: "pick", Status: session.StatusReady, Issue: 8, Branch: "8/pick"},
	})
	backend.templates = exampleTemplates(t)
	root := t.TempDir()
	writeIssue(t, root, 7, "Embed grapes")
	writeIssue(t, root, 8, "Pick a session")
	writeIssue(t, root, 9, "Fix the header")
	appendMeta(t, root, 7, "labels = ['bug']\n")
	appendMeta(t, root, 8, "parent = 7\n")
	appendMeta(t, root, 9, "parent = 7\n")
	meta := filepath.Join(root, ".grapes", "9", "meta.toml")
	content, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(meta, []byte(strings.Replace(string(content), "'todo'", "'done'", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	m = withGrapes(t, m, root)

	m = send(m, embedded.SessionsMsg{IssueID: 7})
	view := screen(m)
	for _, want := range []string{"‹ bug ›", "[ ] Build sub-issues", "You own grapes issue #7"} {
		if !strings.Contains(view, want) {
			t.Errorf("dialog lacks %q:\n%s", want, view)
		}
	}
	m = keys(m, "tab", "tab", "space", "enter") // tick Build sub-issues, create
	i := m.find("embed-grapes")
	if i < 0 {
		t.Fatalf("no session created:\n%s", screen(m))
	}
	got := m.sessions[i].PendingPrompt
	for _, want := range []string{
		"It is a bug. Reproduce it first",
		"- #8 Pick a session (todo): another session works on it in branch 8/pick; do not edit its files\n- #9 Fix the header (done)",
		"Build the open sub-issues no other session works on yourself",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt lacks %q:\n%s", want, got)
		}
	}
}

// startedAt builds a model the way main does for vineyard --issue id.
func startedAt(t *testing.T, id int, sessions []session.Session) Model {
	t.Helper()
	root := t.TempDir()
	writeIssue(t, root, 7, "Embed grapes")
	g, err := embedded.New(filepath.Join(root, ".grapes"))
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(&fakeBackend{templates: exampleTemplates(t)}, sessions, Options{Grapes: &g, Issue: id})
}

func TestStartIssue_OpensTheNewSessionDialogOnceSized(t *testing.T) {
	m := startedAt(t, 7, nil)
	if m.dialog != nil {
		t.Fatal("the dialog needs the window's width; it must wait for the first size")
	}

	m = send(m, tea.WindowSizeMsg{Width: 110, Height: 26})
	if _, ok := m.dialog.(*dialog.NewSession); !ok || !strings.Contains(screen(m), "New session for #7") {
		t.Fatalf("dialog = %T, want the new-session dialog for #7:\n%s", m.dialog, screen(m))
	}

	m = keys(m, "esc")
	m = update(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	if m.dialog != nil {
		t.Error("a later resize must not reopen the dialog")
	}
}

func TestStartIssue_JumpsToTheIssuesSession(t *testing.T) {
	m := startedAt(t, 7, []session.Session{
		{ID: "a", Title: "other", Status: session.StatusReady},
		{ID: "b", Title: "embed", Status: session.StatusReady, Issue: 7},
	})
	m = update(m, tea.WindowSizeMsg{Width: 110, Height: 26})
	if m.dialog != nil || selectedID(m) != "b" {
		t.Errorf("dialog=%T selected=%q; want session b selected", m.dialog, selectedID(m))
	}
}

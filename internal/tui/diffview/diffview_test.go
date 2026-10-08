package diffview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/diff"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

const sampleDiff = `diff --git a/internal/auth/login.go b/internal/auth/login.go
index 1111111..2222222 100644
--- a/internal/auth/login.go
+++ b/internal/auth/login.go
@@ -8,7 +8,8 @@ func Login(w http.ResponseWriter, r *http.Request) {
 	user, err := lookup(r)
 	if err != nil {
-		http.Redirect(w, r, "/login", http.StatusFound)
+		target := "/login?next=" + url.QueryEscape(r.URL.Path)
+		http.Redirect(w, r, target, http.StatusFound)
 		return
 	}
-	session.Start(w, user)
+	session.Start(w, user, session.Remember(true))
 }
diff --git a/docs/auth.md b/docs/auth.md
new file mode 100644
--- /dev/null
+++ b/docs/auth.md
@@ -0,0 +1,3 @@
+# Authentication
+
+Logins now redirect back to the page that asked for them, which is a long sentence that has to wrap.
diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
`

func newView(width, height int) Model {
	return New(common.NewTheme(true)).SetSize(width, height).SetDiff(sampleDiff)
}

func TestDiffView_Default(t *testing.T) {
	testutil.RequireGolden(t, newView(72, 40).View())
}

func TestDiffView_Narrow(t *testing.T) {
	testutil.RequireGolden(t, newView(44, 44).View())
}

func TestDiffView_Empty(t *testing.T) {
	testutil.RequireGolden(t, New(common.NewTheme(true)).SetSize(40, 5).SetDiff("").View())
}

func TestDiffView_FileNavigation(t *testing.T) {
	m := newView(72, 10)
	if m.CurrentFile() != -1 {
		t.Fatalf("view should start on the summary, current file = %d", m.CurrentFile())
	}
	for want := 0; want < 3; want++ {
		m, _ = m.Update(testutil.Key("]"))
		if got := m.CurrentFile(); got != want {
			t.Fatalf("after %d next-file presses current file = %d, want %d", want+1, got, want)
		}
	}
	m, _ = m.Update(testutil.Key("["))
	if got := m.CurrentFile(); got != 1 {
		t.Errorf("prev file from file 2 landed on %d", got)
	}
}

func TestDiffView_UnchangedDiffKeepsScroll(t *testing.T) {
	m := newView(72, 10)
	m, _ = m.Update(testutil.Key("]"))
	m, _ = m.Update(testutil.Key("]"))
	before := m.CurrentFile()
	m = m.SetDiff(sampleDiff)
	if m.CurrentFile() != before {
		t.Error("refreshing with the same diff must not move the view")
	}
}

func TestDiffView_MarksChangedWords(t *testing.T) {
	view := newView(72, 40).View()
	theme := common.NewTheme(true)
	emph := "48;2;31;81;48" // AddEmph #1f5130 as a truecolor background
	if !strings.Contains(view, emph) {
		t.Errorf("expected the added word to use the emphasis background %v", theme.ColorAddEmph)
	}
}

func TestEmphasize_SplitsAtRangeEdges(t *testing.T) {
	spans := []span{{text: "hello "}, {text: "world"}}
	got := emphasize(spans, diff.Range{Start: 3, End: 8})
	var b strings.Builder
	for _, s := range got {
		if s.emph {
			b.WriteString("[" + s.text + "]")
		} else {
			b.WriteString(s.text)
		}
	}
	if b.String() != "hel[lo ][wo]rld" {
		t.Errorf("emphasize = %q", b.String())
	}
}

func TestWrapSpans_BreaksWideTextAcrossRows(t *testing.T) {
	rows := wrapSpans([]span{{text: "abcd"}, {text: "efg"}}, 3)
	var got []string
	for _, row := range rows {
		var b strings.Builder
		for _, s := range row {
			b.WriteString(s.text)
		}
		got = append(got, b.String())
	}
	if strings.Join(got, "|") != "abc|def|g" {
		t.Errorf("rows = %q", got)
	}
}

func TestExpandTabs_UsesTabStops(t *testing.T) {
	if got := expandTabs("a\tb\t\tc"); got != "a   b       c" {
		t.Errorf("expandTabs = %q", got)
	}
}

func TestDiffView_ToggleFoldsCurrentFile(t *testing.T) {
	m := newView(72, 10)
	m, _ = m.Update(testutil.Key("]"))
	full := len(m.lines)
	m, _ = m.Update(testutil.Key("enter"))
	if len(m.lines) >= full {
		t.Fatalf("folding file 1 left %d lines, was %d", len(m.lines), full)
	}
	if m.CurrentFile() != 0 {
		t.Errorf("folded file should stay at the top, current = %d", m.CurrentFile())
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "▸ M internal/auth/login.go") {
		t.Errorf("folded header should show ▸:\n%s", view)
	}
	m, _ = m.Update(testutil.Key("enter"))
	if len(m.lines) != full {
		t.Errorf("unfolding restored %d lines, want %d", len(m.lines), full)
	}
}

func TestDiffView_CollapseAndExpandAll(t *testing.T) {
	m := newView(72, 40)
	full := len(m.lines)
	m, _ = m.Update(testutil.Key("c"))
	if want := len(m.Files()) * 4; len(m.lines)-len(renderSummary(m.files, 72, m.theme)) != want {
		t.Errorf("collapsed lines = %d, want summary + %d (rule, header, rule, blank per file)", len(m.lines), want)
	}
	m, _ = m.Update(testutil.Key("e"))
	if len(m.lines) != full {
		t.Errorf("expand all restored %d lines, want %d", len(m.lines), full)
	}
}

func TestDiffView_FoldSurvivesNewDiff(t *testing.T) {
	m := newView(72, 40)
	m, _ = m.Update(testutil.Key("c"))
	m = m.SetDiff(sampleDiff + "diff --git a/x.txt b/x.txt\nnew file mode 100644\n--- /dev/null\n+++ b/x.txt\n@@ -0,0 +1 @@\n+hi\n")
	view := ansi.Strip(strings.Join(m.lines, "\n"))
	if !strings.Contains(view, "▸ M internal/auth/login.go") {
		t.Error("a file folded before a refresh should stay folded")
	}
	if !strings.Contains(view, "▾ A x.txt") {
		t.Error("collapse all folds files present at the time only; x.txt arrived later and should be expanded")
	}
}

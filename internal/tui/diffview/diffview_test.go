package diffview

import (
	"strings"
	"testing"

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

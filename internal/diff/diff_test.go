package diff

import (
	"testing"
)

const sample = `diff --git a/app.go b/app.go
index 1111111..2222222 100644
--- a/app.go
+++ b/app.go
@@ -1,4 +1,5 @@ package main
 package main
-func old() {}
+func renamed() {}
+-- a removed-looking added line

 // end
diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+one
+two
\ No newline at end of file
diff --git a/gone.txt b/gone.txt
deleted file mode 100644
--- a/gone.txt
+++ /dev/null
@@ -1 +0,0 @@
--- looks like a header
diff --git a/old name.go b/new name.go
similarity index 90%
rename from old name.go
rename to new name.go
diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
`

func TestParse_Files(t *testing.T) {
	files := Parse(sample)
	if len(files) != 5 {
		t.Fatalf("parsed %d files, want 5", len(files))
	}
	want := []struct {
		path           string
		status         FileStatus
		added, removed int
		binary         bool
	}{
		{"app.go", Modified, 2, 1, false},
		{"new.txt", Added, 2, 0, false},
		{"gone.txt", Deleted, 0, 1, false},
		{"new name.go", Renamed, 0, 0, false},
		{"logo.png", Modified, 0, 0, true},
	}
	for i, w := range want {
		f := files[i]
		if f.Path() != w.path || f.Status != w.status || f.Added != w.added || f.Removed != w.removed || f.Binary != w.binary {
			t.Errorf("file %d = %q status %d +%d -%d binary %v; want %+v", i, f.Path(), f.Status, f.Added, f.Removed, f.Binary, w)
		}
	}
	if files[3].OldPath != "old name.go" {
		t.Errorf("rename source = %q", files[3].OldPath)
	}
}

func TestParse_LineNumbersAndHeaderLookalikes(t *testing.T) {
	files := Parse(sample)
	h := files[0].Hunks[0]
	if h.Section != "package main" {
		t.Errorf("section = %q", h.Section)
	}
	got := []Line{}
	for _, l := range h.Lines {
		got = append(got, Line{Kind: l.Kind, OldNo: l.OldNo, NewNo: l.NewNo})
	}
	want := []Line{
		{Kind: Context, OldNo: 1, NewNo: 1},
		{Kind: Delete, OldNo: 2},
		{Kind: Add, NewNo: 2},
		{Kind: Add, NewNo: 3},
		{Kind: Context, OldNo: 3, NewNo: 4},
		{Kind: Context, OldNo: 4, NewNo: 5},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(got), len(want), h.Lines)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if h.Lines[3].Text != "-- a removed-looking added line" {
		t.Errorf("added line text = %q", h.Lines[3].Text)
	}
	if deleted := files[2].Hunks[0].Lines[0]; deleted.Kind != Delete || deleted.Text != "-- looks like a header" {
		t.Errorf("deleted line = %+v; a removed line must not be read as a file header", deleted)
	}
	if !files[1].Hunks[0].Lines[1].NoNewline {
		t.Error("missing-newline marker not attached to the last line")
	}
}

func TestParse_QuotedPaths(t *testing.T) {
	files := Parse("diff --git \"a/caf\\303\\251.txt\" \"b/caf\\303\\251.txt\"\n--- \"a/caf\\303\\251.txt\"\n+++ \"b/caf\\303\\251.txt\"\n@@ -1 +1 @@\n-a\n+b\n")
	if len(files) != 1 || files[0].Path() != "café.txt" {
		t.Fatalf("files = %+v", files)
	}
}

func TestParse_Empty(t *testing.T) {
	if files := Parse(""); len(files) != 0 {
		t.Errorf("Parse(\"\") = %+v", files)
	}
}

func TestPairs_MatchesRunsInOrder(t *testing.T) {
	lines := []Line{
		{Kind: Context}, {Kind: Delete}, {Kind: Delete}, {Kind: Add}, {Kind: Add}, {Kind: Add},
		{Kind: Context}, {Kind: Delete}, {Kind: Context}, {Kind: Add},
	}
	got := Pairs(lines)
	want := [][2]int{{1, 3}, {2, 4}}
	if len(got) != len(want) {
		t.Fatalf("pairs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pair %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestChangedRanges(t *testing.T) {
	cases := []struct {
		old, new   string
		oldR, newR Range
		ok         bool
		name       string
	}{
		{name: "word swap", old: "return userName, nil", new: "return accountName, nil", oldR: Range{7, 15}, newR: Range{7, 18}, ok: true},
		{name: "insertion", old: "call(a, b)", new: "call(a, b, c)", oldR: Range{9, 9}, newR: Range{9, 12}, ok: true},
		{name: "identical", old: "same", new: "same", ok: false},
		{name: "rewritten", old: "alpha", new: "omega", ok: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oldR, newR, ok := ChangedRanges(c.old, c.new)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v (ranges %v %v)", ok, c.ok, oldR, newR)
			}
			if ok && (oldR != c.oldR || newR != c.newR) {
				t.Errorf("ranges = %v %v, want %v %v", oldR, newR, c.oldR, c.newR)
			}
		})
	}
}

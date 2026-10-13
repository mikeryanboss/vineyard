package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A log past maxSize starts afresh, keeping the previous one beside it, so
// logs never grow without bound.
func TestOpen_StartsALargeLogAfresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("old line\n", maxSize/9+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("first\nsecond\nthird\n")
	f.Close()
	if end, err := Tail(path, 2); err != nil || end != "second\nthird" {
		t.Errorf("Tail = %q, %v; want the new log's last two lines", end, err)
	}
	if end, err := Tail(path+".old", 1); err != nil || end != "old line" {
		t.Errorf("old log ends %q, %v; want the previous log", end, err)
	}
	if end, err := Tail(filepath.Join(t.TempDir(), "none.log"), 2); err != nil || end != "" {
		t.Errorf("Tail of a missing log = %q, %v; want empty", end, err)
	}
}

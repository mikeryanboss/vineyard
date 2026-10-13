package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/vterm"
)

func TestRun_CommandsAndUsageErrors(t *testing.T) {
	cases := []struct {
		args     []string
		code     int
		inStdout string
		inStderr string
	}{
		{args: []string{"version"}, code: 0, inStdout: version},
		{args: []string{"help"}, code: 0, inStdout: "USAGE"},
		{args: []string{"-h"}, code: 0, inStdout: "USAGE"},
		{args: []string{"bogus"}, code: 2, inStderr: "Unknown command: bogus"},
		{args: []string{"version", "extra"}, code: 2, inStderr: "does not accept arguments"},
		{args: []string{"--nope"}, code: 2, inStderr: "flag provided but not defined"},
		{args: []string{"--issue", "x"}, code: 2, inStderr: "invalid value"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(c.args, &stdout, &stderr)
			if code != c.code {
				t.Errorf("exit code = %d, want %d", code, c.code)
			}
			if !strings.Contains(stdout.String(), c.inStdout) || !strings.Contains(stderr.String(), c.inStderr) {
				t.Errorf("stdout %q / stderr %q missing %q / %q", stdout.String(), stderr.String(), c.inStdout, c.inStderr)
			}
		})
	}
}

// Only go install builds take their version from Go: a checkout build may be
// stamped with another worktree's commit.
func TestModuleVersion_OnlyForGoInstall(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"go install", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.5"}}, "0.1.5"},
		{"checkout", &debug.BuildInfo{
			Main:     debug.Module{Version: "v0.1.4+dirty"},
			Settings: []debug.BuildSetting{{Key: "vcs", Value: "git"}},
		}, "dev"},
		{"go run", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"no build info", nil, "dev"},
	}
	for _, c := range cases {
		if got := moduleVersion(c.info); got != c.want {
			t.Errorf("%s: moduleVersion = %q, want %q", c.name, got, c.want)
		}
	}
}

// debug reports where a repository's data lives without creating it.
func TestRun_DebugPrintsPaths(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(repo)
	t.Setenv("TMPDIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := run([]string{"debug"}, &stdout, &stderr); code != 0 {
		t.Fatalf("debug exit code %d: %s", code, stderr.String())
	}
	for _, want := range []string{
		"not running",
		filepath.Join(repo, ".vineyard", "config.toml"),
		filepath.Join(repo, ".vineyard", "worktrees"),
		filepath.Join(repo, ".vineyard", "vineyard.log"),
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("debug output lacks %s:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".vineyard")); !os.IsNotExist(err) {
		t.Error("debug created .vineyard")
	}
}

// Vineyard's server is always available, tmux only when it is installed.
func TestMultiplexers_WithoutTmux(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	server, err := vineyardServer()
	if err != nil {
		t.Fatal(err)
	}
	terminals := multiplexers(server)
	if _, ok := terminals[config.MultiplexerVineyard].(*vterm.Client); !ok || len(terminals) != 1 {
		t.Errorf("multiplexers = %v, want Vineyard's server alone", terminals)
	}
}

// A Vineyard of another protocol than the server's cannot reach the agents
// on it, so it refuses to start while this repository's sessions may run
// there, instead of showing them as stopped.
func TestCheckServer(t *testing.T) {
	other := &vterm.ServerInfo{PID: 4242, Protocol: vterm.Protocol() - 1}
	on := func(multiplexer string, status session.Status) []session.Session {
		return []session.Session{{Title: "fix", Multiplexer: multiplexer, Status: status}}
	}
	for _, tc := range []struct {
		name     string
		info     *vterm.ServerInfo
		sessions []session.Session
		refuse   bool
	}{
		{"no server", nil, on("vineyard", session.StatusRunning), false},
		{"same protocol", &vterm.ServerInfo{PID: 4242, Protocol: vterm.Protocol()}, on("vineyard", session.StatusRunning), false},
		{"running session", other, on("vineyard", session.StatusRunning), true},
		{"ready session", other, on("vineyard", session.StatusReady), true},
		{"paused session", other, on("vineyard", session.StatusPaused), false},
		{"stopped session", other, on("vineyard", session.StatusStopped), false},
		{"tmux session", other, on("tmux", session.StatusRunning), false},
	} {
		err := checkServer(tc.info, tc.sessions)
		if !tc.refuse {
			if err != nil {
				t.Errorf("%s: %v, want none", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: no error", tc.name)
			continue
		}
		for _, want := range []string{"process 4242", `"fix"`, "kill 4242"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %q lacks %q", tc.name, err, want)
			}
		}
	}
}

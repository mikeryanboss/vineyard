// Command vineyard runs coding agents side by side, each in its own terminal
// and git worktree, and shows them in one terminal UI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Mibokess/grapes/embedded"
	"github.com/charmbracelet/x/term"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tmux"
	"github.com/mikeryanboss/vineyard/internal/tui"
	"github.com/mikeryanboss/vineyard/internal/vterm"
)

// version is the release version. GoReleaser stamps it in with -X, which sets
// only a string variable without a computed initial value; other builds get
// it from init.
var version string

func init() {
	if version == "" {
		info, _ := debug.ReadBuildInfo()
		version = moduleVersion(info)
	}
}

// moduleVersion is the version of a build without GoReleaser: the module
// version for go install, and "dev" for a build in a checkout. Go stamps a
// checkout build with the commit of the nearest .git directory, which for a
// worktree inside the main checkout is the main checkout's.
func moduleVersion(info *debug.BuildInfo) string {
	if info == nil || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs" {
			return "dev"
		}
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("vineyard", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	program := flags.String("p", "", "")
	flags.StringVar(program, "program", "", "")
	autoYes := flags.Bool("y", false, "")
	flags.BoolVar(autoYes, "autoyes", false, "")
	issue := flags.Int("i", 0, "")
	flags.IntVar(issue, "issue", 0, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeHelp(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "%v\n\n", err)
		writeHelp(stderr)
		return 2
	}

	rest := flags.Args()
	if len(rest) == 2 && rest[0] == "server" {
		// Vineyard starts its terminal server this way; see vterm.Client.
		if err := vterm.Serve(rest[1]); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "%s does not accept arguments\n\n", rest[0])
		writeHelp(stderr)
		return 2
	}
	if len(rest) == 1 {
		switch rest[0] {
		case "help":
			writeHelp(stdout)
			return 0
		case "version":
			fmt.Fprintln(stdout, version)
			return 0
		case "debug":
			return runDebug(stdout, stderr)
		default:
			fmt.Fprintf(stderr, "Unknown command: %s\n\n", rest[0])
			writeHelp(stderr)
			return 2
		}
	}

	if err := runTUI(*program, *autoYes, *issue); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func runTUI(program string, autoYes bool, issue int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repo, err := git.FindRepo(cwd)
	if err != nil {
		return err
	}
	dir := config.Dir(repo.Root)
	if err := config.Prepare(dir); err != nil {
		return err
	}
	cfg, cfgErr := config.Load(dir)

	store := session.NewStore(dir)
	worktreeDir, err := config.ResolveWorktreeDir(repo.Root, cfg.WorktreeDir)
	if err != nil {
		return fmt.Errorf("worktree directory: %w", err)
	}
	leader := session.NewLeader(dir)
	terminals, err := multiplexers()
	if err != nil {
		return err
	}
	managed := map[string]session.Terminal{}
	for name, t := range terminals {
		managed[name] = t
	}
	manager := session.NewManager(repo, managed, cfg.Multiplexer, config.ProjectName(repo.Root))
	sessions, err := store.Update(func(saved []session.Session) ([]session.Session, error) {
		return manager.Restore(saved, worktreeDir)
	})
	if err != nil {
		return err
	}

	grapes, grapesErr := loadGrapes(repo.Root)
	if issue != 0 {
		if grapesErr != nil {
			return fmt.Errorf("--issue: %w", grapesErr)
		}
		if _, ok := grapes.Issue(issue); !ok {
			return fmt.Errorf("no grapes issue #%d", issue)
		}
	}

	opts := tui.Options{
		Config:   cfg,
		RepoName: repo.Name(),
		RepoRoot: repo.Root,
		Version:  version,
		AutoYes:  autoYes,
		Program:  program,
		// The config screen shows where it saves.
		ConfigPath: displayPath(config.Path(dir)),
		ConfigErr:  cfgErr,
		Grapes:     grapes,
		GrapesErr:  grapesErr,
		Issue:      issue,
	}
	model := tui.NewModel(tui.LiveBackend{Manager: manager, Terminals: terminals, Store: store, Leader: leader, Transcripts: subagent.NewReader(), Dir: dir}, sessions, opts)

	in, out, closeTTY, err := terminal()
	if err != nil {
		return err
	}
	defer closeTTY()
	_, err = tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

// loadGrapes loads the grapes issue tracker of the repository at root, from
// the main checkout's .grapes directory, where grapes keeps the canonical
// copies. Grapes finds every worktree's copies from there.
func loadGrapes(root string) (*embedded.Model, error) {
	dir := filepath.Join(root, ".grapes")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s has no .grapes directory", displayPath(root))
	} else if err != nil {
		return nil, err
	}
	m, err := embedded.New(dir)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// terminal returns the files the TUI reads and draws on. Standard input and
// output are preferred whenever they are terminals: attached tmux clients
// inherit these files, and tmux refuses to run on a descriptor opened through
// /dev/tty ("can't use /dev/tty"), which is what tea.OpenTTY returns. OpenTTY
// remains the fallback for when standard streams are redirected.
func terminal() (in, out *os.File, closeTTY func(), err error) {
	if term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		return os.Stdin, os.Stdout, func() {}, nil
	}
	in, out, err = tea.OpenTTY()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("vineyard needs an interactive terminal: %w", err)
	}
	return in, out, func() {
		in.Close()
		if out != in {
			out.Close()
		}
	}, nil
}

// multiplexers returns the multiplexers available to run sessions, by name:
// Vineyard's server, and tmux when it is installed.
func multiplexers() (map[string]tui.Terminal, error) {
	server, err := vineyardServer()
	if err != nil {
		return nil, err
	}
	terminals := map[string]tui.Terminal{config.MultiplexerVineyard: server}
	if _, err := exec.LookPath("tmux"); err == nil {
		terminals[config.MultiplexerTmux] = tmux.New(socketName())
	}
	return terminals, nil
}

// vineyardServer returns a client of Vineyard's own terminal server, which
// this executable runs.
func vineyardServer() (*vterm.Client, error) {
	socket, err := vterm.SocketPath(socketName())
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return vterm.NewClient(socket, exe, "server", socket), nil
}

// socketName names the sockets of the multiplexers, tmux's and Vineyard's own
// server's. VINEYARD_SOCKET overrides it, which keeps
// development runs away from real sessions.
func socketName() string {
	if socket := os.Getenv("VINEYARD_SOCKET"); socket != "" {
		return socket
	}
	return tmux.DefaultSocket
}

func runDebug(stdout, stderr io.Writer) int {
	fmt.Fprintf(stdout, "version:     %s\n", version)
	if _, err := exec.LookPath("tmux"); err != nil {
		fmt.Fprintf(stdout, "tmux socket: none, tmux not found\n")
	} else {
		fmt.Fprintf(stdout, "tmux socket: %s (tmux -L %s ls)\n", socketName(), socketName())
	}
	if server, err := vineyardServer(); err == nil {
		fmt.Fprintf(stdout, "server:      %s\n", server.Socket())
	} else {
		fmt.Fprintf(stdout, "server:      %v\n", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	repo, err := git.FindRepo(cwd)
	if err != nil {
		fmt.Fprintf(stdout, "repository:  none (%v)\n", err)
		return 0
	}
	dir := config.Dir(repo.Root)
	cfg, cfgErr := config.Load(dir)
	fmt.Fprintf(stdout, "repository:  %s\n", repo.Root)
	fmt.Fprintf(stdout, "data:        %s\n", dir)
	fmt.Fprintf(stdout, "config:      %s\n", config.Path(dir))
	if cfgErr != nil {
		fmt.Fprintf(stdout, "             unreadable, using defaults: %v\n", cfgErr)
	}
	fmt.Fprintf(stdout, "multiplexer: %s, for new sessions\n", cfg.Multiplexer)
	fmt.Fprintf(stdout, "sessions:    %s\n", session.NewStore(dir).Path())
	if worktrees, err := config.ResolveWorktreeDir(repo.Root, cfg.WorktreeDir); err == nil {
		fmt.Fprintf(stdout, "worktrees:   %s\n", worktrees)
	} else {
		fmt.Fprintf(stdout, "worktrees:   %v\n", err)
	}
	return 0
}

func writeHelp(w io.Writer) {
	fmt.Fprint(w, `vineyard — run coding agents side by side, each in its own worktree

USAGE:
  vineyard [flags]          Launch the TUI for the repository in the current directory
  vineyard <command>

FLAGS:
  -p, --program <command>   Agent to launch in new sessions, e.g. "codex" or "aider --model x"
  -y, --autoyes             Accept agent permission prompts automatically in new sessions
  -i, --issue <id>          Open at grapes issue <id>: its session, or a new one for it

COMMANDS:
  debug                     Print configuration and data paths
  version                   Print the version
  help                      Show this help

Sessions keep running after vineyard exits, in Vineyard's own terminal server or
in tmux (multiplexer in config.toml); the next launch picks them up.
Each repository keeps its configuration and sessions in .vineyard/ in its main checkout;
only .vineyard/config.toml is meant to be committed.
Agent sessions run on the socket "vineyard" (override with VINEYARD_SOCKET).
`)
}

// displayPath shortens a path in the user's home directory to start with ~.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	return path
}

// Command vineyard runs coding agents side by side, each in its own tmux
// session and git worktree, and shows them in one terminal UI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Mibokess/grapes/embedded"
	"github.com/charmbracelet/x/term"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tmux"
	"github.com/mikeryanboss/vineyard/internal/tui"
)

var version = "0.1.1"

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

	if err := runTUI(*program, *autoYes); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func runTUI(program string, autoYes bool) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return errors.New("vineyard needs tmux; install it and try again")
	}
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

	release, err := session.Lock(dir)
	if err != nil {
		return err
	}
	defer release()

	store := session.NewStore(dir)
	sessions, err := store.Load()
	if err != nil {
		return err
	}
	client := tmux.New(tmuxSocket())
	manager := session.NewManager(repo, client, config.ProjectName(repo.Root))
	sessions = manager.Restore(sessions)
	if err := store.Save(sessions); err != nil {
		return err
	}

	grapes, grapesErr := loadGrapes(repo.Root)

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
	}
	model := tui.NewModel(tui.LiveBackend{Manager: manager, Tmux: client, Store: store, Dir: dir}, sessions, opts)

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

// tmuxSocket returns the tmux socket for agent sessions. VINEYARD_TMUX_SOCKET
// overrides it, which keeps development runs away from real sessions.
func tmuxSocket() string {
	if socket := os.Getenv("VINEYARD_TMUX_SOCKET"); socket != "" {
		return socket
	}
	return tmux.DefaultSocket
}

func runDebug(stdout, stderr io.Writer) int {
	fmt.Fprintf(stdout, "version:     %s\n", version)
	fmt.Fprintf(stdout, "tmux socket: %s (tmux -L %s ls)\n", tmuxSocket(), tmuxSocket())
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

COMMANDS:
  debug                     Print configuration and data paths
  version                   Print the version
  help                      Show this help

Sessions keep running in tmux after vineyard exits; the next launch picks them up.
Each repository keeps its configuration and sessions in .vineyard/ in its main checkout;
only .vineyard/config.toml is meant to be committed.
Agent sessions run on the tmux socket "vineyard" (override with VINEYARD_TMUX_SOCKET).
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

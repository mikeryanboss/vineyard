package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
)

// listedSession is a session as `vineyard sessions` prints it.
type listedSession struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Branch string `json:"branch"`
	// Status is the one Vineyard last saved.
	Status string `json:"status"`
	// Issues are those the TUI links the session to.
	Issues   []int  `json:"issues"`
	Worktree string `json:"worktree"`
	// Agents name the Claude Code sessions running in the worktree, which
	// other Claude Code sessions message by these names.
	Agents []string `json:"agents"`
}

// runSessions prints the repository's sessions as a JSON array, for agents
// to find the session, and the agent, working on an issue. It changes
// nothing.
func runSessions(stdout io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repo, err := git.FindRepo(cwd)
	if err != nil {
		return err
	}
	dir := config.Dir(repo.Root)
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	worktreeDir, err := config.ResolveWorktreeDir(repo.Root, cfg.WorktreeDir)
	if err != nil {
		return err
	}
	saved, err := session.NewStore(dir).Load()
	if err != nil {
		return err
	}
	sessions, err := session.NewManager(repo, nil, "", "").Locate(saved, worktreeDir)
	if err != nil {
		return err
	}
	agents, err := claudeAgents()
	if err != nil {
		return err
	}
	listed := []listedSession{}
	for _, s := range sessions {
		var paths []string
		if s.MainCheckout || git.IsWorktree(s.WorktreePath) {
			base, err := git.Base(s.WorktreePath, s.BaseCommit)
			if err != nil {
				return err
			}
			stat, err := git.DiffStat(s.WorktreePath, base)
			if err != nil {
				return err
			}
			paths = stat.Paths
		}
		listed = append(listed, listedSession{
			ID:       s.ID,
			Title:    s.Title,
			Branch:   s.Branch,
			Status:   string(s.Status),
			Issues:   append([]int{}, s.Issues(paths)...),
			Worktree: s.WorktreePath,
			Agents:   append([]string{}, agents[s.WorktreePath]...),
		})
	}
	out, err := json.MarshalIndent(listed, "", "  ")
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(out, '\n'))
	return err
}

// claudeAgents returns the names of the running Claude Code sessions by
// working directory, sorted, from the file each keeps in Claude Code's
// sessions directory while it runs. The format is undocumented and was read
// off Claude Code 2.1.299. A session that crashed leaves its file behind, so
// only those whose process runs count.
func claudeAgents() (map[string][]string, error) {
	dir, err := recap.ConfigDir()
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(dir, "sessions", "*.json"))
	if err != nil {
		return nil, err
	}
	agents := map[string][]string{}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var entry struct {
			PID  int    `json:"pid"`
			Cwd  string `json:"cwd"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(content, &entry); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if entry.PID <= 0 {
			return nil, fmt.Errorf("%s names no process", file)
		}
		if err := syscall.Kill(entry.PID, 0); err != nil && !errors.Is(err, syscall.EPERM) {
			continue
		}
		agents[entry.Cwd] = append(agents[entry.Cwd], entry.Name)
	}
	for _, names := range agents {
		slices.Sort(names)
	}
	return agents, nil
}

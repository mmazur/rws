package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmazur/rws/internal/gitutil"
)

type Config struct {
	Name          string
	WorkspaceRoot string // e.g. ~/work
	BaseDir       string // cwd — where repos live
	Repos         []string
	BranchName    string
	SingleRepo    bool
}

// DiscoverRepos finds git repo subdirectories under dir (non-symlink dirs with .git).
func DiscoverRepos(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var repos []string
	for _, e := range entries {
		if !e.Type().IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if gitutil.IsGitRepo(sub) {
			repos = append(repos, e.Name())
		}
	}
	return repos, nil
}

// DiscoverSymlinks finds symlinks in dir that point to one of the repo subdirectories.
func DiscoverSymlinks(dir string, repos []string) (map[string]string, error) {
	repoSet := make(map[string]bool, len(repos))
	for _, r := range repos {
		repoSet[r] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	symlinks := make(map[string]string)
	for _, e := range entries {
		if e.Type()&os.ModeSymlink == 0 {
			continue
		}
		linkPath := filepath.Join(dir, e.Name())
		target, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}
		// Normalize: target may be relative or absolute
		targetBase := filepath.Base(target)
		if repoSet[targetBase] {
			symlinks[e.Name()] = targetBase
		}
	}
	return symlinks, nil
}

// Create orchestrates workspace creation.
func Create(cfg Config) error {
	wsDir := filepath.Join(cfg.WorkspaceRoot, cfg.Name)

	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}

	var failed []string
	var created []string

	for _, repo := range cfg.Repos {
		repoSrc := filepath.Join(cfg.BaseDir, repo)
		worktreeDst := filepath.Join(wsDir, repo)
		baseBranch, err := gitutil.DefaultBranch(repoSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", repo, err)
			failed = append(failed, repo)
			continue
		}

		if err := gitutil.AddWorktree(repoSrc, worktreeDst, cfg.BranchName, baseBranch); err != nil {
			fmt.Fprintf(os.Stderr, "error: worktree for %s: %v\n", repo, err)
			failed = append(failed, repo)
			continue
		}
		created = append(created, repo)
	}

	fmt.Fprintf(os.Stdout, "Created workspace '%s' (branch %s)\n", cfg.Name, cfg.BranchName)
	fmt.Fprintf(os.Stdout, "Repos: %s\n", strings.Join(created, ", "))

	// Multi-repo extras
	var extras []string
	if !cfg.SingleRepo {
		extras = copyExtras(cfg.BaseDir, wsDir)
		symlinkExtras := recreateSymlinks(cfg.BaseDir, wsDir, cfg.Repos)
		extras = append(extras, symlinkExtras...)
	}

	if len(extras) > 0 {
		fmt.Fprintf(os.Stdout, "Files: %s\n", strings.Join(extras, ", "))
	} else {
		fmt.Fprintln(os.Stdout, "Files: -")
	}

	fmt.Fprintln(os.Stdout, "Dir:")
	if cfg.SingleRepo && len(created) == 1 {
		fmt.Println(filepath.Join(wsDir, created[0]))
	} else {
		fmt.Println(wsDir)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to create worktrees for: %s", strings.Join(failed, ", "))
	}
	return nil
}

func copyExtras(baseDir, wsDir string) []string {
	var extras []string

	// Copy CLAUDE.md
	if copied := copyFile(filepath.Join(baseDir, "CLAUDE.md"), filepath.Join(wsDir, "CLAUDE.md")); copied {
		extras = append(extras, "CLAUDE.md")
	}

	// Copy .claude/settings.json
	src := filepath.Join(baseDir, ".claude", "settings.json")
	dst := filepath.Join(wsDir, ".claude", "settings.json")
	if _, err := os.Stat(src); err == nil {
		if err := os.MkdirAll(filepath.Join(wsDir, ".claude"), 0o755); err == nil {
			if copyFile(src, dst) {
				extras = append(extras, ".claude/settings.json")
			}
		}
	}

	return extras
}

func copyFile(src, dst string) bool {
	in, err := os.Open(src)
	if err != nil {
		return false
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return false
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return false
	}
	return true
}

func recreateSymlinks(baseDir, wsDir string, repos []string) []string {
	symlinks, err := DiscoverSymlinks(baseDir, repos)
	if err != nil {
		return nil
	}

	var extras []string
	for name, target := range symlinks {
		linkPath := filepath.Join(wsDir, name)
		if err := os.Symlink(target, linkPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: symlink %s -> %s: %v\n", name, target, err)
			continue
		}
		extras = append(extras, fmt.Sprintf("%s -> %s", name, target))
	}
	return extras
}

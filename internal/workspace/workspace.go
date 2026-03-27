package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	repoTargets := make(map[string]string, len(repos))
	for _, r := range repos {
		repoPath := filepath.Clean(filepath.Join(dir, r))
		repoTargets[repoPath] = r
		if resolved, err := filepath.EvalSymlinks(repoPath); err == nil {
			repoTargets[filepath.Clean(resolved)] = r
		}
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

		resolvedTarget := target
		if !filepath.IsAbs(resolvedTarget) {
			resolvedTarget = filepath.Join(filepath.Dir(linkPath), resolvedTarget)
		}
		resolvedTarget = filepath.Clean(resolvedTarget)

		if resolved, err := filepath.EvalSymlinks(resolvedTarget); err == nil {
			resolvedTarget = filepath.Clean(resolved)
		}

		if repoName, ok := repoTargets[resolvedTarget]; ok {
			symlinks[e.Name()] = repoName
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

	var extras []string
	var warnings []string
	if !cfg.SingleRepo {
		var copyWarnings []string
		extras, copyWarnings = copyExtras(cfg.BaseDir, wsDir)
		warnings = append(warnings, copyWarnings...)

		var symlinkExtras []string
		var symlinkWarnings []string
		symlinkExtras, symlinkWarnings = recreateSymlinks(cfg.BaseDir, wsDir, cfg.Repos)
		extras = append(extras, symlinkExtras...)
		warnings = append(warnings, symlinkWarnings...)
	}

	if len(created) == 0 {
		if len(failed) > 0 {
			fmt.Fprintf(os.Stderr, "Failed to create workspace '%s': no worktrees created\n", cfg.Name)
			fmt.Fprintf(os.Stderr, "Failed repos: %s\n", strings.Join(failed, ", "))
		}
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}
		return fmt.Errorf("failed to create worktrees for: %s", strings.Join(failed, ", "))
	}

	// Best-effort metadata write
	now := time.Now().UTC()
	if err := WriteMetadata(wsDir, Metadata{CreatedAt: now, UpdatedAt: now}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing metadata: %v\n", err)
	}

	header := fmt.Sprintf("Created workspace '%s' (branch %s)", cfg.Name, cfg.BranchName)
	if len(failed) > 0 {
		header += " with errors"
	}
	fmt.Fprintln(os.Stdout, header)
	fmt.Fprintf(os.Stdout, "Repos: %s\n", strings.Join(created, ", "))

	if len(extras) > 0 {
		fmt.Fprintf(os.Stdout, "Files: %s\n", strings.Join(extras, ", "))
	} else {
		fmt.Fprintln(os.Stdout, "Files: -")
	}

	var dirPath string
	if cfg.SingleRepo && len(created) == 1 {
		dirPath = filepath.Join(wsDir, created[0])
	} else {
		dirPath = wsDir
	}
	fmt.Fprintf(os.Stdout, "Dir: %s\n", dirPath)

	// Write recent dir to state file
	writeRecentDir(dirPath)

	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "Failed repos: %s\n", strings.Join(failed, ", "))
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to create worktrees for: %s", strings.Join(failed, ", "))
	}
	return nil
}

// AmendConfig holds parameters for amending an existing workspace.
type AmendConfig struct {
	Name          string
	WorkspaceRoot string
	BaseDir       string
	NewRepos      []string
	BranchName    string
	SingleRepo    bool
}

// Amend adds new repos to an existing workspace.
func Amend(cfg AmendConfig) error {
	wsDir := filepath.Join(cfg.WorkspaceRoot, cfg.Name)

	var failed []string
	var created []string
	var warnings []string

	for _, repo := range cfg.NewRepos {
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

	// In multi-repo mode, only add symlinks for newly created repos.
	if !cfg.SingleRepo && len(created) > 0 {
		_, symlinkWarnings := recreateSymlinks(cfg.BaseDir, wsDir, created)
		warnings = append(warnings, symlinkWarnings...)
	}

	if len(created) == 0 && len(failed) > 0 {
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}
		return fmt.Errorf("failed to add worktrees for: %s", strings.Join(failed, ", "))
	}

	// Update metadata — if none exists, create fresh
	now := time.Now().UTC()
	meta, err := ReadMetadata(wsDir)
	if err != nil {
		meta = Metadata{CreatedAt: now}
	}
	meta.UpdatedAt = now
	if writeErr := WriteMetadata(wsDir, meta); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: writing metadata: %v\n", writeErr)
	}

	header := fmt.Sprintf("Amended workspace '%s' (branch %s)", cfg.Name, cfg.BranchName)
	if len(failed) > 0 {
		header += " with errors"
	}
	fmt.Fprintln(os.Stdout, header)
	fmt.Fprintf(os.Stdout, "Added repos: %s\n", strings.Join(created, ", "))

	dirPath := wsDir
	if cfg.SingleRepo && len(created) == 1 {
		dirPath = filepath.Join(wsDir, created[0])
	}
	fmt.Fprintf(os.Stdout, "Dir: %s\n", dirPath)

	writeRecentDir(dirPath)

	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "Failed repos: %s\n", strings.Join(failed, ", "))
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to add worktrees for: %s", strings.Join(failed, ", "))
	}
	return nil
}

func copyExtras(baseDir, wsDir string) ([]string, []string) {
	var extras []string
	var warnings []string

	// Copy CLAUDE.md
	claudeSrc := filepath.Join(baseDir, "CLAUDE.md")
	claudeDst := filepath.Join(wsDir, "CLAUDE.md")
	if _, err := os.Stat(claudeSrc); err == nil {
		if err := copyFile(claudeSrc, claudeDst); err == nil {
			extras = append(extras, "CLAUDE.md")
		} else {
			warnings = append(warnings, fmt.Sprintf("copy %s: %v", claudeSrc, err))
		}
	} else if !os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("stat %s: %v", claudeSrc, err))
	}

	// Copy .claude/settings.json
	src := filepath.Join(baseDir, ".claude", "settings.json")
	dst := filepath.Join(wsDir, ".claude", "settings.json")
	if _, err := os.Stat(src); err == nil {
		if err := os.MkdirAll(filepath.Join(wsDir, ".claude"), 0o755); err != nil {
			warnings = append(warnings, fmt.Sprintf("create %s: %v", filepath.Join(wsDir, ".claude"), err))
		} else if err := copyFile(src, dst); err == nil {
			extras = append(extras, ".claude/settings.json")
		} else {
			warnings = append(warnings, fmt.Sprintf("copy %s: %v", src, err))
		}
	} else if !os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("stat %s: %v", src, err))
	}

	return extras, warnings
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return nil
}

func writeRecentDir(dir string) {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	rwsState := filepath.Join(stateDir, "rws")
	if err := os.MkdirAll(rwsState, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(rwsState, "rws_recent_dir"), []byte(dir), 0o644)
}

func recreateSymlinks(baseDir, wsDir string, repos []string) ([]string, []string) {
	symlinks, err := DiscoverSymlinks(baseDir, repos)
	if err != nil {
		return nil, []string{fmt.Sprintf("discover symlinks in %s: %v", baseDir, err)}
	}

	var extras []string
	var warnings []string
	for name, target := range symlinks {
		linkPath := filepath.Join(wsDir, name)
		if err := os.Symlink(target, linkPath); err != nil {
			warnings = append(warnings, fmt.Sprintf("symlink %s -> %s: %v", name, target, err))
			continue
		}
		extras = append(extras, fmt.Sprintf("%s -> %s", name, target))
	}
	return extras, warnings
}

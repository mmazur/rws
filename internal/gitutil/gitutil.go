package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsGitRepo checks if the given path is a git repository.
func IsGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}

// DefaultBranch returns the default branch for the repo at repoPath.
// Tries upstream/HEAD, then origin/HEAD, then checks if "main" or "master" exists locally.
func DefaultBranch(repoPath string) (string, error) {
	for _, ref := range []string{"refs/remotes/upstream/HEAD", "refs/remotes/origin/HEAD"} {
		out, err := runGit(repoPath, "symbolic-ref", ref)
		if err == nil {
			parts := strings.Split(strings.TrimSpace(out), "/")
			if len(parts) > 0 {
				return parts[len(parts)-1], nil
			}
		}
	}
	for _, branch := range []string{"main", "master"} {
		if _, err := runGit(repoPath, "rev-parse", "--verify", branch); err == nil {
			return branch, nil
		}
	}
	return "", fmt.Errorf("could not determine default branch for %s", repoPath)
}

// AddWorktree creates a new git worktree with a new branch.
func AddWorktree(repoPath, worktreePath, newBranch, baseBranch string) error {
	_, err := runGit(repoPath, "worktree", "add", "-b", newBranch, worktreePath, baseBranch)
	return err
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

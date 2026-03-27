package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IsGitRepo checks if the given path is a git repository.
func IsGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}

// RepoRoot returns the absolute repository root for the given path.
func RepoRoot(path string) (string, error) {
	out, err := runGit(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(out)), nil
}

// DefaultBranch returns the default branch for the repo at repoPath.
// Tries upstream/HEAD, then origin/HEAD, then checks if "main" or "master" exists locally.
func DefaultBranch(repoPath string) (string, error) {
	for _, remote := range []string{"upstream", "origin"} {
		ref := "refs/remotes/" + remote + "/HEAD"
		out, err := runGit(repoPath, "symbolic-ref", ref)
		if err == nil {
			prefix := "refs/remotes/" + remote + "/"
			resolved := strings.TrimSpace(out)
			if strings.HasPrefix(resolved, prefix) {
				branch := strings.TrimPrefix(resolved, prefix)
				if branch != "" {
					return branch, nil
				}
			}
		}
	}
	for _, branch := range []string{"main", "master"} {
		if _, err := runGit(repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
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

// LatestCommitTime returns the timestamp of the most recent commit in the repo.
// Returns zero time if the repo has no commits or on error.
func LatestCommitTime(repoPath string) (time.Time, error) {
	out, err := runGit(repoPath, "log", "-1", "--format=%ct")
	if err != nil {
		return time.Time{}, err
	}
	s := strings.TrimSpace(out)
	if s == "" {
		return time.Time{}, nil
	}
	epoch, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing commit timestamp: %w", err)
	}
	return time.Unix(epoch, 0), nil
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

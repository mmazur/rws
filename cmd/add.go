package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mmazur/rws/internal/gitutil"
	"github.com/mmazur/rws/internal/userutil"
	"github.com/mmazur/rws/internal/workspace"
	"github.com/spf13/cobra"
)

var workspaceRoot string

var wsNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var addCmd = &cobra.Command{
	Use:   "add <workspace-name>",
	Short: "Create a new workspace with git worktrees",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func init() {
	defaultRoot := os.Getenv("RWS_WORKSPACE_ROOT")
	if defaultRoot == "" {
		home, _ := os.UserHomeDir()
		defaultRoot = filepath.Join(home, "work")
	}
	addCmd.Flags().StringVarP(&workspaceRoot, "workspace-root", "r", defaultRoot, "root directory for workspaces (env: RWS_WORKSPACE_ROOT)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	wsName := args[0]

	// Validate workspace name format
	if !wsNameRegexp.MatchString(wsName) {
		return fmt.Errorf("invalid workspace name %q: only alphanumeric, hyphens, underscores, and dots allowed", wsName)
	}

	// Expand ~ in workspace root
	root := expandHome(workspaceRoot)

	// Check workspace doesn't already exist
	wsDir := filepath.Join(root, wsName)
	if _, err := os.Stat(wsDir); err == nil {
		return fmt.Errorf("workspace %q already exists at %s", wsName, wsDir)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	singleRepo := gitutil.IsGitRepo(cwd)

	var repos []string
	if singleRepo {
		repos = []string{filepath.Base(cwd)}
	} else {
		repos, err = workspace.DiscoverRepos(cwd)
		if err != nil {
			return fmt.Errorf("discovering repos: %w", err)
		}
		if len(repos) == 0 {
			return fmt.Errorf("no git repos found in %s", cwd)
		}
	}

	// Validate all repos before creating anything
	if singleRepo {
		// Already validated by IsGitRepo check above
	} else {
		for _, repo := range repos {
			repoPath := filepath.Join(cwd, repo)
			if !gitutil.IsGitRepo(repoPath) {
				return fmt.Errorf("%s is not a git repository", repoPath)
			}
		}
	}

	// Discover username and build branch name
	username, found := userutil.DiscoverUsername()
	var branchName string
	if found {
		branchName = username + "/" + wsName
	} else {
		fmt.Fprintln(os.Stderr, "warning: could not discover username, using workspace name as branch name")
		branchName = wsName
	}

	// In single-repo mode, baseDir is the parent of cwd (the repo is cwd itself)
	baseDir := cwd
	if singleRepo {
		baseDir = filepath.Dir(cwd)
	}

	cfg := workspace.Config{
		Name:          wsName,
		WorkspaceRoot: root,
		BaseDir:       baseDir,
		Repos:         repos,
		BranchName:    branchName,
		SingleRepo:    singleRepo,
	}

	return workspace.Create(cfg)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

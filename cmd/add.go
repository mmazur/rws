package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mmazur/rws/internal/gitutil"
	"github.com/mmazur/rws/internal/userutil"
	"github.com/mmazur/rws/internal/workspace"
	"github.com/spf13/cobra"
)

var wsNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var addCmd = &cobra.Command{
	Use:   "add <workspace-name>",
	Short: "Create a new workspace with git worktrees",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	wsName := args[0]

	// Validate workspace name format
	if !wsNameRegexp.MatchString(wsName) {
		return fmt.Errorf("invalid workspace name %q: only alphanumeric, hyphens, underscores, and dots allowed", wsName)
	}

	appCfg, err := resolveConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	root := appCfg.WorkspaceRoot

	// Check workspace doesn't already exist
	wsDir := filepath.Join(root, wsName)
	if _, err := os.Stat(wsDir); err == nil {
		return fmt.Errorf("workspace %q already exists at %s", wsName, wsDir)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	repoRoot, repoErr := gitutil.RepoRoot(cwd)
	singleRepo := repoErr == nil

	var repos []string
	if singleRepo {
		repos = []string{filepath.Base(repoRoot)}
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
		// Already validated by RepoRoot lookup above
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

	// In single-repo mode, baseDir is the parent of repo root.
	baseDir := cwd
	if singleRepo {
		baseDir = filepath.Dir(repoRoot)
	}

	cfg := workspace.Config{
		Name:          wsName,
		WorkspaceRoot: root,
		BaseDir:       baseDir,
		Repos:         repos,
		BranchName:    branchName,
		SingleRepo:    singleRepo,
	}

	if err := workspace.Create(cfg); err != nil {
		return err
	}

	if os.Getenv("RWS_SHELL_FUNCTION") == "1" {
		fmt.Println("\nRun 'rws cd' to cd to the new workspace")
	} else {
		fmt.Println("\nRun 'rws cd' to install shell support for quick workspace switching")
	}

	return nil
}

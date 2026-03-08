package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mmazur/rws/internal/config"
	"github.com/mmazur/rws/internal/gitutil"
	"github.com/mmazur/rws/internal/userutil"
	"github.com/mmazur/rws/internal/workspace"
	"github.com/spf13/cobra"
)

var wsNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var (
	addGroups []string
	addAll    bool
	addAmend  bool
	addSkip   bool
)

var addCmd = &cobra.Command{
	Use:   "add <workspace-name> [repo...]",
	Short: "Create a new workspace with git worktrees",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringSliceVarP(&addGroups, "groups", "g", nil, "repo groups to include")
	addCmd.Flags().BoolVarP(&addAll, "all", "A", false, "discover all repos (ignore defaults)")
	addCmd.Flags().BoolVarP(&addAmend, "amend", "a", false, "add repos to an existing workspace")
	addCmd.Flags().BoolVarP(&addSkip, "skip", "s", false, "skip repos that already exist in workspace (with --amend)")
	rootCmd.AddCommand(addCmd)
}

// parseGroupFlags splits each -g value on commas and whitespace to collect group names.
func parseGroupFlags(raw []string) []string {
	seen := make(map[string]bool)
	var groups []string
	for _, val := range raw {
		for _, part := range strings.FieldsFunc(val, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		}) {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				groups = append(groups, part)
			}
		}
	}
	return groups
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

	wsDir := filepath.Join(root, wsName)

	if addAmend {
		// Workspace must exist for amend
		if _, err := os.Stat(wsDir); os.IsNotExist(err) {
			return fmt.Errorf("workspace %q does not exist at %s (cannot amend)", wsName, wsDir)
		}
	} else {
		// Check workspace doesn't already exist
		if _, err := os.Stat(wsDir); err == nil {
			return fmt.Errorf("workspace %q already exists at %s", wsName, wsDir)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	repoRoot, repoErr := gitutil.RepoRoot(cwd)
	singleRepo := repoErr == nil

	explicitRepos := args[1:]
	groups := parseGroupFlags(addGroups)

	var repos []string
	if singleRepo {
		repos = []string{filepath.Base(repoRoot)}
	} else {
		repos, err = resolveRepos(cwd, explicitRepos, groups, &appCfg)
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			return fmt.Errorf("no git repos found in %s", cwd)
		}
	}

	// If amending, filter out repos already in the workspace
	if addAmend {
		var newRepos []string
		for _, repo := range repos {
			worktreePath := filepath.Join(wsDir, repo)
			if _, err := os.Stat(worktreePath); err == nil {
				if addSkip {
					fmt.Fprintf(os.Stderr, "skipping %s: already in workspace\n", repo)
					continue
				}
				return fmt.Errorf("repo %q already exists in workspace %q (use --skip to skip existing repos)", repo, wsName)
			}
			newRepos = append(newRepos, repo)
		}
		repos = newRepos
		if len(repos) == 0 {
			fmt.Fprintln(os.Stderr, "no new repos to add")
			return nil
		}
	}

	// Validate all repos before creating anything
	if !singleRepo {
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

	if addAmend {
		cfg := workspace.AmendConfig{
			Name:          wsName,
			WorkspaceRoot: root,
			BaseDir:       baseDir,
			NewRepos:      repos,
			BranchName:    branchName,
			SingleRepo:    singleRepo,
		}
		if err := workspace.Amend(cfg); err != nil {
			return err
		}
		printCdHint()
		return nil
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

	printCdHint()
	return nil
}

func printCdHint() {
	if os.Getenv("RWS_SHELL_FUNCTION") == "1" {
		fmt.Println("\nRun 'rws cd' to cd to the new workspace")
	} else {
		fmt.Println("\nRun 'rws cd' to install shell support for quick workspace switching")
	}
}

// resolveRepos determines which repos to use based on flags and config.
func resolveRepos(cwd string, explicitRepos []string, groups []string, appCfg *config.Config) ([]string, error) {
	// --all: discover everything
	if addAll {
		return workspace.DiscoverRepos(cwd)
	}

	hasExplicit := len(explicitRepos) > 0
	hasGroups := len(groups) > 0

	if hasExplicit || hasGroups {
		// Union of explicit repos + group-expanded repos
		seen := make(map[string]bool)
		var result []string
		for _, r := range explicitRepos {
			if !seen[r] {
				seen[r] = true
				result = append(result, r)
			}
		}
		if hasGroups {
			groupRepos, err := appCfg.ResolveGroups(groups)
			if err != nil {
				return nil, err
			}
			for _, r := range groupRepos {
				if !seen[r] {
					seen[r] = true
					result = append(result, r)
				}
			}
		}
		return result, nil
	}

	// Use default_repos if configured
	if len(appCfg.DefaultRepos) > 0 {
		return appCfg.ResolveDefaultRepos()
	}

	// Fall back to discovering all
	return workspace.DiscoverRepos(cwd)
}

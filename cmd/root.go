package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mmazur/rws/internal/config"
	"github.com/spf13/cobra"
)

var workspaceRoot string

var rootCmd = &cobra.Command{
	Use:   "rws",
	Short: "Manage multi-repo workspaces via git worktrees",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&workspaceRoot, "workspace-root", "r", "", "root directory for workspaces")

	// Set custom help function to show resolved config values
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Best-effort config load to enrich help output
		cfg, err := resolveConfig()
		if err == nil {
			enrichHelp(cmd, cfg)
		}
		defaultHelp(cmd, args)
	})
}

func enrichHelp(cmd *cobra.Command, cfg config.Config) {
	// Update workspace-root flag usage on root command
	if f := cmd.Root().PersistentFlags().Lookup("workspace-root"); f != nil {
		f.Usage = fmt.Sprintf("root directory for workspaces (current: %s)", cfg.WorkspaceRoot)
	}

	// Enrich add command flags
	if cmd.Name() == "add" {
		if f := cmd.Flags().Lookup("groups"); f != nil && len(cfg.Groups) > 0 {
			names := make([]string, 0, len(cfg.Groups))
			for k := range cfg.Groups {
				names = append(names, k)
			}
			sort.Strings(names)
			f.Usage = fmt.Sprintf("repo groups to include (available: %s)", strings.Join(names, ", "))
		}

		if len(cfg.DefaultRepos) > 0 {
			cmd.Long = fmt.Sprintf("%s\n\nDefault repos: %s",
				cmd.Short, strings.Join(cfg.DefaultRepos, ", "))
		}
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// resolveConfig loads the layered config and applies the CLI flag override.
func resolveConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}

	if rootCmd.PersistentFlags().Changed("workspace-root") {
		cfg.WorkspaceRoot = config.ExpandHome(workspaceRoot)
	}

	return cfg, nil
}

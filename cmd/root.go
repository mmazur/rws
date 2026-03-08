package cmd

import (
	"fmt"
	"os"

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

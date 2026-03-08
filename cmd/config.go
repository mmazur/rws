package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"},
	Short:   "View resolved configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print the resolved value of a config key",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

func init() {
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg, err := resolveConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	switch args[0] {
	case "workspace_root":
		fmt.Println(cfg.WorkspaceRoot)
	default:
		return fmt.Errorf("unknown config key: %s", args[0])
	}

	return nil
}

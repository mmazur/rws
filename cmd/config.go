package cmd

import (
	"fmt"
	"sort"
	"strings"

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
	case "groups":
		if len(cfg.Groups) == 0 {
			fmt.Println("(none)")
		} else {
			names := make([]string, 0, len(cfg.Groups))
			for k := range cfg.Groups {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("%s: %s\n", name, strings.Join(cfg.Groups[name], ", "))
			}
		}
	case "default_repos":
		if len(cfg.DefaultRepos) == 0 {
			fmt.Println("(none)")
		} else {
			fmt.Println(strings.Join(cfg.DefaultRepos, ", "))
		}
	default:
		return fmt.Errorf("unknown config key: %s", args[0])
	}

	return nil
}

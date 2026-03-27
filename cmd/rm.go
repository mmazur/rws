package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/mmazur/rws/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	rmOlderThan  string
	rmInactiveFor string
	rmDryRun     bool
)

var rmCmd = &cobra.Command{
	Use:     "rm [workspace-name]",
	Aliases: []string{"remove"},
	Short:   "Move workspaces to trash",
	Args:    cobra.MaximumNArgs(1),
	RunE:    runRm,
}

func init() {
	rmCmd.Flags().StringVar(&rmOlderThan, "older-than", "", "trash workspaces created more than this long ago (e.g. 2w, 30d, 3m)")
	rmCmd.Flags().StringVar(&rmInactiveFor, "inactive-for", "", "trash workspaces with no commits for this long (e.g. 2w, 30d, 3m)")
	rmCmd.Flags().BoolVarP(&rmDryRun, "dry-run", "n", false, "show what would be trashed")
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
	hasName := len(args) == 1
	hasFilters := rmOlderThan != "" || rmInactiveFor != ""

	if hasName && hasFilters {
		return fmt.Errorf("cannot combine workspace name with filter flags")
	}
	if !hasName && !hasFilters {
		return fmt.Errorf("provide a workspace name or use --older-than / --inactive-for to select workspaces")
	}

	appCfg, err := resolveConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	root := appCfg.WorkspaceRoot

	if hasName {
		return rmSingle(root, args[0])
	}
	return rmBulk(root)
}

func rmSingle(root, name string) error {
	if rmDryRun {
		fmt.Printf("Would trash '%s'\n", name)
		return nil
	}
	if err := workspace.Trash(root, name); err != nil {
		return err
	}
	fmt.Printf("Trashed '%s'\n", name)
	return nil
}

func rmBulk(root string) error {
	var olderThan, inactiveFor time.Duration
	var err error

	if rmOlderThan != "" {
		olderThan, err = parseDuration(rmOlderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than value: %w", err)
		}
	}
	if rmInactiveFor != "" {
		inactiveFor, err = parseDuration(rmInactiveFor)
		if err != nil {
			return fmt.Errorf("invalid --inactive-for value: %w", err)
		}
	}

	workspaces, err := workspace.ListWorkspaces(root)
	if err != nil {
		return err
	}

	now := time.Now()
	var trashed int

	for _, ws := range workspaces {
		if rmOlderThan != "" {
			if now.Sub(ws.CreatedAt) < olderThan {
				continue
			}
		}

		if rmInactiveFor != "" {
			lastActive, err := workspace.LastActivity(ws.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: checking activity for %s: %v\n", ws.Name, err)
				continue
			}
			if now.Sub(lastActive) < inactiveFor {
				continue
			}
		}

		// Resolve last activity for display
		lastActive, _ := workspace.LastActivity(ws.Path)
		createdStr := ws.CreatedAt.Format("2006-01-02")
		activeStr := lastActive.Format("2006-01-02")

		if rmDryRun {
			fmt.Printf("Would trash '%s' (created %s, last active %s)\n", ws.Name, createdStr, activeStr)
		} else {
			if err := workspace.Trash(root, ws.Name); err != nil {
				fmt.Fprintf(os.Stderr, "error: trashing %s: %v\n", ws.Name, err)
				continue
			}
			fmt.Printf("Trashed '%s' (created %s, last active %s)\n", ws.Name, createdStr, activeStr)
		}
		trashed++
	}

	if trashed == 0 {
		fmt.Println("No workspaces matched")
		return nil
	}

	if rmDryRun {
		fmt.Printf("%d workspaces would be trashed\n", trashed)
	} else {
		fmt.Printf("Trashed %d workspaces\n", trashed)
	}
	return nil
}

var durationRegexp = regexp.MustCompile(`^(\d+)([dwm])$`)

func parseDuration(s string) (time.Duration, error) {
	m := durationRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("%q: expected format like 2w, 30d, or 3m", s)
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unreachable")
}

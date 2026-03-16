package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmazur/rws/internal/shell"
	"github.com/spf13/cobra"
)

var (
	installBash bool
	installZsh  bool
)

var cdCmd = &cobra.Command{
	Use:    "cd",
	Short:  "Install shell integration for quick workspace switching",
	Hidden: true,
	RunE:   runCd,
}

func init() {
	cdCmd.Flags().BoolVar(&installBash, "install-bash", false, "install bash shell integration")
	cdCmd.Flags().BoolVar(&installZsh, "install-zsh", false, "install zsh shell integration")
	rootCmd.AddCommand(cdCmd)
}

const (
	markerStart = "# >>> rws shell wrapper >>>"
	markerEnd   = "# <<< rws shell wrapper <<<"
)

func runCd(cmd *cobra.Command, args []string) error {
	if os.Getenv("RWS_SHELL_FUNCTION") == "1" && !installBash && !installZsh {
		fmt.Println("rws shell integration is already active.")
		fmt.Println("Use 'rws cd <workspace>' or just 'rws cd' to cd to the most recent workspace.")
		return nil
	}

	// Auto-detect shell if no flags given
	if !installBash && !installZsh {
		shellEnv := os.Getenv("SHELL")
		base := filepath.Base(shellEnv)
		switch base {
		case "bash":
			installBash = true
		case "zsh":
			installZsh = true
		default:
			return fmt.Errorf("unknown shell: %s. Use --install-bash or --install-zsh", shellEnv)
		}
	}

	var actions []string

	if installBash {
		action, err := installBashIntegration()
		if err != nil {
			return fmt.Errorf("bash integration: %w", err)
		}
		actions = append(actions, action)
	}

	if installZsh {
		action, err := installZshIntegration()
		if err != nil {
			return fmt.Errorf("zsh integration: %w", err)
		}
		actions = append(actions, action)
	}

	fmt.Println("Installed rws shell wrapper:")
	for _, a := range actions {
		fmt.Println("  " + a)
	}

	fmt.Println()
	if installBash {
		fmt.Println("Restart your shell or run: source ~/.bashrc")
	}
	if installZsh {
		fmt.Println("Restart your shell or run: source ~/.zshrc")
	}

	return nil
}

func installBashIntegration() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	content := markerStart + "\n" + shell.Script + "\n" + markerEnd + "\n"

	// Prefer ~/.bashrc.d/ if it exists
	bashrcD := filepath.Join(home, ".bashrc.d")
	if info, err := os.Stat(bashrcD); err == nil && info.IsDir() {
		target := filepath.Join(bashrcD, "rws-wrapper")
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return "", err
		}
		return "Created " + target, nil
	}

	// Fall back to ~/.bashrc with guard markers
	bashrc := filepath.Join(home, ".bashrc")
	return writeWithMarkers(bashrc, content)
}

func installZshIntegration() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	content := markerStart + "\n" + shell.Script + "\n" + markerEnd + "\n"
	zshrc := filepath.Join(home, ".zshrc")
	return writeWithMarkers(zshrc, content)
}

func writeWithMarkers(filePath, block string) (string, error) {
	existing, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	text := string(existing)
	verb := "Appended to"

	startIdx := strings.Index(text, markerStart)
	endIdx := strings.Index(text, markerEnd)

	if startIdx >= 0 && endIdx >= 0 {
		// Replace existing block
		endIdx += len(markerEnd)
		// Include trailing newline if present
		if endIdx < len(text) && text[endIdx] == '\n' {
			endIdx++
		}
		text = text[:startIdx] + block + text[endIdx:]
		verb = "Updated"
	} else {
		// Append
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += block
	}

	if err := os.WriteFile(filePath, []byte(text), 0o644); err != nil {
		return "", err
	}
	return verb + " " + filePath, nil
}

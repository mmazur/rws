package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	WorkspaceRoot string              `toml:"workspace_root"`
	Groups        map[string][]string `toml:"groups"`
	DefaultRepos  []string            `toml:"default_repos"`
}

// Load returns a Config by layering defaults, global config, and project config.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		WorkspaceRoot: filepath.Join(home, "work"),
	}

	// Global config dir
	globalDir := os.Getenv("XDG_CONFIG_HOME")
	if globalDir == "" {
		globalDir = filepath.Join(home, ".config")
	}
	globalDir = filepath.Join(globalDir, "rws")

	// Project config dir (walk up from CWD looking for .rws/)
	projectDir := findProjectDir()

	// Load in priority order; each overrides previous non-empty values
	for _, path := range []string{
		filepath.Join(globalDir, "config.toml"),
		filepath.Join(globalDir, "config.local.toml"),
	} {
		if err := overlayFile(&cfg, path); err != nil {
			return Config{}, err
		}
	}

	if projectDir != "" {
		for _, path := range []string{
			filepath.Join(projectDir, "config.toml"),
			filepath.Join(projectDir, "config.local.toml"),
		} {
			if err := overlayFile(&cfg, path); err != nil {
				return Config{}, err
			}
		}
	}

	cfg.WorkspaceRoot = ExpandHome(cfg.WorkspaceRoot)

	return cfg, nil
}

// findProjectDir walks up from CWD looking for a .rws/ directory.
func findProjectDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".rws")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// overlayFile reads a TOML file and overlays non-empty values onto cfg.
func overlayFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var fileCfg Config
	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return err
	}

	if fileCfg.WorkspaceRoot != "" {
		cfg.WorkspaceRoot = fileCfg.WorkspaceRoot
	}

	// Merge groups (later layers override same-named groups)
	for name, repos := range fileCfg.Groups {
		if cfg.Groups == nil {
			cfg.Groups = make(map[string][]string)
		}
		cfg.Groups[name] = repos
	}

	// Override default_repos if set in this layer
	if len(fileCfg.DefaultRepos) > 0 {
		cfg.DefaultRepos = fileCfg.DefaultRepos
	}

	return nil
}

// ResolveRepoNames expands group:name references in a list of repo names
// using the config's Groups map. Returns deduplicated repo names in order.
func (c *Config) ResolveRepoNames(names []string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string
	for _, name := range names {
		if strings.HasPrefix(name, "group:") {
			groupName := strings.TrimPrefix(name, "group:")
			repos, ok := c.Groups[groupName]
			if !ok {
				return nil, fmt.Errorf("unknown group %q", groupName)
			}
			for _, r := range repos {
				if !seen[r] {
					seen[r] = true
					result = append(result, r)
				}
			}
		} else {
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}
	return result, nil
}

// ResolveDefaultRepos expands group:name references in DefaultRepos.
func (c *Config) ResolveDefaultRepos() ([]string, error) {
	return c.ResolveRepoNames(c.DefaultRepos)
}

// ResolveGroups expands a list of group names into repo names.
func (c *Config) ResolveGroups(groupNames []string) ([]string, error) {
	var refs []string
	for _, g := range groupNames {
		refs = append(refs, "group:"+g)
	}
	return c.ResolveRepoNames(refs)
}

// ExpandHome expands a leading ~/ to the user's home directory.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

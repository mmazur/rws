package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mmazur/rws/internal/gitutil"
)

const trashDir = ".trash"

type WorkspaceInfo struct {
	Name      string
	Path      string
	CreatedAt time.Time
	UpdatedAt time.Time
	HasMeta   bool
}

// Trash moves a workspace to <workspaceRoot>/.trash/<name>.
// On name collision, appends a timestamp suffix.
func Trash(workspaceRoot, name string) error {
	src := filepath.Join(workspaceRoot, name)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("workspace %q not found: %w", name, err)
	}

	trash := filepath.Join(workspaceRoot, trashDir)
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return fmt.Errorf("creating trash directory: %w", err)
	}

	dstName := name
	dst := filepath.Join(trash, dstName)
	if _, err := os.Stat(dst); err == nil {
		// Name collision — append timestamp
		dstName = name + "-" + time.Now().UTC().Format("20060102T150405")
		dst = filepath.Join(trash, dstName)
	}

	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving workspace to trash: %w", err)
	}
	return nil
}

// ListWorkspaces lists non-hidden subdirectories of workspaceRoot and reads
// metadata from each. For workspaces without .rws.toml, falls back to
// directory mtime.
func ListWorkspaces(workspaceRoot string) ([]WorkspaceInfo, error) {
	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("reading workspace root: %w", err)
	}

	var workspaces []WorkspaceInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}

		wsPath := filepath.Join(workspaceRoot, e.Name())
		info := WorkspaceInfo{
			Name: e.Name(),
			Path: wsPath,
		}

		meta, err := ReadMetadata(wsPath)
		if err == nil {
			info.CreatedAt = meta.CreatedAt
			info.UpdatedAt = meta.UpdatedAt
			info.HasMeta = true
		} else {
			// Fall back to oldest mtime across workspace dir and its
			// git repo subdirs. At least one repo usually stays untouched,
			// so its mtime reflects the original creation time.
			info.CreatedAt = oldestMtime(wsPath)
			info.UpdatedAt = info.CreatedAt
		}

		workspaces = append(workspaces, info)
	}
	return workspaces, nil
}

// LastActivity scans subdirectories of wsDir for git repos and returns
// the most recent commit timestamp. Falls back to metadata updated_at,
// then directory mtime.
func LastActivity(wsDir string) (time.Time, error) {
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		return time.Time{}, fmt.Errorf("reading workspace dir: %w", err)
	}

	var latest time.Time
	foundGit := false

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		sub := filepath.Join(wsDir, e.Name())
		if !gitutil.IsGitRepo(sub) {
			continue
		}
		foundGit = true
		t, err := gitutil.LatestCommitTime(sub)
		if err != nil || t.IsZero() {
			continue
		}
		if t.After(latest) {
			latest = t
		}
	}

	if foundGit && !latest.IsZero() {
		return latest, nil
	}

	// Fall back to metadata
	meta, err := ReadMetadata(wsDir)
	if err == nil && !meta.UpdatedAt.IsZero() {
		return meta.UpdatedAt, nil
	}

	// Fall back to oldest mtime
	return oldestMtime(wsDir), nil
}

// oldestMtime returns the oldest modification time across the directory
// itself and its non-hidden subdirectories that are git repos.
func oldestMtime(dir string) time.Time {
	var oldest time.Time
	if fi, err := os.Stat(dir); err == nil {
		oldest = fi.ModTime()
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return oldest
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		fi, err := os.Stat(sub)
		if err != nil {
			continue
		}
		if oldest.IsZero() || fi.ModTime().Before(oldest) {
			oldest = fi.ModTime()
		}
	}
	return oldest
}

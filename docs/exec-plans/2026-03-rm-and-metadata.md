# Exec Plan: `rws rm` Command + Workspace Metadata (Issues #2, #3)

## Context

Issue #2 requests `rws rm`. Issue #3 wants auto-cleanup of stale workspaces. The user's core workflow: move workspaces to a `.trash/` dir inside `workspace_root` freely and without friction, then auto-clean trash on a schedule with conservative settings (future scope). `rm` is a safe, no-confirmation operation — it just moves to trash.

## Design Principles

- **Bulk-first.** The common case is "trash everything older than X" or "trash everything inactive for X".
- **Safe by default.** `rws rm` moves to `.trash/`, never deletes. No `--force` needed.
- **No git cleanup on rm.** Worktrees and branches stay intact when moved to trash. Cleanup happens when trash is purged (future scope).
- **Metadata is lightweight.** `.rws.toml` stores `created_at` and `updated_at` (structural changes). Actual last-activity detection uses git commit timestamps at query time.

## Steps

### 1. Write exec plan
Write to `docs/exec-plans/2026-03-rm-and-metadata.md`.

### 2. Create `internal/workspace/metadata.go`

**TOML file:** `<workspace>/.rws.toml`

```toml
created_at = 2026-03-27T14:30:00Z
updated_at = 2026-03-27T14:30:00Z
```

**Go struct:**
```go
type Metadata struct {
    CreatedAt time.Time `toml:"created_at"`
    UpdatedAt time.Time `toml:"updated_at"`
}
```

**Functions:**
- `WriteMetadata(wsDir string, meta Metadata) error`
- `ReadMetadata(wsDir string) (Metadata, error)`

### 3. Wire metadata into Create/Amend

In `internal/workspace/workspace.go`:
- `Create()`: after worktree creation, write `.rws.toml` with both timestamps = `time.Now().UTC()`. Best-effort — warn on failure, don't abort.
- `Amend()`: read existing `.rws.toml`, update only `UpdatedAt`, write back. If no metadata exists, create fresh (both = now).

No changes to Config/AmendConfig structs needed.

### 4. Add `LatestCommitTime` to gitutil

Single new function in `internal/gitutil/gitutil.go`:

```go
func LatestCommitTime(repoPath string) (time.Time, error)
```

Runs `git -C <repoPath> log -1 --format=%ct` and parses the unix timestamp. Returns zero time if the repo has no commits or on error.

This is used by `--inactive-for` to check actual work activity: for each workspace, we run this against each repo worktree subdirectory and take the max.

### 5. Create `internal/workspace/trash.go`

**Trash location:** `<workspace_root>/.trash/`

**Functions:**

- `Trash(workspaceRoot, name string) error` — moves `<root>/<name>` to `<root>/.trash/<name>`. Creates `.trash/` if needed. On name collision in trash, appends timestamp suffix: `<name>-20260327T143000`.

- `ListWorkspaces(workspaceRoot string) ([]WorkspaceInfo, error)` — lists non-hidden subdirectories of `workspaceRoot`, reads `.rws.toml` if present. For workspaces without metadata, falls back to directory mtime as a rough `created_at` proxy.

- `LastActivity(wsDir string) (time.Time, error)` — scans subdirectories of `wsDir`, calls `gitutil.LatestCommitTime()` on each that is a git repo, returns the most recent timestamp. If no git repos found or all fail, falls back to `updated_at` from metadata, then directory mtime.

```go
type WorkspaceInfo struct {
    Name      string
    Path      string
    CreatedAt time.Time
    UpdatedAt time.Time
    HasMeta   bool
}
```

### 6. Create `cmd/rm.go`

**Usage:**
```
rws rm <workspace-name>           # move single workspace to trash
rws rm --older-than 2w            # trash workspaces created more than 2 weeks ago
rws rm --inactive-for 2w          # trash workspaces with no commits for 2 weeks
rws rm --older-than 4w --inactive-for 1w  # both conditions (AND)
```

**Command:**
```go
var rmCmd = &cobra.Command{
    Use:     "rm [workspace-name]",
    Aliases: []string{"remove"},
    Short:   "Move workspaces to trash",
    Args:    cobra.MaximumNArgs(1),
    RunE:    runRm,
}
```

**Flags:**
- `--older-than <duration>` — filter by `created_at` age
- `--inactive-for <duration>` — filter by last git commit time (via `LastActivity()`)
- `--dry-run / -n` — show what would be trashed

**Duration parsing:** `Nd` (days), `Nw` (weeks), `Nm` (months, 30 days each). Simple regex-based parser.

**Validation:**
- Single-workspace mode: exactly 1 positional arg, no filter flags
- Bulk mode: no positional arg, at least one filter flag required
- Error if no args and no filter flags (prevents trashing everything)

**`--inactive-for` logic:**
1. For each workspace, call `LastActivity(wsDir)` which runs `git log -1 --format=%ct` in each repo worktree
2. Take the most recent commit time across all repos
3. If `time.Since(lastActivity) > duration`, the workspace qualifies

**Output:**
```
# Single:
Trashed 'feature-x'

# Bulk:
Trashed 'old-feature' (created 2026-01-15, last active 2026-01-20)
Trashed 'abandoned-thing' (created 2025-12-01, last active 2025-12-03)
Trashed 2 workspaces

# Bulk dry-run:
Would trash 'old-feature' (created 2026-01-15, last active 2026-01-20)
Would trash 'abandoned-thing' (created 2025-12-01, last active 2025-12-03)
2 workspaces would be trashed
```

### 7. Verification

- `rws add test-ws` — verify `.rws.toml` created with correct timestamps
- `rws add -a test-ws extra-repo` — verify `updated_at` changes, `created_at` unchanged
- `rws rm test-ws` — verify moved to `.trash/test-ws`
- `rws rm --older-than 0d --dry-run` — verify all workspaces listed
- `rws rm --older-than 0d` — verify all moved to trash
- `rws rm --inactive-for 0d --dry-run` — verify git-based activity detection works
- `rws rm` (no args, no flags) — verify it errors
- Workspace without `.rws.toml` — verify dir mtime fallback works
- Trash name collision — verify timestamp suffix added

### 8. Write execution summary

Add execution summary section to the exec plan.

## Files

| File | Change |
|------|--------|
| `internal/workspace/metadata.go` | **New** — Metadata struct, read/write TOML |
| `internal/workspace/trash.go` | **New** — Trash(), ListWorkspaces(), LastActivity(), WorkspaceInfo |
| `internal/workspace/workspace.go` | Call WriteMetadata in Create() and Amend() |
| `internal/gitutil/gitutil.go` | Add LatestCommitTime() |
| `cmd/rm.go` | **New** — cobra command with single + bulk modes, duration parsing |
| `docs/exec-plans/2026-03-rm-and-metadata.md` | **New** — exec plan |

## Execution Summary

Implementation completed with no deviations from the plan.

**Files created/modified:**
- `internal/workspace/metadata.go` — Metadata struct with TOML serialization, ReadMetadata/WriteMetadata functions.
- `internal/workspace/trash.go` — Trash(), ListWorkspaces(), LastActivity(), WorkspaceInfo struct. All fallback logic implemented as designed.
- `internal/gitutil/gitutil.go` — Added LatestCommitTime() using `git log -1 --format=%ct` with unix timestamp parsing.
- `internal/workspace/workspace.go` — Added metadata writes in Create() (both timestamps = now, best-effort) and Amend() (reads existing, updates UpdatedAt, creates fresh if missing).
- `cmd/rm.go` — Full cobra command with single/bulk modes, `--older-than`/`--inactive-for`/`--dry-run` flags, duration parsing (d/w/m), and validation preventing accidental bulk trash.

**Build:** `go build ./...` and `go vet ./...` both pass cleanly.

**No deviations from plan.** All steps executed as specified.

### Post-plan change: improved mtime fallback for `--older-than`

The original plan used a single directory mtime as the `created_at` fallback when `.rws.toml` is absent. This was refined: `oldestMtime()` now scans the workspace dir **and** all its non-hidden subdirectories, returning the oldest mtime. Since `rws add` typically creates multiple repo worktrees at once and at least one usually stays uncommitted, that repo's mtime preserves the original creation time. This fallback is used in both `ListWorkspaces()` (for `--older-than`) and `LastActivity()` (final fallback).

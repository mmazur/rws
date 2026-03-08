# Plan: Add explicit repos, groups, defaults, amend, and dynamic help to `rws add`

## Context

Currently, `rws add` in multi-repo mode always discovers ALL git repo subdirectories in the current directory. Users need finer control:
- Specify exact repos via positional arguments
- Define named groups of repos in config and select them via `--groups`/`-g`
- Configure a default set of repos (instead of "all") via `default_repos` config key
- Override defaults back to "all" with `--all`/`-A`
- Amend existing workspaces with additional repos via `--amend`/`-a`
- See resolved config values (workspace_root, available groups, default_repos) in `--help` output

## Step 1: Write execution plan

Write the full plan to `docs/exec-plans/2026-03-add-repos-groups.md`.

## Step 2: Add `groups` and `default_repos` to config

**File:** `internal/config/config.go`

- Add fields to `Config` struct:
  - `Groups map[string][]string` with `toml:"groups"`
  - `DefaultRepos []string` with `toml:"default_repos"`
- Update `overlayFile` to:
  - Merge groups from each config layer (later layers override same-named groups)
  - Override `default_repos` if non-empty in later layer
- Config format:
  ```toml
  default_repos = ["repo-x", "group:frontend"]

  [groups]
  frontend = ["repo-a", "repo-b"]
  backend = ["repo-c", "repo-d"]
  ```
- Add a `ResolveDefaultRepos()` method or helper that expands `group:name` references in `default_repos` into actual repo names using the `Groups` map. Error if a referenced group doesn't exist.

## Step 3: Change `add` command signature

**File:** `cmd/add.go`

- Change `Args` from `cobra.ExactArgs(1)` to `cobra.MinimumNArgs(1)`
- Update `Use` string to `"add <workspace-name> [repo...]"`
- Add flags:
  - `--groups`/`-g` string slice flag (supports `-g frontend,backend`, `-g frontend -g backend`, `-g "frontend backend"`)
  - `--all`/`-A` bool flag — ignore defaults, discover all repos
  - `--amend`/`-a` bool flag — add repos to an existing workspace
- In `runAdd`, repo resolution (multi-repo mode only; single-repo unchanged):
  1. Parse `-g` values: split each on commas and whitespace, collect unique group names
  2. Determine repo list based on precedence:
     - If `--all`/`-A`: discover all (ignore defaults, explicit repos, and groups)
     - If explicit repos (`args[1:]`) and/or `--groups` provided: union + deduplicate
     - If neither but `default_repos` configured: resolve `default_repos` (expanding `group:` refs)
     - If none of the above: discover all (current behavior)
  3. Validate: each repo dir must exist and be a git repo; abort before creating anything

## Step 4: Implement `--amend`/`-a`

**File:** `cmd/add.go`

When `--amend` is set:
- `args[0]` is the existing workspace name
- Workspace must already exist (opposite of current check)
- For each new repo: validate it exists, isn't already in the workspace, create worktree
- If any repo already exists in the worktree, abort
- If --skip/-s argument provided, skip adding the existing repositories, do not abort
- Works for both multi (with or without arguments) and single repo
- Recreate symlinks for the full set of repos (existing + new)
- Don't re-copy extras (CLAUDE.md etc.) — they're already there

**File:** `internal/workspace/workspace.go`
- May need a small helper or to make `Create` more flexible to support amending. Or add a separate `Amend` function that creates worktrees for additional repos and updates symlinks.

## Step 5: Filter symlinks to only used repos

**No changes needed.** `recreateSymlinks` already calls `DiscoverSymlinks(baseDir, wsDir, cfg.Repos)` which only returns symlinks pointing to repos in the provided list. Since `cfg.Repos` will contain only the selected subset, symlinks are automatically filtered.

## Step 6: Dynamic `--help` with resolved values

**Files:** `cmd/root.go`, `cmd/add.go`

- On `rootCmd`, set a custom help function (`SetHelpFunc`) that:
  1. Loads config via `resolveConfig()` (best-effort, ignore errors)
  2. Updates the `-r`/`--workspace-root` flag's usage string to include the resolved value, e.g. `"root directory for workspaces (current: /home/user/work)"`
  3. If on `addCmd`, updates `-g`/`--groups` usage string to show available groups, e.g. `"group names (available: frontend, backend)"`
  4. If on `addCmd` and `default_repos` is set, show it in the command's Long description or flag area
  5. Calls Cobra's default help function

## Step 7: Update `rws config get` to support new keys

**File:** `cmd/config.go`

- Add handling for `rws config get groups` and `rws config get default_repos`

## Step 8: Verification

- `rws add --help` shows resolved workspace_root, available groups, and default_repos
- `rws --help` shows resolved workspace_root
- `rws add ws repo1 repo2` creates workspace with only repo1 and repo2
- `rws add ws nonexistent` fails with clear error before creating anything
- `rws add ws -g frontend` creates workspace with repos from frontend group
- `rws add ws -g frontend,backend` works (comma separation)
- `rws add ws -g frontend -g backend` works (repeated flag)
- `rws add ws -g "frontend backend"` works (space separation within quotes)
- `rws add ws repo1 -g backend` creates workspace with union of repo1 + backend repos
- With `default_repos` configured: `rws add ws` uses only the defaults
- `rws add ws -A` overrides defaults and discovers all
- `rws add -a existingws repo-new` adds repo-new to existing workspace
- Symlinks in workspace only point to repos that were actually included
- Single-repo mode (inside a git repo) remains unchanged

## Step 9: Write execution summary

Add a summary section to `docs/exec-plans/2026-03-add-repos-groups.md` documenting deviations.

## Execution Summary

All steps executed as planned. Files modified:

- `internal/config/config.go` — Added `Groups` and `DefaultRepos` fields, merging logic in `overlayFile`, and helper methods `ResolveRepoNames`, `ResolveDefaultRepos`, `ResolveGroups`
- `cmd/add.go` — Changed to `MinimumNArgs(1)`, added `--groups/-g`, `--all/-A`, `--amend/-a`, `--skip/-s` flags, added `parseGroupFlags` and `resolveRepos` helpers, amend flow in `runAdd`
- `cmd/root.go` — Added `SetHelpFunc` with `enrichHelp` to dynamically show resolved workspace_root, available groups, and default_repos in help output
- `cmd/config.go` — Added `groups` and `default_repos` cases to `config get`
- `internal/workspace/workspace.go` — Added `AmendConfig` struct and `Amend` function for adding repos to existing workspaces with symlink recreation

### Deviations

- **Step 4 (Amend)**: Implemented as a separate `Amend` function in workspace package (rather than making `Create` more flexible), since the flows are sufficiently different (no extras copying, symlink cleanup before recreation, different output messaging).
- **Step 6 (Dynamic help)**: The custom help function was placed entirely in `cmd/root.go` using `enrichHelp` helper, rather than splitting logic between `root.go` and `add.go`. The help function checks `cmd.Name() == "add"` to conditionally enrich add-specific flags.
- **Step 5**: Confirmed no changes needed — symlink filtering works automatically through existing `DiscoverSymlinks` mechanism.

## Post-Commit Addendum: `--amend` is additive and non-destructive

### Decision

`rws add --amend` must be strictly additive in workspace contents:

- Never delete files, directories, symlinks, or worktrees from the workspace.
- Only add new worktrees for repos selected in the amend invocation.
- Only attempt to add symlinks related to repos newly created in that amend run.
- If a symlink destination path already exists, warn and continue.
- Existing destination paths must remain untouched (no overwrite, retarget, chmod/chown, or removal).

### Implementation

**File:** `internal/workspace/workspace.go`

- Removed the amend-time symlink cleanup/rebuild behavior that deleted symlinks in workspace root.
- Updated amend symlink handling to call `recreateSymlinks` only for `created` repos from the current amend run.
- Collected symlink creation warnings and emitted them as non-fatal warnings.

### Execution Summary (Addendum)

Completed as planned with no deviations:

- `--amend` no longer performs any delete operations in workspace path.
- Symlink collisions now follow the requested behavior via existing create semantics: warn and continue, without changing the existing path.

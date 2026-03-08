# Config System with workspace_root

## Context
Currently `--workspace-root` / `-r` is a flag only on the `add` command, with `RWS_WORKSPACE_ROOT` env var as fallback and `~/work` as default. We want a proper config file system with layered priority, make `-r` a global flag, drop the env var, and add `rws config get` for shell integration.

## Plan

### 1. Write execution plan
Write the full plan to `docs/exec-plans/2026-03-config-system.md`.

### 2. Add TOML dependency
Add `github.com/pelletier/go-toml/v2` to `go.mod` via `go get`.

### 3. Create `internal/config/` package
New file: `internal/config/config.go`

**Config struct:**
```go
type Config struct {
    WorkspaceRoot string `toml:"workspace_root"`
}
```

**`Load()` function** that:
1. Starts with defaults (`workspace_root` = `~/work`)
2. Determines global config dir: `$XDG_CONFIG_HOME/rws/` or `~/.config/rws/`
3. Walks up from CWD looking for `.rws/` directory (like git walks up for `.git/`)
4. Loads files in priority order (each overrides previous non-empty values):
   - `<global>/config.toml`
   - `<global>/config.local.toml`
   - `<project>/.rws/config.toml` (if found by walk-up)
   - `<project>/.rws/config.local.toml` (if found by walk-up)
5. Expands `~` in values (reuse `expandHome` from `cmd/add.go`, moved here)
6. Returns merged `Config`

The walk-up search: starting from CWD, check if `.rws/` exists. If not, go to parent. Stop at filesystem root. This means `rws config get workspace_root` returns the correct project-overridden value even when 3 subdirs deep.

### 4. Move `--workspace-root` to global persistent flag
In `cmd/root.go`:
- Add `rootCmd.PersistentFlags().StringVarP(&workspaceRoot, "workspace-root", "r", "", "root directory for workspaces")`
- Default is empty string (meaning "not set via CLI")

In `cmd/add.go`:
- Remove the local `--workspace-root` flag and `RWS_WORKSPACE_ROOT` env var logic
- Remove the `workspaceRoot` package var (it moves to root.go)

### 5. Create shared config resolution helper
In `cmd/root.go` (or a helper in cmd package), add a function like `resolveConfig()` that:
1. Calls `config.Load()`
2. If `-r` flag was changed (`rootCmd.PersistentFlags().Changed("workspace-root")`), override `WorkspaceRoot` with the flag value (expand `~` on it too)
3. Returns the resolved `Config`

This is reusable by both `add` and `config get`.

### 6. Wire config into `runAdd`
In `cmd/add.go` `runAdd()`:
- Call `resolveConfig()` to get the final workspace root
- Remove `expandHome(workspaceRoot)` call (handled by config layer)

### 7. Add `rws config get` subcommand
New file: `cmd/config.go`

- `configCmd` with `Use: "config"`, `Aliases: []string{"cfg"}`
- `configGetCmd` with `Use: "get <key>"`, `Args: cobra.ExactArgs(1)`
- For now only supports key `workspace_root`
- Calls `resolveConfig()` and prints the resolved value
- Shell script calls `command rws config get workspace_root`

### 8. Update `rws.sh` shell integration
Replace all `${RWS_WORKSPACE_ROOT:-$HOME/work}` with `$(command rws config get workspace_root)` in:
- `rws cd` name resolution
- Bash completion
- Zsh completion

Spawn `rws` on each invocation (always correct, ~5-20ms latency is imperceptible).

### 9. Clean up
- Remove all references to `RWS_WORKSPACE_ROOT` from Go code
- Remove `expandHome` from `cmd/add.go` (moved to config package)

### 10. Add execution summary
Add summary section to `docs/exec-plans/2026-03-config-system.md`.

## Files to modify
- `go.mod` / `go.sum` — add `github.com/pelletier/go-toml/v2`
- `cmd/root.go` — add persistent `-r` flag, `resolveConfig()` helper
- `cmd/add.go` — remove local flag, use `resolveConfig()`, remove env var
- `cmd/config.go` — **new** — `config`/`cfg` command with `get` subcommand
- `internal/config/config.go` — **new** — config loading with walk-up discovery
- `internal/shell/rws.sh` — replace env var with `rws config get workspace_root`

## Verification
1. `go build` succeeds
2. `rws add` without any config or flag uses `~/work`
3. Create `~/.config/rws/config.toml` with `workspace_root = "/tmp/test"`, verify `rws config get workspace_root` returns `/tmp/test`
4. Create `~/.config/rws/config.local.toml` with `workspace_root = "/tmp/local"`, verify it overrides
5. `rws -r /tmp/override config get workspace_root` — flag overrides config
6. Test `rws -r /tmp/cli add testws` — flag works in global position
7. Test `rws add -r /tmp/cli testws` — flag works after subcommand (Cobra persistent flags)
8. Verify `~` expansion works in config file values
9. Create `/tmp/testproject/.rws/config.toml`, cd to `/tmp/testproject/sub/dir`, run `rws config get workspace_root` — project config found via walk-up

## Execution Summary

All steps were executed as planned with one minor deviation:

### Deviation
- In `cmd/add.go`, the variable returned by `resolveConfig()` was named `appCfg` instead of `cfg` to avoid a naming collision with the existing `cfg := workspace.Config{...}` variable later in `runAdd()`. This is purely a local variable rename with no functional impact.

### Files changed
- `go.mod` / `go.sum` — added `github.com/pelletier/go-toml/v2 v2.2.4`
- `cmd/root.go` — added `workspaceRoot` var, persistent `-r` flag in `init()`, `resolveConfig()` helper, imported `config` package
- `cmd/add.go` — removed `workspaceRoot` var, local `-r` flag, `RWS_WORKSPACE_ROOT` env var logic, `expandHome()` function, `strings` import; now calls `resolveConfig()`
- `cmd/config.go` — **new** — `config`/`cfg` command with `get` subcommand
- `internal/config/config.go` — **new** — `Config` struct, `Load()` with layered config + walk-up discovery, `ExpandHome()`
- `internal/shell/rws.sh` — replaced 3 occurrences of `${RWS_WORKSPACE_ROOT:-$HOME/work}` with `$(command rws config get workspace_root)`

### Verification results
All 9 verification checks passed:
1. `go build` succeeds
2. Default `workspace_root` resolves to `~/work`
3. Global `config.toml` is loaded correctly
4. `config.local.toml` overrides `config.toml`
5. `-r` flag overrides all config files
6. `cfg` alias works for `config` command
7. `~` expansion works in config file values
8. Project `.rws/config.toml` found via walk-up from subdirectories
9. Persistent flag visible in `add --help` under Global Flags

### Post-commit review updates
After reviewing the committed change, one follow-up fix was applied to `internal/shell/rws.sh`:

1. Hardened `rws cd <name>` when config lookup fails:
   - Before: `target` was computed directly from `$(command rws config get workspace_root)/$1`.
   - Issue: if `rws config get workspace_root` failed or returned empty output, `target` could become `/$1`.
   - Now: resolve into `root` first, check command success and non-empty value, and return a clear error if unavailable.

### Additional deviation (post-commit)
- The original plan step for shell integration did not explicitly include failure handling for `rws config get workspace_root`; this was added as a defensive hardening improvement discovered during commit review.

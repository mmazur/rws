# Plan: Shell integration for `rws cd`

## Context
After `rws add`, the user wants to quickly cd into the new workspace. A child process can't change the parent shell's cwd, so we need a shell function wrapper. This plan adds `rws cd` for installing shell support, modifies `rws add` to record state, and provides tab completion.

## Step 1: Write down the plan
This file.

## Step 2: Create the embedded shell function
**New file: `internal/shell/rws.sh`** (embedded via Go `embed`)

Single script compatible with both bash and zsh:

- **`rws` function:** intercepts calls to the `rws` binary
  - If `$1 == "cd"`:
    - No additional arg: read last dir from `$STATE_DIR/rws/rws_recent_dir`, cd to it
    - Arg is absolute path (starts with `/`): cd directly
    - Arg is relative name: cd to `${RWS_WORKSPACE_ROOT:-$HOME/work}/$arg`
    - Print error if dir doesn't exist
  - Otherwise: set `RWS_SHELL_FUNCTION=1` env var and pass through to real `rws` binary (found via `command -v rws`)

- **Completion (bash):** register `complete -F` function
  - For `rws cd <tab>`: list directory names under `$RWS_WORKSPACE_ROOT` (or `~/work`)

- **Completion (zsh):** register via `compdef`
  - Same logic as bash but using zsh completion API (`compadd`)

## Step 3: Create Go embed package
**New file: `internal/shell/shell.go`**

```go
package shell

import _ "embed"

//go:embed rws.sh
var Script string
```

## Step 4: Create `cmd/cd.go` — the `rws cd` subcommand
**New file: `cmd/cd.go`**

Cobra command with `Hidden: true` (so it doesn't appear in cobra's built-in completion or help).

When invoked as the binary (not the shell function), `rws cd` handles installation.

**Flags:** `--install-bash`, `--install-zsh`

**Shell detection** (when no flags provided):
- Read `$SHELL` env var, check if basename is `bash` or `zsh`
- If unknown: print `"Unknown shell: $SHELL. Use --install-bash or --install-zsh."` and exit with error
- If both flags provided: run both codepaths

**When shell function IS active** (`RWS_SHELL_FUNCTION=1`) and no `--install-*` flags:
- The shell function handles `rws cd` directly (cd to recent workspace), so the binary should never receive a bare `rws cd` in this case. But as a safety net, print something helpful.

**Bash codepath:**
1. If `~/.bashrc.d/` exists → write to `~/.bashrc.d/rws` (always overwrite)
2. Else → write to `~/.bashrc` using guard markers:
   ```
   # >>> rws shell integration >>>
   ...script content...
   # <<< rws shell integration <<<
   ```
   If markers already exist, replace the block. Otherwise append.

**Zsh codepath:**
1. Write to `~/.zshrc` using the same guard markers (replace existing block or append)

**Output:** Print exactly what was done:
```
Installed rws shell integration:
  Created ~/.bashrc.d/rws
  Appended to ~/.zshrc

Restart your shell or run: source ~/.bashrc
```
(Use "Created" vs "Updated" vs "Appended to" as appropriate)

## Step 5: Modify `rws add` — state file + hint message

**File: `internal/workspace/workspace.go`** — `Create` function

After printing `Dir:` and the path (lines 152-157):
- Write the directory path to `$STATE_DIR/rws/rws_recent_dir`
  - `STATE_DIR` = `$XDG_STATE_HOME` if set, else `~/.local/state`
  - Create parent dirs if needed (`os.MkdirAll`)

**File: `cmd/add.go`** — `runAdd` function

After `workspace.Create(cfg)` returns successfully:
- Check `os.Getenv("RWS_SHELL_FUNCTION")`:
  - If `"1"`: print `"\nRun 'rws cd' to cd to the new workspace"`
  - Else: print `"\nRun 'rws cd' to install shell support for quick workspace switching"`

## Step 6: Files summary

| File | Action |
|------|--------|
| `internal/shell/rws.sh` | **Create** — embedded shell function + completion |
| `internal/shell/shell.go` | **Create** — Go embed wrapper |
| `cmd/cd.go` | **Create** — `rws cd` install subcommand |
| `cmd/add.go` | **Modify** — add hint message |
| `internal/workspace/workspace.go` | **Modify** — write recent dir to state file |

## Step 7: Verification
1. `go build .` — ensure it compiles
2. `rws cd` with no flags, no shell function → detects shell, installs
3. `rws cd --install-bash` → check `~/.bashrc.d/rws` or `~/.bashrc` is correct
4. Run install twice → verify idempotent (replaces markers, overwrites bashrc.d file)
5. Source the installed script, verify `type rws` shows function
6. `rws cd <tab>` → completes workspace names from ~/work/
7. `rws add test-ws` → verify state file at `~/.local/state/rws/rws_recent_dir` and hint printed
8. `rws cd` (via function, no args) → cd to recent workspace
9. `rws cd test-ws` (via function) → cd to `~/work/test-ws`

## Step 8: Summarize execution

All steps executed as planned with no deviations from the spec.

### Files created
- `internal/shell/rws.sh` — shell function (bash/zsh compatible) with `rws cd` handling and tab completion
- `internal/shell/shell.go` — Go embed wrapper
- `cmd/cd.go` — `rws cd` subcommand (hidden) with `--install-bash`/`--install-zsh` flags, auto-detection, guard markers

### Files modified
- `cmd/add.go` — prints hint message after successful `workspace.Create()` (different message depending on whether shell function is active)
- `internal/workspace/workspace.go` — writes workspace dir path to `$XDG_STATE_HOME/rws/rws_recent_dir` (or `~/.local/state/rws/rws_recent_dir`) after printing `Dir:`

### Post-plan change
- Changed `Dir:` output from two lines (`Dir:\n<path>`) to single line (`Dir: <path>`) per user request.

### Verification
- `go build .` succeeds
- `rws cd --help` shows correct usage

### Post-fix: zsh completion `nomatch` handling

#### Full, unabridged post-fix plan
1. Update zsh completion glob in `internal/shell/rws.sh` from `"$root"/*(/:t)` to `"$root"/*(/N:t)`.
2. Keep completion behavior stable by calling `compadd -a dirs` only when `dirs` is non-empty.

#### Post-fix execution summary
- Implemented Step 1: changed zsh glob to include `N` nullglob qualifier (`"$root"/*(/N:t)`).
- Implemented Step 2: added a non-empty check before `compadd -a dirs`.
- Implemented Step 3: documented this post-fix here in `docs/exec-plans/2026-03-shell-integration.md`.

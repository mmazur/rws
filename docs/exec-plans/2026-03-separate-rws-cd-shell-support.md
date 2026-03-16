# Plan: Separate "rws cd" shell support -- rename to rws-wrapper

## Context

Issue #5 requests separating the shell integration naming. The shell script currently wraps the `rws` command (intercepting `cd` calls), and may be extended in the future. Per user direction, rename the shell file to `rws-wrapper.sh` and update markers/references accordingly.

## Steps

### 1. Write exec plan to `docs/exec-plans/2026-03-separate-rws-cd-shell-support.md`

### 2. Rename `internal/shell/rws.sh` to `internal/shell/rws-wrapper.sh`

Update the header comment from `# rws shell integration` to `# rws shell wrapper`.

### 3. Update `internal/shell/shell.go`

Change `//go:embed rws.sh` to `//go:embed rws-wrapper.sh`.

### 4. Update markers in `cmd/cd.go`

- `markerStart`: `# >>> rws shell integration >>>` to `# >>> rws shell wrapper >>>`
- `markerEnd`: `# <<< rws shell integration <<<` to `# <<< rws shell wrapper <<<`

### 5. Update bashrc.d filename in `cmd/cd.go`

Line 102: `filepath.Join(bashrcD, "rws")` to `filepath.Join(bashrcD, "rws-wrapper")`.

### 6. Update user-facing messages in `cmd/cd.go`

Line 75: `"Installed rws shell integration:"` to `"Installed rws shell wrapper:"`.

### 7. Add execution summary to exec plan

## Execution Summary

All steps executed as planned with no deviations:

1. Exec plan written.
2. Renamed `internal/shell/rws.sh` to `internal/shell/rws-wrapper.sh` and updated header comment.
3. Updated `//go:embed` directive in `internal/shell/shell.go`.
4. Updated `markerStart` and `markerEnd` constants in `cmd/cd.go`.
5. Updated bashrc.d filename from `"rws"` to `"rws-wrapper"` in `cmd/cd.go`.
6. Updated user-facing message from `"Installed rws shell integration:"` to `"Installed rws shell wrapper:"` in `cmd/cd.go`.
7. Verified: `go build ./...` and `go vet ./...` both pass.

## Verification

- `go build ./...` compiles without errors
- `go vet ./...` passes
- Run `rws cd --install-bash` (or `--install-zsh`) and verify:
  - The markers in the shell config use `rws shell wrapper`
  - If using bashrc.d, the file is named `rws-wrapper.sh`
  - The embedded script header says `rws shell wrapper`

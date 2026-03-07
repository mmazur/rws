# Execution Plan: Initial fixups for `rws add`

## Goal

Address five correctness and UX issues in the current implementation of `rws add`:

1. Single-repo detection fails when run from a subdirectory inside a git repo.
2. Symlink recreation uses basename matching and can recreate incorrect links.
3. Output reports success even when worktree creation failed.
4. Default-branch parsing and local checks can be incorrect for some branch/ref cases.
5. Extra file copy operations swallow errors, making failures invisible.

## Plan

### 1) Detect single-repo mode from any subdirectory

- Add gitutil helper(s) to detect whether cwd is inside a repo and return repo root.
- Preferred mechanism: `git rev-parse --show-toplevel`.
- Update `cmd/add.go`:
  - Replace `IsGitRepo(cwd)` mode check with "inside repo" check.
  - In single-repo mode, set repo name from repo root basename.
  - Set `BaseDir` so existing `workspace.Create` path logic remains valid.

### 2) Make symlink recreation target-accurate

- Update `DiscoverSymlinks` in `internal/workspace/workspace.go`:
  - Resolve symlink targets to normalized absolute paths.
  - For relative targets, resolve against link location.
  - Build exact-path allowlist from discovered repos (`baseDir/<repo>`).
  - Recreate only symlinks whose resolved target matches the allowlist.
- Keep recreated symlink targets workspace-local (`<repo>`), preserving output form (`name -> repo`).

### 3) Make output truthful for partial and total failure

- Refactor `workspace.Create` output flow:
  - Track `created` and `failed`, then print final summary.
  - If zero worktrees are created, do not emit success header.
  - If partial success, emit clear created and failed sections.
- Keep existing behavior of returning non-zero when any repo fails.

### 4) Harden default-branch detection and parsing

- Update `internal/gitutil/gitutil.go`:
  - Parse symbolic refs by trimming known prefixes so branch names containing `/` are preserved.
  - Verify local fallback refs explicitly via `refs/heads/main` then `refs/heads/master`.
  - Keep fallback order unchanged.

### 5) Surface extra copy and symlink errors

- Change helper signatures:
  - `copyFile` returns `error`.
  - `copyExtras` returns copied extras plus warnings.
  - `recreateSymlinks` returns created extras plus warnings.
- In `Create`, print non-fatal warnings to stderr so skipped extras are visible.

## Execution Notes (to update during implementation)

- [x] Step 1 complete; details: added `gitutil.RepoRoot(path)` using `git rev-parse --show-toplevel`, switched single-repo mode detection in `cmd/add.go` to repo-root lookup, and derived single-repo name from repo root basename.
- [x] Step 2 complete; details: replaced basename-only symlink matching with normalized absolute target matching against an exact repo path allowlist (including resolved repo paths via `EvalSymlinks`).
- [x] Step 3 complete; details: refactored `workspace.Create` to defer header emission until outcomes are known, emit "with errors" on partial success, and avoid success header entirely when zero worktrees are created.
- [x] Step 4 complete; details: hardened symbolic-ref parsing by trimming only `refs/remotes/<remote>/` prefixes and changed local fallback checks to explicit `show-ref --verify --quiet refs/heads/{main,master}`.
- [x] Step 5 complete; details: converted `copyFile` to return `error`, updated extras/symlink helpers to return warnings, and print non-fatal warnings to stderr in `Create`.

## Actual execution updates

- Files changed: `docs/exec-plans/initial-fixups.md`, `cmd/add.go`, `internal/gitutil/gitutil.go`, `internal/workspace/workspace.go`.
- Notable implementation deviations: symlink matching also whitelists resolved repo paths (`EvalSymlinks`) in addition to direct `baseDir/<repo>` targets to support canonicalized paths.
- Runtime checks performed: ran `gofmt -w` on modified Go files; ran `go test ./...`; ran CLI sanity checks (`go run . add --help`, invalid name case); built `/tmp/rws-bin` and ran smoke scenarios for single-repo-from-subdir, multi-repo symlink filtering, partial failure, and total failure output paths.
- Observed output behavior before and after: partial failures now show a success header with "with errors" and explicit failed repos; total failures now show failure-only header with no misleading success line.

# Execution Plan: `rws add` command

## Context

`rws` is a Go CLI tool for managing multi-repo workspaces via git worktrees. The `add` subcommand creates a new workspace under `~/work/<name>/` with worktrees for one or more repos.

## Two Modes of Operation

### Single-repo mode (cwd IS a git repo)
- Create worktree at `~/work/<wsname>/<reponame>/`
- No extra files copied (no CLAUDE.md, no symlinks)
- Just the worktree, nothing else

### Multi-repo mode (cwd is NOT a git repo)
- Scan all subdirectories of cwd for git repos
- Create `~/work/<wsname>/` directory
- For each discovered repo, create a worktree at `~/work/<wsname>/<reponame>/`
- Copy `CLAUDE.md` from cwd into workspace dir if it exists
- Copy `.claude/settings.json` from cwd into workspace dir if it exists
- Recreate symlinks from cwd that point to any of the repo subdirs
- Print info about every extra file/symlink added

## Branch Naming

- Discover OS username via `os/user.Current()`, fallback to `$USER` env var
- Branch name: `<username>/<wsname>`
- If username can't be discovered: warn on stderr, use just `<wsname>` as branch name

## Default Branch Detection

Try `git symbolic-ref refs/remotes/upstream/HEAD`, then `refs/remotes/origin/HEAD`, then check
if `main` exists locally, then `master`, then error.

## Validation

- Workspace name: reject if `~/work/<wsname>/` already exists
- Repos: validate each is a git repo before creating anything (fail fast)
- Workspace name format: alphanumeric, hyphens, underscores, dots only

## CLI Framework: cobra

## File Structure

```
rws/
  go.mod
  go.sum
  main.go
  cmd/
    root.go
    add.go
  internal/
    gitutil/
      gitutil.go
    workspace/
      workspace.go
    userutil/
      userutil.go
```

## Output

Multi-repo mode:
```
Created workspace 'my-feature' (branch mmazur/my-feature)
Repos: repo-a, repo-b, Full-REPO-Name
Files: CLAUDE.md, .claude/settings.json, name -> Full-REPO-Name
Dir:
/home/user/work/my-feature
```

Single-repo mode:
```
Created workspace 'my-feature' (branch mmazur/my-feature)
Repos: repo-a
Files: -
Dir:
/home/user/work/my-feature/repo-a
```

"Files: -" is printed when no extra files or symlinks were added.

## Implementation Notes

Implementation followed the plan closely. No significant divergences.

- All files created per the planned structure
- cobra v1.10.2 added as the CLI framework
- Tested single-repo mode, multi-repo mode, name validation, and duplicate detection
- On partial worktree failure, the tool continues with remaining repos and exits 1
- `DefaultBranch` fallback chain changed from the original plan: instead of blindly
  falling back to "main", it now tries upstream/HEAD, origin/HEAD, then checks if
  "main" exists locally (`rev-parse --verify`), then "master", and returns an error
  if none are found. Signature changed to `(string, error)`.
- Output format changed: separate lines for header, Repos, Files, and Dir instead of
  a single summary line. "Files: -" shown when no extras.

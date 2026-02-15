---
name: git-worktrees
description: Use when running multiple agents in parallel on the same codebase. Provides git worktree setup, coordination patterns, and project-specific build setup. Invoke when asked to "work in parallel", "set up a worktree", or coordinate with other agents.
---

# Git Worktrees for Parallel Agent Development

Git worktrees allow multiple working directories from a single repo, each with its own branch. Perfect for parallel agent work without conflicts.

## Directory Structure

```
Dev-Source-NoBackup/
├── ora/                    # Main worktree (main branch)
└── ora-worktrees/          # Parallel worktrees
    ├── feat-calendar/
    └── fix-audio/
```

## Quick Commands

```bash
# Create worktree with new branch
git worktree add ../ora-worktrees/feat-xyz -b feat/xyz

# Create worktree from existing branch
git worktree add ../ora-worktrees/feat-xyz feat/xyz

# List all worktrees
git worktree list

# Remove worktree (after merge)
git worktree remove ../ora-worktrees/feat-xyz

# Prune stale references
git worktree prune
```

## Setup New Worktree (Ora-specific)

Run the setup script after creating a worktree:

```bash
# Create and setup in one step
./.claude/skills/git-worktrees/scripts/setup-worktree.sh feat/my-feature
```

Or manually:
```bash
cd ../ora-worktrees/feat-xyz
xcodegen generate              # Generate Xcode project
./build.sh                     # Build (creates build/)
```

## Coordination Between Agents

### Before Starting
1. Sync main: `git fetch origin && git pull origin main`
2. Create worktree with descriptive branch name
3. Record active branch in story doc if applicable

### While Working
- Each agent works independently in their worktree
- No coordination needed during implementation
- Commit frequently (never stash)

### Merging Back
1. First agent to finish: merge to main normally via PR
2. Other agents: sync with main using merge (not rebase — see CLAUDE.md git safety rules):
   ```bash
   git fetch origin
   git merge origin/main
   # Resolve conflicts if any, then commit the merge
   ```
3. Continue or complete work

### After Merge
```bash
# From main worktree
git worktree remove ../ora-worktrees/feat-xyz
git branch -d feat/xyz  # Delete local branch
```

## Gotchas

- **Same branch in two places**: Git prevents checking out the same branch in multiple worktrees
- **Vendored binaries**: May need to copy/symlink `Vendor/` frameworks to each worktree
- **Build artifacts**: Each worktree has its own `build/` and `DerivedData/` (no sharing)
- **Stale worktrees**: Run `git worktree prune` periodically to clean up references

## When to Use

- Multiple agents working on separate features simultaneously
- Long-running feature work while also doing quick fixes
- Comparing implementations side-by-side
- Testing changes against clean main branch

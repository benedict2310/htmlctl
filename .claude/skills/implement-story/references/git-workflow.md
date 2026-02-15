# Git Workflow Guidelines

## Pre-Implementation Safety Checks

Run before writing any code:

```bash
echo "=== GIT SAFETY CHECK ==="
echo "Branch: $(git branch --show-current)"
echo "On main: $([ "$(git branch --show-current)" = "main" ] && echo "YES - DANGER" || echo "no")"
echo "Uncommitted: $(git status --short | wc -l | tr -d ' ') files"
echo "Stashes: $(git stash list | wc -l | tr -d ' ')"
echo "Merge in progress: $([ -f .git/MERGE_HEAD ] && echo "YES" || echo "no")"
echo "Rebase in progress: $([ -d .git/rebase-merge ] && echo "YES" || echo "no")"
```

### Stop Conditions

| Condition | Action |
|:----------|:-------|
| On `main` branch | Create feature branch first - never commit to main |
| Uncommitted changes | Commit them or get user approval to discard |
| Stashed work exists | ⚠️ Recover immediately: `git stash pop` or `git stash branch <name>` |
| Merge in progress | Complete or abort: `git merge --abort` |
| Rebase in progress | Complete or abort: `git rebase --abort` |
| Detached HEAD | Create branch: `git checkout -b <name>` |

## Branch Naming Conventions

| Type | Pattern | Example |
|:-----|:--------|:--------|
| Feature | `feat/<story-id>-<short-name>` | `feat/f07-overlay-window` |
| Bug fix | `fix/<issue>-<short-name>` | `fix/calendar-nil-crash` |
| Refactor | `refactor/<area>` | `refactor/audio-pipeline` |
| Docs | `docs/<topic>` | `docs/testing-guide` |

## Commit Message Format

```
<type>(<scope>): <short description>

<optional body>
```

**Types:** `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

**Examples:**
```bash
git commit -m "feat(overlay): add transcript view with streaming updates"
git commit -m "fix(calendar): handle nil event store gracefully"
git commit -m "test(tools): add CalendarTool unit tests"
git commit -m "refactor(audio): extract ring buffer to separate file"
```

**Rules:**
- Use imperative mood: "Add", "Fix", "Update" (not "Added", "Fixed")
- Keep first line ≤72 characters
- Reference issue numbers if applicable

## Forbidden Commands

> 🚫 **NEVER run these without explicit user permission:**

| Command | Risk | Alternative |
|:--------|:-----|:------------|
| `git stash` | Hides work, easily lost | Commit to branch instead |
| `git reset --hard` | Destroys uncommitted work | Ask user first |
| `git clean -fd` | Deletes untracked files permanently | Review files first |
| `git checkout -- <file>` | Discards changes to file | Commit or review first |
| `git branch -D` | Force deletes branch | Use `-d` (safe delete) |
| `git push --force` | Rewrites remote history | Almost never needed |
| `git rebase` | Rewrites commit history | Use merge for shared branches |

If you need to run any of these, **STOP and ask the user**, explaining what will be lost.

## During Implementation

Commit frequently - each logical unit of work:

```bash
# After each meaningful change
git add <files>
git commit -m "<type>(<scope>): <description>"
```

## Clean Handoff Requirements

Before launching code review, ensure:

```bash
# All of these should pass
git status  # "nothing to commit, working tree clean"
./build.sh  # Build succeeds
xcodebuild test -project Ora.xcodeproj -scheme Ora  # Tests pass
```

Checklist:
- [ ] All changes committed
- [ ] Working tree clean
- [ ] Build succeeds
- [ ] Tests pass

## PR & Merge Workflow

```bash
# Push branch
git push -u origin $(git branch --show-current)

# Create PR
gh pr create --title "feat: <story title>" --body "See story file for details"

# Check CI status (only if CI is configured; skip if no workflows exist)
# gh pr checks

# Merge (squash for clean history)
gh pr merge --squash --delete-branch

# Return to main and clean up
git checkout main
git pull origin main
git branch -d <branch-name> 2>/dev/null || true
git fetch --prune
```

## Recovery

If something goes wrong, you can recover if you noted the starting commit:

```bash
# Reset to a known good state
git checkout <branch>
git reset --hard <commit-sha>
```

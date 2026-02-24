---
name: implement-story
description: Orchestrate story implementation end-to-end by delegating coding work to subagents (pi for standard, codex for complex stories) with cross-agent code review. This skill should be used when asked to implement a story, build a feature from a story doc, or work on a specific story file. Handles the full lifecycle from pre-flight validation through delegated implementation, independent verification, cross-agent review, to PR merge and branch cleanup.
---

# Implement Story (Orchestrator)

Orchestrate Ora story implementation by delegating coding to external subagents and managing the full lifecycle from branch creation to merge.

## Orchestrator Principle

**The orchestrator (Claude Code) NEVER writes implementation code.** It:
- Validates, plans, and routes
- Delegates coding to pi (standard) or codex (complex)
- Independently verifies builds and tests
- Manages the review loop with cross-agent review
- Handles git, PR, and cleanup

## When to Use

- User asks to implement a story
- User provides a story file path
- Keywords: "implement", "build", "story", "feature"

## Required Input

- `STORY`: Path to the story markdown file (e.g., `docs/stories/foundations/F.07-OVERLAY-WINDOW.md`)

## Workflow Overview

### Single Story (default)

```
Phase 0: Pre-Flight ─── BLOCKED? → Stop
         │
Phase 1: Assess Complexity → Route to pi or codex
         │
Phase 2: Setup Branch
         │
Phase 3: Delegate Implementation → Subagent codes, builds, tests, commits
         │
Phase 4: Orchestrator Verification → Independent build + test
         │
         ┌──────────────────────────────────────────┐
         │        Phase 5: Cross-Agent Review        │
         │  ┌────────────────────────────────────┐   │
         │  │ Launch review agent (cross-agent)  │   │
         │  │ Read findings from story file      │   │
         │  │ If issues: delegate fix → re-verify│←──┤
         │  │ If approved: exit loop             │   │
         │  └────────────────────────────────────┘   │
         └──────────────────────────────────────────┘
         │
Phase 6: PR → Merge → Cleanup
```

### Multiple Stories (parallel mode — uses worktrees)

```
Meta-1: Pre-flight ALL stories
         │
Meta-2: Assess complexity for ALL stories
         │
Meta-3: Create worktrees (one per story)
         │
Meta-4: Per story (in its worktree):
         │   ├── Phase 3: impl ──────┐  (subagents run in parallel)
         │   ├── Phase 4: verify     │  (sequential per story)
         │   └── Phase 5: review     │  (sequential per story)
         │
Meta-5: Merge ordering (first-done-first-merged, others sync before merge)
         │
Meta-6: Remove worktrees, prune
```

See "Parallel Mode (Worktrees)" section below for details.

## Phase 0: Pre-Flight

### Step 1: Check Dependencies and Metadata

```bash
.claude/skills/implement-story/scripts/preflight.sh "$STORY" --quiet --no-color
```

**Exit codes:**
- `0` = Ready → Continue
- `1` = Blocked → Stop and report blockers
- `2` = Warnings → Review, then proceed if acceptable

Omit `--quiet` if you need the full pre-flight report.

### Step 2: Read the Story

Read `$STORY` and extract:
- Title and ID
- Acceptance criteria (the checklist)
- Dependencies
- Out of scope
- Implementation plan (if present)

### Step 3: Verify Tool Availability

```bash
which pi >/dev/null 2>&1 || echo "WARNING: pi not installed"
which codex >/dev/null 2>&1 || echo "WARNING: codex not installed"
```

If the selected agent is unavailable, fall back to the other. If neither is available, stop and inform the user.

## Phase 1: Assess Complexity & Route

Run automatic complexity assessment:

```bash
eval "$(.claude/skills/implement-story/scripts/assess-complexity.sh "$STORY")"
```

This sets variables: `COMPLEXITY_SCORE`, `AGENT`, `LEVEL`, `AC_COUNT`, `FILES`, `COMPONENTS`, `COMPLEX_KEYWORDS`, `DEPS`.

Log the routing decision:

```
Complexity: $LEVEL (score: $COMPLEXITY_SCORE/10)
  → Implementation agent: $AGENT
  → Review agent: $([ "$AGENT" = "pi" ] && echo "codex" || echo "pi")
  Factors: ACs=$AC_COUNT, Files=$FILES, Components=$COMPONENTS, Keywords=$COMPLEX_KEYWORDS, Deps=$DEPS
```

See `references/complexity-rubric.md` for scoring details.

**Cross-agent assignment:**
- If `AGENT=pi` → implementation: pi, review: codex
- If `AGENT=codex` → implementation: codex, review: pi

Store the review agent:
```bash
REVIEW_AGENT=$([ "$AGENT" = "pi" ] && echo "codex" || echo "pi")
```

## Phase 2: Setup

### Step 1: Verify Git State

See `references/git-workflow.md` for safety checks. Ensure clean working tree and not on main.

### Step 2: Create Feature Branch

**Single story (default):**

```bash
git checkout main && git pull origin main
git checkout -b feat/<story-id>-<short-name>
```

**Parallel mode (worktrees):** If running multiple stories simultaneously, a worktree has already been created by the meta-orchestrator (see "Parallel Mode" section). The branch exists and the CWD is the worktree root. Skip this step.

### Step 3: Review Project Map

```bash
.claude/skills/implement-story/scripts/project-map.sh --summary --no-color
```

Use this to understand existing components and identify exemplar files.
Run without `--summary` for the full file list.

### Step 4: Load Context

See `references/context-loading.md` for which PRD/Architecture sections to load per story type.

## Phase 3: Delegate Implementation

Launch the implementation subagent:

```bash
.claude/skills/implement-story/scripts/launch-impl.sh "$STORY" "$AGENT" --quiet
```

**Timeout:** Set a generous timeout — minimum 10 minutes for simple stories, 20+ minutes for complex ones.

The script:
1. Gathers story content, project map, branch name
2. Builds prompt from `references/implementation-prompt.md` template
3. Launches the selected agent in non-interactive mode
4. Logs output to `docs/impl-logs/` (quiet mode prints only log path)
Use `--verbose` to stream full subagent output to stdout.

After the subagent exits, verify it produced commits:

```bash
COMMIT_COUNT=$(git log --oneline main..HEAD | wc -l | tr -d ' ')
if [ "$COMMIT_COUNT" -eq 0 ]; then
    echo "ERROR: Subagent produced no commits"
    # Retry or escalate to the other agent
fi
```

## Phase 4: Orchestrator Verification

Independently verify the subagent's work:

### Step 1: Build

```bash
mkdir -p .artifacts
./build.sh > .artifacts/build.log 2>&1
```

If build fails, delegate the fix to the implementation agent (see Phase 5 fix flow). Inspect the log only if needed:

```bash
tail -n 50 .artifacts/build.log
```

### Step 2: Test

```bash
mkdir -p .artifacts
./build.sh test > .artifacts/xcodebuild.test.log 2>&1
```

If tests fail, delegate the fix to the implementation agent. Inspect the log only if needed:

```bash
tail -n 50 .artifacts/xcodebuild.test.log
```

### Step 3: Check Acceptance Criteria

Read the story file and verify that the implementation addresses each acceptance criterion. Cross-reference with the actual code changes:

```bash
git diff --stat main..HEAD
git log --oneline main..HEAD
```

### Step 4: Verify Clean Working Tree

```bash
git status  # Should show "nothing to commit, working tree clean"
```

If there are uncommitted changes, review and commit them (stage specific files, not `git add -A`):

```bash
git status                          # Review what's uncommitted
git add <specific-changed-files>    # Stage only relevant files
git commit -m "chore: commit remaining subagent changes"
```

## Phase 5: Cross-Agent Review Loop

**Maximum iterations: 5.** If not approved after 5, stop and ask the user for guidance.

### Each Iteration:

**1. Launch review subagent (cross-agent):**

```bash
.claude/skills/implement-story/scripts/launch-review.sh "$STORY" "$REVIEW_AGENT" --quiet
```

**Timeout:** Minimum 5 minutes, more for complex stories.
Quiet mode prints only the log path; use `--verbose` to stream full output.

**2. Read findings from story file:**

```bash
grep -A 50 "## Code Review Findings" "$STORY"
```

**3. Parse approval status:**
- `- [x] Ready for merge` → Approved, exit loop
- Unchecked P0/P1 items → Issues to fix

**4. If approved:** Proceed to Phase 6.

**5. If issues found:**

a. Build a fix prompt with the specific findings:

```bash
.claude/skills/implement-story/scripts/launch-impl.sh "$STORY" "$AGENT" --fix --quiet
```

The `--fix` flag includes review findings in the prompt and instructs the agent to fix only those issues.

b. Orchestrator re-verifies (build + test):

```bash
mkdir -p .artifacts
./build.sh test > .artifacts/xcodebuild.test.log 2>&1
```

c. Return to step 1 (relaunch reviewer).

**IMPORTANT:** Even if all issues appear fixed, always run a final review iteration before proceeding to Phase 6 to catch issues introduced by the fixes.

## Phase 6: PR & Merge

### Step 1: Update Story

Update the story file with the implementation summary:

```markdown
## Implementation Summary
**Date:** [ISO date]
**Branch:** `feat/<story-id>`
**Commits:** [count]
**Implemented by:** [pi/codex] (complexity score: N/10)
**Reviewed by:** [codex/pi] (N iterations)

### Files Changed
- `Ora/...` - Created/Modified
```

Commit the story update:

```bash
git add "$STORY"
git commit -m "docs: update story with implementation summary"
```

### Step 2: Push and Create PR

```bash
git push -u origin $(git branch --show-current)

gh pr create --title "feat: <story title>" \
  --body "$(cat <<'EOF'
## Summary
See story file for implementation details and review findings.

**Implemented by:** <agent> | **Reviewed by:** <agent> | **Complexity:** <score>/10

## Acceptance Criteria
<paste checked-off ACs from story>

## Review Status
- Code review: Passed (<N> iterations)
- Build: ✅
- Tests: ✅
EOF
)"
```

### Step 3: Merge

```bash
gh pr merge --squash --delete-branch
```

### Step 4: Update Story with Completion Status

```markdown
## Completion Status
- [x] Implementation complete
- [x] Code review passed ([N] iterations)
- [x] PR merged: <URL>
- [x] Merged to main: <SHA>
- [x] Date: [ISO date]
```

### Step 5: Return to Main and Clean Up

**Single story (default):**

```bash
git checkout main && git pull origin main
git branch -d feat/<story-id>-<short-name> 2>/dev/null || true
git fetch --prune
```

**Parallel mode (worktrees):** Return to the main worktree first, then remove the story's worktree:

```bash
cd <main-worktree-path>
git worktree remove ../ora-worktrees/<worktree-name>
git branch -d feat/<story-id>-<short-name> 2>/dev/null || true
git worktree prune
git checkout main && git pull origin main
git fetch --prune
```

## Parallel Mode (Worktrees)

When the user requests multiple stories implemented simultaneously, the orchestrator enters **parallel mode**. This wraps the per-story workflow (Phases 0-6) in a meta-orchestration layer that manages worktrees and merge ordering.

**Trigger:** User provides multiple story paths, or uses keywords like "in parallel", "simultaneously", "at the same time".

### Meta-Orchestration Workflow

```
Meta-1: Pre-flight ALL stories (Phase 0 for each — stop if any are blocked)
         │
Meta-2: Assess complexity for ALL stories (Phase 1 for each)
         │
Meta-3: Create one worktree per story
         │
Meta-4: For each story, run Phases 2-5 in its worktree
         │  (implementation subagents run in parallel;
         │   verification and review run sequentially per story)
         │
Meta-5: Merge ordering — first-done-first-merged
         │
Meta-6: Clean up all worktrees
```

### Meta-1: Pre-flight All Stories

Run Phase 0 (pre-flight) for every story before creating any worktrees. If any story is blocked, report all blockers and stop. Do not partially proceed.

```bash
for STORY in $STORIES; do
    .claude/skills/implement-story/scripts/preflight.sh "$STORY" --quiet --no-color
done
```

### Meta-2: Assess Complexity

Run Phase 1 for each story. Record the agent assignment per story:

```bash
for STORY in $STORIES; do
    eval "$(.claude/skills/implement-story/scripts/assess-complexity.sh "$STORY")"
    # Store: STORY → AGENT, REVIEW_AGENT, COMPLEXITY_SCORE
done
```

### Meta-3: Create Worktrees

From the main worktree, sync main and create one worktree per story:

```bash
git checkout main && git pull origin main

for STORY_ID in $STORY_IDS; do
    .claude/skills/git-worktrees/scripts/setup-worktree.sh "feat/${STORY_ID}"
done
```

Each worktree is at `../ora-worktrees/feat-<story-id>/` with its own branch. The setup script runs `xcodegen generate` per worktree so builds work independently.

### Meta-4: Run Per-Story Workflow

For each story, `cd` to its worktree and run Phases 2-5. The per-story workflow is identical to single-story mode except:

- **Phase 2 Step 2 is skipped** — the branch already exists from worktree creation.
- **All commands run from the worktree CWD** — `./build.sh`, `launch-impl.sh`, `launch-review.sh` all operate relative to the worktree root. The scripts use `SCRIPT_DIR`-relative paths and `git diff main...HEAD`, both of which work correctly in worktrees.

**Parallelism strategy:** Launch implementation subagents (Phase 3) for all stories in parallel, then run verification (Phase 4) and review (Phase 5) sequentially per story. This maximizes throughput on the longest phase while keeping verification safe.

```
Story A: [───── Phase 3: impl ─────][Phase 4][Phase 5]
Story B: [───── Phase 3: impl ─────────][Phase 4][Phase 5]
Story C: [──── Phase 3: impl ────][Phase 4][Phase 5]
```

### Meta-5: Merge Ordering

Stories merge in completion order. After the first story merges to main, subsequent stories must sync before merging:

**First story to complete:**

```bash
cd ../ora-worktrees/feat-<first-story-id>/
# Run Phase 6 normally (push, PR, merge)
```

**Each subsequent story:**

```bash
cd ../ora-worktrees/feat-<next-story-id>/
git fetch origin
git merge origin/main
# Resolve conflicts if any, then commit the merge

# Re-verify: build and test after merge
mkdir -p .artifacts
./build.sh test > .artifacts/xcodebuild.test.log 2>&1

# If build/test fail after merge: delegate fix to implementation agent, re-verify
# Then run Phase 6 normally (push, PR, merge)
```

### Meta-6: Clean Up All Worktrees

After all stories are merged:

```bash
cd <main-worktree-path>
git checkout main && git pull origin main

for WORKTREE in ../ora-worktrees/feat-*; do
    git worktree remove "$WORKTREE" 2>/dev/null || true
done

git worktree prune
git fetch --prune
```

### Conflict Resolution

When merging `origin/main` into a story branch after a prior story merged:

1. **Auto-resolvable conflicts** (non-overlapping changes): git resolves these automatically.
2. **Real conflicts** (same file/region modified by two stories): Delegate the resolution to the story's implementation agent with a fix prompt that includes the conflict markers and both story contexts.
3. **If conflicts cannot be resolved**: Stop and report to user with the conflicting files and both story summaries.

## Agent Escalation

If the initially selected agent fails (no commits, build failures after retry, repeated review failures):

1. Log the failure reason
2. Switch to the other agent
3. Restart from Phase 3 on the same branch
4. If both agents fail, stop and report to user with full context

## Scripts

| Script | Purpose |
|:-------|:--------|
| `scripts/assess-complexity.sh` | Analyze story and recommend pi or codex |
| `scripts/launch-impl.sh` | Launch implementation subagent (`--quiet` for minimal stdout) |
| `scripts/launch-review.sh` | Launch review subagent (`--quiet` for minimal stdout) |
| `scripts/launch-review-diff.sh` | Review based on commit range (post-merge, supports `--agent` and `--quiet`) |
| `scripts/preflight.sh` | Validate story readiness (`--quiet`/`--no-color` for summary output) |
| `scripts/project-map.sh` | Show project structure (`--summary`/`--no-color` for compact output) |

## References

| File | Purpose |
|:-----|:--------|
| `references/implementation-prompt.md` | Prompt template for implementation subagent |
| `references/fix-prompt.md` | Prompt template for fixing review findings |
| `references/code-review-prompt.md` | Prompt for review subagent |
| `references/complexity-rubric.md` | Scoring algorithm and thresholds |
| `references/context-loading.md` | Which PRD/Architecture sections to load per story type |
| `references/git-workflow.md` | Git safety, branching, commits |
| `references/testing-guidelines.md` | Test patterns and coverage requirements |

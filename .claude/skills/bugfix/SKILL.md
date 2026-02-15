---
name: bugfix
description: A step-by-step workflow for investigating, fixing, and verifying bugs. This skill should be used when asked to fix a bug, investigate an issue, or resolve a regression. It covers the full lifecycle from context gathering, root cause analysis, reproduction, implementation, verification, review, to PR merge and branch cleanup.
---

# Bugfix Skill

Rigorous workflow for fixing bugs in the Ora project. Ensures fixes are well-understood, tested, and compliant with project standards before merging.

## Workflow

### Phase 1: Context & Investigation

**Goal:** Understand *what* is broken, *why* it is broken, and *how* to prove it.

1.  **Load Project Context**:
    *   Read `CLAUDE.md` to understand the architecture, build system, and git workflow.
    *   Read `docs/stories/README.md` or relevant story/bug files if the bug is related to a specific feature.
    *   If the user mentions a specific file or error, read that file first.

2.  **Root Cause Analysis (RCA)**:
    *   Search the codebase to locate relevant code.
    *   Read and examine the code logic.
    *   Formulate a hypothesis: "I believe X is failing because Y state is not handled."

3.  **Reproduction (The "Red" Step)**:
    *   **Preferred**: Create a new test case in `OraTests/` that reproduces the bug. It *must* fail before the fix.
    *   **Alternative**: If a unit test is impossible (e.g., UI glitch), define a clear manual verification step and document it.
    *   *Deliverable*: A failing test or a documented reproduction path.

### Phase 2: Implementation & Verification

**Goal:** Apply the fix and prove it works without breaking regressions.

1.  **Git Setup**:
    *   Sync with main before branching:
        ```bash
        git checkout main && git pull origin main
        ```
    *   Create a fix branch:
        ```bash
        git checkout -b fix/<descriptive-name>
        ```
    *   *Never* commit directly to `main`.

2.  **Apply Fix (The "Green" Step)**:
    *   Edit the code to address the root cause identified in Phase 1.
    *   Keep changes focused — don't refactor unrelated code.

3.  **Commit**:
    *   Stage specific files (not `git add -A` or `git add .`):
        ```bash
        git add <specific-files>
        git commit -m "fix(<scope>): <description>"
        ```

4.  **Verification**:
    *   **Build**: `./build.sh`
    *   **Test**: `./build.sh test`
    *   *Loop*: If tests fail, analyze → fix → commit → retry. Continue until the reproduction test passes AND all other tests pass (no regressions).

### Phase 3: Review

**Goal:** Ensure code quality before merging.

1.  **Code Review (MANDATORY)**:
    *   If the bug has an associated story/bug file (e.g., `docs/stories/bugs/...`), launch a review subagent:
        ```bash
        .claude/skills/implement-story/scripts/launch-review.sh <path-to-bug-doc>
        ```
        The review script accepts an optional agent parameter (`pi` or `codex`).
    *   If no story/bug doc exists, perform a **Strict Self-Review**:
        *   Analyze the diff: `git diff main...HEAD`
        *   Check for hardcoded values, leftover debug prints, or commented-out code.
        *   Verify variable names are clear and conform to Swift style.
        *   Ensure concurrency is handled (MainActor, Sendable, etc.).
    *   **Loop**: If issues are found, fix them, commit, and repeat the review.

2.  **Update Bug Documentation** (if a bug doc exists):
    *   Add a brief implementation summary to the bug doc:
        ```markdown
        ## Fix Summary
        **Date:** [ISO date]
        **Branch:** `fix/<name>`
        **Root Cause:** <brief explanation>
        **Fix:** <what was changed and why>
        ```

### Phase 4: PR & Merge

**Goal:** Merge the fix to mainline via PR.

1.  **Ask for Confirmation**: Present the fix summary, test results, and review status to the user before proceeding.

2.  **Push and Create PR**:
    ```bash
    git push -u origin $(git branch --show-current)

    gh pr create --title "fix: <description>" \
      --body "$(cat <<'EOF'
    ## Summary
    <what was broken and why>

    ## Fix
    <what was changed>

    ## Test Plan
    - <how the fix was verified>
    EOF
    )"
    ```

3.  **Merge**:
    ```bash
    gh pr merge --squash --delete-branch
    ```

4.  **Return to Main and Clean Up**:
    ```bash
    git checkout main && git pull origin main
    git branch -d fix/<name> 2>/dev/null || true
    git fetch --prune
    ```

## Best Practices

*   **Atomic Commits**: Keep the fix focused. Don't refactor unrelated code.
*   **Documentation**: If the bug revealed a gap in documentation (e.g., a setup step or architectural constraint), update `docs/` or `CLAUDE.md`.
*   **Communication**: Explain *why* the fix works, not just *what* changed.

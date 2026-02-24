---
name: github-docs
description: Use the GitHub CLI (gh) for repo discovery, issues/PRs, reviews, checks, releases, and automation workflows. Use when working with GitHub operations or documentation that previously relied on JS SDK.
---

# GitHub CLI Skill

Use `gh` for all GitHub operations. Prefer CLI over custom scripts/SDKs. Keep commands minimal, idempotent, and aligned with repo policies.

## Quick Start

- **Auth & context**
  - `gh auth status`
  - `gh repo view --json name,owner,defaultBranchRef`
- **Repo selection**
  - If not in a git repo: `gh repo clone OWNER/REPO` or `gh repo view OWNER/REPO`.

## Guardrails

- Follow repo workflow rules (no direct commits to `main`, use feature branches, no force push, etc.).
- Avoid destructive actions unless explicitly requested (delete branch/tag/release, force merge, etc.).
- When in doubt, **list first**, then act.

## Core Workflows (Senior Use Cases)

### 1) Issue Triage & Grooming

- List with filters:
  - `gh issue list --assignee @me --state open --label bug --limit 50`
  - `gh issue list --search "is:open label:ux sort:updated-desc"`
- Inspect details:
  - `gh issue view <num> --json title,body,labels,assignees,comments`
- Update:
  - `gh issue edit <num> --add-label "triaged" --remove-label "needs-info"`
  - `gh issue comment <num> --body "Status update..."`

### 2) PR Creation & Iteration

- Create PR (draft first when unsure):
  - `gh pr create --draft --base main --head <branch> --title "..." --body "..."`
- Update PR metadata:
  - `gh pr edit <num> --add-label "ready" --reviewer USER --assignee @me`
- View PR status & checks:
  - `gh pr view <num> --json statusCheckRollup,reviewDecision`
  - `gh pr checks <num>`

### 3) Code Review & Risk Analysis

- Review a PR:
  - `gh pr view <num> --files`
  - `gh pr diff <num>`
- Submit review:
  - `gh pr review <num> --approve --body "LGTM"`
  - `gh pr review <num> --request-changes --body "Blocking issues..."`

### 4) Merge & Branch Hygiene

- Merge with guardrails:
  - `gh pr merge <num> --merge --delete-branch`
- Verify merge status:
  - `gh pr view <num> --json merged,mergeCommit`

### 5) Release Management

- Draft release:
  - `gh release create vX.Y.Z --draft --notes "..."`
- Publish release:
  - `gh release edit vX.Y.Z --draft=false`
- Download artifacts:
  - `gh release download vX.Y.Z --pattern "*.zip"`

### 6) CI / Actions Triage

- List workflows:
  - `gh workflow list`
- Run workflow (manual):
  - `gh workflow run "CI" --ref <branch>`
- Check runs:
  - `gh run list --workflow "CI" --limit 20`
  - `gh run view <run-id> --log`

### 7) Repo Insights & Ownership

- View CODEOWNERS and policies:
  - `gh repo view --json name,defaultBranchRef,visibility`
- Search code (fast triage):
  - `gh search code "pattern" --repo OWNER/REPO --limit 20`

### 8) Security & Secrets (Read-Only unless asked)

- List secrets:
  - `gh secret list`
- Avoid modifying secrets unless explicitly requested.

### 9) Projects & Discussions

- List discussions:
  - `gh discussion list --limit 30`
- List projects (beta):
  - `gh project list --owner OWNER`

## Patterns to Prefer

- **List → Inspect → Act** for any risky change.
- **JSON output** for deterministic parsing:
  - `gh pr list --json number,title,headRefName,reviewDecision`
- **Explicit base/head** for PRs and merges.

## When You Need API Access

Use `gh api` for endpoints not covered by commands:

- Example:
  - `gh api /repos/OWNER/REPO/branches --paginate`
  - `gh api graphql -f query='...'`

Always prefer `gh` subcommands before `gh api`.

## Output Expectations

- Summarize key info; do not dump large logs unless requested.
- When user asks for “latest”, check with `gh` rather than guessing.

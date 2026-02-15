---
name: write-story
description: Draft or update Ora user story markdown files in docs/stories, including required metadata, dependencies, scope, implementation plan outline, and acceptance criteria checkboxes. Use when asked to create a new story, revise an existing story file, or define requirements/specs for a feature before implementation.
---

# Write Story

Create implementable story specs that align with Ora's architecture and the implement-story workflow.

## Workflow

### 1) Identify Story Placement

- Choose the epic and story ID (F/A/L/T/X/O/E/S).
- File path must be `docs/stories/<epic>/<ID>-<TITLE>.md` and title format `# <ID> - <Title>`.
- Check `docs/stories/README.md` and the epic `docs/stories/<epic>/README.md` for the next available ID and dependency graph.

### 2) Load Context

- Always read `docs/stories/README.md` for project context.
- Use `.claude/skills/implement-story/references/context-loading.md` to load the right PRD/ARCHITECTURE sections.
- Use `.claude/skills/implement-story/scripts/project-map.sh` to see current components and avoid duplicating existing work.

### 3) Draft the Story

- Start from `references/story-template.md`.
- Or run `.claude/skills/write-story/scripts/new_story.py --id <ID> --title "<Title>"` to generate the file and update indexes.
- Ensure required metadata fields are present and formatted exactly as `**Field:** Value`.
- Write acceptance criteria as checkboxes so preflight passes.
- Keep scope tight; explicitly list what is out of scope.

### 4) Architectural Review

- Validate the design against Ora architecture and conventions.
- Use `references/architectural-review.md` to ensure correct component boundaries, concurrency model, guardrails, and audit logging requirements.

### 5) Quality Checks

- Run `.claude/skills/write-story/scripts/story_lint.py <story-path>` for structural checks.
- Run `.claude/skills/implement-story/scripts/preflight.sh <story-path>` to ensure implement-story readiness.
- Use `references/quality-checks.md` to verify completeness and implementability.

### 6) Update Story Indexes

- Add the story to `docs/stories/README.md` and the epic README in `docs/stories/<epic>/README.md`.
- Ensure status matches the story file.

### 7) Commit the Story

Story files must be committed — uncommitted stories are invisible to other agents and easily lost.

- If already on a feature branch (e.g., writing a story as part of implementation), commit to the current branch:
  ```bash
  git add docs/stories/<epic>/<ID>-<TITLE>.md docs/stories/<epic>/README.md docs/stories/README.md
  git commit -m "docs(<epic>): add story <ID> - <Title>"
  ```

- If on `main` or no branch exists, create a docs branch:
  ```bash
  git checkout main && git pull origin main
  git checkout -b docs/<story-id>-<short-name>
  git add docs/stories/<epic>/<ID>-<TITLE>.md docs/stories/<epic>/README.md docs/stories/README.md
  git commit -m "docs(<epic>): add story <ID> - <Title>"
  git push -u origin $(git branch --show-current)
  gh pr create --title "docs: add story <ID> - <Title>" --body "New story spec for <Title>."
  gh pr merge --squash --delete-branch
  git checkout main && git pull origin main
  ```

- Stage only the story file and any updated indexes — not `git add -A`.

## Resources

- `scripts/new_story.py` - Create a new story from the template and update indexes
- `scripts/story_lint.py` - Lint a story file for completeness and formatting
- `references/story-template.md` - Canonical story template (aligned with implement-story preflight and review loop)
- `references/quality-checks.md` - Story readiness checklist (metadata, scope, tests, acceptance criteria)
- `references/architectural-review.md` - Architecture alignment checklist by component type

# Repository Guidelines

## Project Structure & Module Organization
- `docs/prd.md`, `docs/technical-spec.md`, and `docs/epics.md` define product, architecture, and delivery sequencing.
- `docs/stories/` contains implementation stories named `E<epic>-S<story>-<slug>.md` (for example, `E3-S4-diff-engine.md`).
- `.claude/skills/` contains contributor automation (story linting, preflight checks, and project mapping scripts).
- Runtime code is planned but not yet checked in; stories reference a Go layout with `cmd/htmlctl`, `cmd/htmlservd`, `internal/`, `pkg/`, and `testdata/`.

## Build, Test, and Development Commands
Current repository work is spec-first. Use these commands while authoring or refining stories:
- `python3 .claude/skills/write-story/scripts/story_lint.py <story-file> --strict` validates story structure and required sections.
- `.claude/skills/implement-story/scripts/preflight.sh <story-file> --quiet --no-color` checks implementation readiness and dependencies.
- `.claude/skills/implement-story/scripts/project-map.sh --summary --no-color` shows the current project/component map.

When Go implementation files are added, follow story-defined targets:
- `make build` to compile binaries.
- `make test` (or `go test ./...`) to run unit/integration tests.
- `make lint` to run static checks.

## Coding Style & Naming Conventions
- Story docs should keep the established numbered section format (`## 1. Objective` through `## 7. Verification Plan`) and checkbox acceptance criteria.
- For Go code, use `gofmt` defaults, lowercase package names, and `_test.go` suffixes.
- Follow existing naming in story plans for multiword files (for example, `rollout_history.go`, `domain_verify.go`).
- Keep CLI output deterministic and machine-parseable, aligned with `docs/technical-spec.md`.

## Testing Guidelines
- Each story must define tests in `### 5.3 Tests to Add` and executable checks in `## 7. Verification Plan`.
- Prefer table-driven unit tests plus targeted integration tests for CLI/API behavior.
- Prioritize safety-critical scenarios: deterministic rendering, atomic release activation, rollback, and promotion hash parity.
- No global coverage percentage is specified; untested acceptance criteria should be treated as incomplete.

## Commit & Pull Request Guidelines
- This workspace snapshot does not include `.git` history; use the documented convention: `<type>(<scope>): <imperative summary>`.
- Recommended types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`.
- Branch naming: `feat/<story-id>-<short-name>`, `docs/<topic>`, `fix/<issue>-<short-name>`.
- PRs should include the linked story file, acceptance criteria status, and test evidence (commands run + outcomes).
- Include screenshots only when changing rendered site output; avoid destructive git operations without explicit approval.

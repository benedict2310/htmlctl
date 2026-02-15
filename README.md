# htmlctl

`htmlctl` is a kubectl-style control plane for static HTML/CSS/JS websites, designed for AI-agent and CLI-first workflows.

It manages declarative website resources, builds deterministic static output, and deploys via immutable releases with atomic activation, rollback, and staged promotion (`staging` -> `prod`).

## Project Status

This repository currently contains product and implementation specifications. Runtime Go binaries are planned as:

- `htmlctl` (CLI)
- `htmlservd` (server daemon)

## Core Goals

- Deterministic publish-time composition of pages and components
- Safe production deployments (immutable releases + atomic switch)
- Artifact promotion without rebuild
- Private-by-default control plane (SSH tunnel/localhost)
- Domain and TLS management via Caddy integration

## Repository Layout

- `docs/prd.md` - Product requirements and scope
- `docs/technical-spec.md` - Architecture and technical design
- `docs/epics.md` - Epic/story map
- `docs/stories/` - Detailed implementation stories
- `.claude/skills/` - Contributor automation scripts and workflows

## Contributor Workflow

1. Read `docs/technical-spec.md` and the relevant story in `docs/stories/`.
2. Validate story quality before implementation:
   - `python3 .claude/skills/write-story/scripts/story_lint.py <story-file> --strict`
   - `.claude/skills/implement-story/scripts/preflight.sh <story-file> --quiet --no-color`
3. Implement according to story acceptance criteria and verification plan.

## Planned Commands

From the technical spec, primary CLI workflows include:

- `htmlctl render -f ./site -o ./dist`
- `htmlctl serve ./dist --port 8080`
- `htmlctl diff -f ./site --context staging`
- `htmlctl apply -f ./site --context staging`
- `htmlctl promote website/<name> --from staging --to prod`

## License

License not yet defined.

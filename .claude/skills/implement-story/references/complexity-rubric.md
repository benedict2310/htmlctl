# Complexity Rubric

How `assess-complexity.sh` scores stories and routes to implementation agents.

## Scoring Factors

| Factor | Low (Score) | Medium (Score) | High (Score) |
|:-------|:------------|:---------------|:-------------|
| **Acceptance Criteria** | 1-3 ACs (+1) | 4-6 ACs (+2) | 7+ ACs (+3) |
| **Files Referenced** | 0-2 files (+1) | 3-5 files (+2) | 6+ files (+3) |
| **Component Diversity** | 1 component (+0) | 2 components (+1) | 3+ components (+2) |
| **Complexity Keywords** | None (+0) | Any present (+1) | — |
| **Dependencies** | 0-2 deps (+0) | — | 3+ deps (+1) |

**Maximum possible score:** 10

## Routing Threshold

| Score | Level | Agent | Rationale |
|:------|:------|:------|:----------|
| 0-4 | Standard | pi (Gemini 3) | Focused changes, few components, straightforward logic |
| 5-10 | Complex | codex | Cross-cutting concerns, many files, architectural complexity |

## What Each Factor Measures

### Acceptance Criteria Count
Counts unchecked checkboxes (`- [ ]`) in the story. More criteria means more work and more ways to get things wrong.

### Files Referenced
Counts mentions of `Ora/*.swift` or `OraTests/*.swift` paths. More files means broader impact and higher integration risk.

### Component Diversity
Counts unique top-level directories under `Ora/` referenced in the story (Audio, ASR, LLM, Tools, TTS, UI, Orchestration, etc.). Multi-component work requires understanding multiple subsystems.

### Complexity Keywords
Searches for terms like: `concurrent`, `threading`, `actor isolation`, `async sequence`, `pipeline`, `multi-step`, `streaming`, `migration`, `cross-cutting`, `state machine`, `protocol witness`, `generic constraint`, `associated type`. Presence indicates architectural complexity.

### Dependencies
Counts referenced story IDs (F.01, S.03, etc.). Stories with many dependencies often have tighter integration requirements and more context to manage.

## Cross-Agent Review

The review agent is always the opposite of the implementation agent:

| Implementer | Reviewer |
|:------------|:---------|
| pi | codex |
| codex | pi |

This ensures diverse model perspectives catch different classes of issues.

## Overriding the Score

The score is fully automatic. If the routing seems wrong for a particular story, the SKILL.md workflow documents the escalation path: if the selected agent fails, it automatically switches to the other agent.

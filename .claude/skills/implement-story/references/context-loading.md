# Context Loading Guide

Load project context based on story type to understand requirements and patterns.

## Project Documentation Paths

| Document | Path | Purpose |
|:---------|:-----|:--------|
| Stories README | `docs/stories/README.md` | Epic overview, implementation order, dependencies |
| PRD | `docs/stories/PRD.md` | Product requirements, user needs, UX principles |
| Architecture | `docs/stories/ARCHITECTURE.md` | Technical design, patterns, component details |

## Loading Strategy

1. **Always load README first** - Quick orientation on where story fits
2. **Load PRD sections** relevant to story type
3. **Load Architecture sections** relevant to story type
4. **Don't load entire files** - Use grep or read with offsets for large sections

## What to Load Per Story Type

| Story Type | Story IDs | From PRD | From Architecture |
|:-----------|:----------|:---------|:------------------|
| **Foundations/UI** | F.04, F.06, F.07 | Target Users, UX Principles, First-Run Experience | Component Diagram, UI State Machine (Section 7) |
| **Audio/ASR** | S.*, A.* | Performance Requirements | Audio Pipeline (Section 3), ASR subsections |
| **LLM** | L.* | Performance Requirements | Model Runtime (Section 4), Structured Output, System Prompt (Section 6) |
| **Tools** | X.* | v1 Features, Confirmation Gates (UX §4) | Tool Schema (Section 2), ToolHost (Section 7), Security (Section 8) |
| **TTS** | T.* | Performance Requirements | TTS subsection in Model Runtime (Section 4) |
| **Orchestration** | O.* | Full UX Principles section | Agentic Loop (Section 2), Full Component Diagram |
| **Reliability** | E.* | Stability Requirements | Error handling patterns throughout |
| **Permissions** | F.02 | Permissions & Privacy section | Security (Section 8) |
| **Model Management** | F.03 | Model Management in Settings | Model Distribution (Section 5) |

## Quick Reference: PRD Sections

```bash
# Target Users & Value Proposition (always useful)
grep -A 30 "## Target Users" docs/stories/PRD.md

# v1 Features (for tool stories)
grep -A 100 "## v1 Features" docs/stories/PRD.md

# UX Principles (for UI stories)
grep -A 60 "## User Experience Principles" docs/stories/PRD.md

# Performance Requirements
grep -A 30 "## Performance Requirements" docs/stories/PRD.md

# Permissions
grep -A 30 "## Permissions" docs/stories/PRD.md
```

## Quick Reference: Architecture Sections

```bash
# Component Diagram (always useful)
grep -A 40 "## 1. System Architecture Overview" docs/stories/ARCHITECTURE.md

# Agentic Loop (for orchestration/tools)
grep -A 100 "## 2. Agentic Loop Design" docs/stories/ARCHITECTURE.md

# Audio Pipeline
grep -A 80 "## 3. Audio Pipeline" docs/stories/ARCHITECTURE.md

# Model Runtime (LLM, TTS)
grep -A 120 "## 4. Model Runtime Strategy" docs/stories/ARCHITECTURE.md

# Swift 6 Patterns
grep -A 100 "## 7. Swift 6 Implementation" docs/stories/ARCHITECTURE.md

# Security
grep -A 50 "## 8. Security" docs/stories/ARCHITECTURE.md
```

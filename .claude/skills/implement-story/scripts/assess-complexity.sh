#!/bin/bash
#
# assess-complexity.sh - Analyze story complexity and recommend implementation agent
#
# Usage: ./assess-complexity.sh <story-path>
#
# Output: Shell-evaluable variables:
#   COMPLEXITY_SCORE  - Total score (0-10)
#   AGENT             - Recommended agent ("pi" or "codex")
#   LEVEL             - Human-readable level ("standard" or "complex")
#   AC_COUNT          - Number of acceptance criteria
#   FILES             - Number of files mentioned in implementation plan
#   COMPONENTS        - Number of distinct Ora components touched
#   COMPLEX_KEYWORDS  - Count of complexity-indicating keywords
#   DEPS              - Number of dependency stories
#
# Routing threshold: score <= 4 → pi (standard), score > 4 → codex (complex)
#
# See references/complexity-rubric.md for full scoring details.

set -e

STORY="$1"
if [ -z "$STORY" ]; then
    echo "Usage: $0 <story-path>" >&2
    exit 1
fi

if [ ! -f "$STORY" ]; then
    echo "Error: Story file not found: $STORY" >&2
    exit 1
fi

# ─── Factor 1: Acceptance Criteria Count ───────────────────────────────────────
# Unchecked checkboxes (- [ ]) indicate work to be done
# Note: Use grep | wc -l instead of grep -c to avoid multiline edge cases on macOS
AC_COUNT=$(grep -E '^[[:space:]]*- \[ \]' "$STORY" 2>/dev/null | wc -l | tr -d ' ')

# ─── Factor 2: Files Mentioned in Implementation Plan ──────────────────────────
# Lines referencing Ora/ or OraTests/ paths (backtick-wrapped or plain)
FILES=$(grep -E '(Ora|OraTests)/[A-Za-z0-9_/]+\.swift' "$STORY" 2>/dev/null | wc -l | tr -d ' ')

# ─── Factor 3: Component Diversity ────────────────────────────────────────────
# How many distinct top-level Ora components are referenced
COMPONENTS=$(grep -oE '(Ora|OraTests)/(Audio|ASR|LLM|Tools|TTS|UI|Orchestration|Utilities|Persistence|Permissions|Hotkey|Models|Overlay|Preferences|Setup)' "$STORY" 2>/dev/null | sed 's|.*/||' | sort -u | wc -l | tr -d ' ')

# ─── Factor 4: Complexity Keywords ────────────────────────────────────────────
# Keywords that signal architectural complexity
# Use grep | wc -l to count lines containing keywords (not total occurrences)
COMPLEX_KEYWORDS=$(grep -iE '(concurren|threading|actor isolation|async sequence|pipeline|multi.step|streaming|migration|cross.cutting|state machine|protocol witness|generic constraint|associated type)' "$STORY" 2>/dev/null | wc -l | tr -d ' ')

# ─── Factor 5: Dependency Count ───────────────────────────────────────────────
# Story IDs referenced (F.01, S.03, X.02, etc.)
DEPS=$(grep -oE '[A-Z]{1,3}\.[0-9]+[A-Z]?' "$STORY" 2>/dev/null | sort -u | wc -l | tr -d ' ')

# ─── Score Calculation ────────────────────────────────────────────────────────

SCORE=0

# AC criteria: 1-3 → +1, 4-6 → +2, 7+ → +3
if [ "$AC_COUNT" -le 3 ]; then
    SCORE=$((SCORE + 1))
elif [ "$AC_COUNT" -le 6 ]; then
    SCORE=$((SCORE + 2))
else
    SCORE=$((SCORE + 3))
fi

# Files: 0-2 → +1, 3-5 → +2, 6+ → +3
if [ "$FILES" -le 2 ]; then
    SCORE=$((SCORE + 1))
elif [ "$FILES" -le 5 ]; then
    SCORE=$((SCORE + 2))
else
    SCORE=$((SCORE + 3))
fi

# Components: 1 → +0, 2 → +1, 3+ → +2
if [ "$COMPONENTS" -ge 3 ]; then
    SCORE=$((SCORE + 2))
elif [ "$COMPONENTS" -ge 2 ]; then
    SCORE=$((SCORE + 1))
fi

# Complex keywords: any present → +1
if [ "$COMPLEX_KEYWORDS" -gt 0 ]; then
    SCORE=$((SCORE + 1))
fi

# Dependencies: 3+ → +1
if [ "$DEPS" -ge 3 ]; then
    SCORE=$((SCORE + 1))
fi

# ─── Routing Decision ────────────────────────────────────────────────────────

if [ "$SCORE" -le 4 ]; then
    AGENT="pi"
    LEVEL="standard"
else
    AGENT="codex"
    LEVEL="complex"
fi

# ─── Output (shell-evaluable) ────────────────────────────────────────────────

echo "COMPLEXITY_SCORE=$SCORE"
echo "AGENT=$AGENT"
echo "LEVEL=$LEVEL"
echo "AC_COUNT=$AC_COUNT"
echo "FILES=$FILES"
echo "COMPONENTS=$COMPONENTS"
echo "COMPLEX_KEYWORDS=$COMPLEX_KEYWORDS"
echo "DEPS=$DEPS"

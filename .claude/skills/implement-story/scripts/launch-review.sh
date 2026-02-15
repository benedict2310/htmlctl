#!/bin/bash
# launch-review.sh
# Launches a code review subagent (supports both codex and pi)
#
# Usage: ./launch-review.sh <story-path> [agent: pi|codex] [--quiet|--verbose]
#
# If no agent is specified, defaults to pi.
#
# This script:
# 1. Loads the code review prompt from references/
# 2. Captures the git diff and story content
# 3. Launches the selected review agent in non-interactive mode
# 4. Saves output to docs/review-logs/

set -e

# ─── Validate Input ───────────────────────────────────────────────────────────

STORY="$1"
shift || true

AGENT="pi"
QUIET=0

if [ "$1" = "pi" ] || [ "$1" = "codex" ]; then
    AGENT="$1"
    shift || true
fi

for arg in "$@"; do
    case "$arg" in
        --quiet)
            QUIET=1
            ;;
        --verbose)
            QUIET=0
            ;;
    esac
done

if [ -z "$STORY" ]; then
    echo "Usage: $0 <story-path> [pi|codex] [--quiet|--verbose]"
    echo ""
    echo "Examples:"
    echo "  $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md"
    echo "  $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md codex"
    exit 1
fi

if [ ! -f "$STORY" ]; then
    echo "Error: Story file not found: $STORY"
    exit 1
fi

if [ "$AGENT" != "pi" ] && [ "$AGENT" != "codex" ]; then
    echo "Error: Unknown agent '$AGENT'. Expected 'pi' or 'codex'."
    exit 1
fi

# Verify agent is installed
if ! which "$AGENT" >/dev/null 2>&1; then
    if [ "$AGENT" = "pi" ]; then
        FALLBACK="codex"
    else
        FALLBACK="pi"
    fi

    if which "$FALLBACK" >/dev/null 2>&1; then
        echo "WARNING: $AGENT not found, falling back to $FALLBACK for review"
        AGENT="$FALLBACK"
    else
        echo "Error: Neither pi nor codex is installed."
        exit 1
    fi
fi

# ─── Gather Context ──────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"
PROMPT_FILE="$SCRIPT_DIR/../references/code-review-prompt.md"
REVIEW_LOGS_DIR="$PROJECT_ROOT/docs/review-logs"

if [ ! -f "$PROMPT_FILE" ]; then
    echo "Error: Code review prompt not found: $PROMPT_FILE"
    exit 1
fi

mkdir -p "$REVIEW_LOGS_DIR"

BRANCH=$(git branch --show-current)
COMMIT=$(git rev-parse --short HEAD)
FILES_CHANGED=$(git diff --name-only main...HEAD 2>/dev/null | wc -l | tr -d ' ')

# Validate we're not on main
if [ "$BRANCH" = "main" ]; then
    echo "Error: Cannot review from main branch. Create a feature branch first."
    exit 1
fi

# Check for changes to review
if [ "$FILES_CHANGED" = "0" ]; then
    echo "Error: No changes found between main and $BRANCH"
    exit 1
fi

# ─── Generate Log File ───────────────────────────────────────────────────────

STORY_BASENAME=$(basename "$STORY" .md)
TIMESTAMP=$(date +"%Y-%m-%d-%H%M%S")
LOG_FILE="$REVIEW_LOGS_DIR/${STORY_BASENAME}-review-${AGENT}-${TIMESTAMP}.log"

if [ "$QUIET" -eq 1 ]; then
    echo "Launching review subagent (quiet). Log: $LOG_FILE"
else
    echo "=========================================="
    echo "Launching Code Review Subagent"
    echo "=========================================="
    echo "Story:   $STORY"
    echo "Agent:   $AGENT"
    echo "Branch:  $BRANCH"
    echo "Commit:  $COMMIT"
    echo "Files:   $FILES_CHANGED changed"
    echo "Log:     $LOG_FILE"
    echo "=========================================="
    echo ""
fi

# ─── Build Prompt ─────────────────────────────────────────────────────────────

TEMP_PROMPT=$(mktemp)
trap "rm -f $TEMP_PROMPT" EXIT

cat > "$TEMP_PROMPT" << PROMPT_END
You are performing a code review for the story: $STORY

## Instructions

$(cat "$PROMPT_FILE")

---

## STORY DOCUMENT

$(cat "$STORY")

---

## CONTEXT

- **Story file path:** $STORY
- **Branch:** $BRANCH
- **Commit:** $COMMIT
- **Files changed:** $FILES_CHANGED

---

## DIFF

$(git diff main...HEAD)

---

Now perform the code review following the instructions above.

CRITICAL: When writing findings to the story file at $STORY, you MUST preserve all existing content. Read the file first, then use a targeted edit to update ONLY the "## Code Review Findings" section. NEVER overwrite or delete the rest of the story (sections 1-10, Implementation Summary, etc.).
PROMPT_END

# ─── Write Log Header ────────────────────────────────────────────────────────

{
    echo "=========================================="
    echo "Code Review Log"
    echo "=========================================="
    echo "Story:     $STORY"
    echo "Agent:     $AGENT"
    echo "Branch:    $BRANCH"
    echo "Commit:    $COMMIT"
    echo "Files:     $FILES_CHANGED changed"
    echo "Timestamp: $(date)"
    echo "=========================================="
    echo ""
} > "$LOG_FILE"

# ─── Launch Agent ─────────────────────────────────────────────────────────────

if [ "$AGENT" = "pi" ]; then
    if [ "$QUIET" -eq 1 ]; then
        pi --model gemini-3-pro-high --print --no-session "@$TEMP_PROMPT" >> "$LOG_FILE" 2>&1
    else
        pi --model gemini-3-pro-high --print --no-session "@$TEMP_PROMPT" 2>&1 | tee -a "$LOG_FILE"
    fi
elif [ "$AGENT" = "codex" ]; then
    if [ "$QUIET" -eq 1 ]; then
        codex exec --sandbox danger-full-access "$(cat "$TEMP_PROMPT")" >> "$LOG_FILE" 2>&1
    else
        codex exec --sandbox danger-full-access "$(cat "$TEMP_PROMPT")" 2>&1 | tee -a "$LOG_FILE"
    fi
fi

if [ "$QUIET" -eq 1 ]; then
    echo "Review complete. Log: $LOG_FILE"
else
    echo ""
    echo "=========================================="
    echo "Review Complete"
    echo "=========================================="
    echo "Agent:   $AGENT"
    echo "Check the story file for findings: $STORY"
    echo "Review log saved to: $LOG_FILE"
    echo ""
fi

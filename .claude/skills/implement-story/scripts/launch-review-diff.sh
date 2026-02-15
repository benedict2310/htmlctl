#!/bin/bash
# launch-review-diff.sh
# Launches a code review subagent based on a specific commit range.
# Use this when the feature branch is already merged or deleted.
#
# Usage: ./launch-review-diff.sh <story-path> <from-commit> <to-commit> [--agent pi|codex] [--quiet|--verbose]
#
# This script:
# 1. Loads the code review prompt from references/
# 2. Captures the git diff between provided commits and story content
# 3. Launches the review agent in non-interactive mode
# 4. Saves output to docs/review-logs/

set -e

# Validate input
STORY="$1"
FROM_COMMIT="$2"
TO_COMMIT="$3"
AGENT="pi"
QUIET=0

shift 3 || true
while [ $# -gt 0 ]; do
    case "$1" in
        --agent)
            AGENT="$2"
            shift 2
            ;;
        --quiet)
            QUIET=1
            shift
            ;;
        --verbose)
            QUIET=0
            shift
            ;;
        -h|--help)
            echo "Usage: $0 <story-path> <from-commit> <to-commit> [--agent pi|codex] [--quiet|--verbose]"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

if [ -z "$STORY" ] || [ -z "$FROM_COMMIT" ] || [ -z "$TO_COMMIT" ]; then
    echo "Usage: $0 <story-path> <from-commit> <to-commit> [--agent pi|codex] [--quiet|--verbose]"
    echo "Example: $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md origin/main HEAD"
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

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"
PROMPT_FILE="$SCRIPT_DIR/../references/code-review-prompt.md"
REVIEW_LOGS_DIR="$PROJECT_ROOT/docs/review-logs"

if [ ! -f "$PROMPT_FILE" ]; then
    echo "Error: Code review prompt not found: $PROMPT_FILE"
    exit 1
fi

# Create review logs directory if it doesn't exist
mkdir -p "$REVIEW_LOGS_DIR"

# Gather context
BRANCH="main (merged)"
COMMIT="$TO_COMMIT"
FILES_CHANGED=$(git diff --name-only "$FROM_COMMIT..$TO_COMMIT" 2>/dev/null | wc -l | tr -d ' ')

# Check for changes to review
if [ "$FILES_CHANGED" = "0" ]; then
    echo "Error: No changes found between $FROM_COMMIT and $TO_COMMIT"
    exit 1
fi

# Generate log file name from story (e.g., F.05-GLOBAL-HOTKEY -> F.05-GLOBAL-HOTKEY-2025-01-15-143022.log)
STORY_BASENAME=$(basename "$STORY" .md)
TIMESTAMP=$(date +"%Y-%m-%d-%H%M%S")
LOG_FILE="$REVIEW_LOGS_DIR/${STORY_BASENAME}-${TIMESTAMP}.log"

if [ "$QUIET" -eq 1 ]; then
    echo "Launching post-merge review (quiet). Log: $LOG_FILE"
else
    echo "=========================================="
    echo "Launching Code Review Subagent (Post-Merge)"
    echo "=========================================="
    echo "Story:   $STORY"
    echo "Range:   $FROM_COMMIT..$TO_COMMIT"
    echo "Files:   $FILES_CHANGED changed"
    echo "Log:     $LOG_FILE"
    echo "=========================================="
    echo ""
fi

# Create a temporary file with the full prompt
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
- **Range:** $FROM_COMMIT..$TO_COMMIT
- **Files changed:** $FILES_CHANGED

---

## DIFF

$(git diff "$FROM_COMMIT..$TO_COMMIT")

---

Now perform the code review following the instructions above.

CRITICAL: When writing findings to the story file at $STORY, you MUST preserve all existing content. Read the file first, then use a targeted edit to update ONLY the "## Code Review Findings" section. NEVER overwrite or delete the rest of the story (sections 1-10, Implementation Summary, etc.).
PROMPT_END

# Write header to log file
{
    echo "=========================================="
    echo "Code Review Log (Post-Merge)"
    echo "=========================================="
    echo "Story:     $STORY"
    echo "Range:     $FROM_COMMIT..$TO_COMMIT"
    echo "Files:     $FILES_CHANGED changed"
    echo "Timestamp: $(date)"
    echo "=========================================="
    echo ""
} > "$LOG_FILE"

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
    echo "Check the story file for findings: $STORY"
    echo "Review log saved to: $LOG_FILE"
    echo ""
fi

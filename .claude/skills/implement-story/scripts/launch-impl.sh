#!/bin/bash
#
# launch-impl.sh - Launch implementation subagent
#
# Usage:
#   ./launch-impl.sh <story-path> <agent: pi|codex> [--quiet|--verbose]          # Full implementation
#   ./launch-impl.sh <story-path> <agent: pi|codex> --fix [--quiet|--verbose]    # Fix review findings
#
# This script:
# 1. Gathers story content, project map, and branch context
# 2. Builds a comprehensive prompt from the implementation template
# 3. Launches the selected agent in non-interactive mode
# 4. Logs output to docs/impl-logs/
#
# Agents:
#   pi    → pi --model gemini-3-pro-high [--print] --no-session
#   codex → codex exec --sandbox danger-full-access -m gpt-5.3-codex -c model_reasoning_effort="high"

set -e

# ─── Validate Input ───────────────────────────────────────────────────────────

STORY="$1"
AGENT="$2"
MODE="impl"
QUIET=0

shift 2 || true
for arg in "$@"; do
    case "$arg" in
        --fix)
            MODE="fix"
            ;;
        --quiet)
            QUIET=1
            ;;
        --verbose)
            QUIET=0
            ;;
    esac
done

if [ -z "$STORY" ] || [ -z "$AGENT" ]; then
    echo "Usage: $0 <story-path> <pi|codex> [--fix] [--quiet|--verbose]"
    echo ""
    echo "Examples:"
    echo "  $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md pi"
    echo "  $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md codex --fix"
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
    # Try the other agent as fallback
    if [ "$AGENT" = "pi" ]; then
        FALLBACK="codex"
    else
        FALLBACK="pi"
    fi

    if which "$FALLBACK" >/dev/null 2>&1; then
        echo "WARNING: $AGENT not found, falling back to $FALLBACK"
        AGENT="$FALLBACK"
    else
        echo "Error: Neither pi nor codex is installed."
        exit 1
    fi
fi

# ─── Gather Context ──────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"
IMPL_LOGS_DIR="$PROJECT_ROOT/docs/impl-logs"

IMPL_PROMPT_FILE="$SCRIPT_DIR/../references/implementation-prompt.md"
FIX_PROMPT_FILE="$SCRIPT_DIR/../references/fix-prompt.md"

BRANCH=$(git branch --show-current)
COMMIT=$(git rev-parse --short HEAD)

# Create logs directory
mkdir -p "$IMPL_LOGS_DIR"

# Generate project map summary for prompt
PROJECT_MAP=$("$SCRIPT_DIR/project-map.sh" --summary --no-color 2>/dev/null || echo "(project map unavailable)")

# ─── Build Prompt ─────────────────────────────────────────────────────────────

TEMP_PROMPT=$(mktemp)
trap "rm -f $TEMP_PROMPT" EXIT

if [ "$MODE" = "fix" ]; then
    # Fix mode: include review findings
    if [ ! -f "$FIX_PROMPT_FILE" ]; then
        echo "Error: Fix prompt template not found: $FIX_PROMPT_FILE"
        exit 1
    fi

    # Extract review findings from story file
    REVIEW_FINDINGS=$(sed -n '/## Code Review Findings/,/## [^C]/p' "$STORY" | head -100)

    cat > "$TEMP_PROMPT" << PROMPT_END
$(cat "$FIX_PROMPT_FILE")

---

## REVIEW FINDINGS TO FIX

$REVIEW_FINDINGS

---

## STORY DOCUMENT

$(cat "$STORY")

---

## PROJECT STRUCTURE

$PROJECT_MAP

---

## CONTEXT

- **Story file path:** $STORY
- **Branch:** $BRANCH
- **Current commit:** $COMMIT
- **Working directory:** $PROJECT_ROOT
- **Mode:** Fix review findings only

---

Now fix the review findings listed above. Only modify code related to the identified issues.
Do NOT refactor or improve anything outside the scope of these findings.
PROMPT_END

else
    # Full implementation mode
    if [ ! -f "$IMPL_PROMPT_FILE" ]; then
        echo "Error: Implementation prompt template not found: $IMPL_PROMPT_FILE"
        exit 1
    fi

    cat > "$TEMP_PROMPT" << PROMPT_END
$(cat "$IMPL_PROMPT_FILE")

---

## STORY DOCUMENT

$(cat "$STORY")

---

## PROJECT STRUCTURE

$PROJECT_MAP

---

## CONTEXT

- **Story file path:** $STORY
- **Branch:** $BRANCH
- **Current commit:** $COMMIT
- **Working directory:** $PROJECT_ROOT
- **Mode:** Full implementation

---

Now implement the story following the instructions above.
PROMPT_END
fi

# ─── Generate Log File Name ──────────────────────────────────────────────────

STORY_BASENAME=$(basename "$STORY" .md)
TIMESTAMP=$(date +"%Y-%m-%d-%H%M%S")
LOG_FILE="$IMPL_LOGS_DIR/${STORY_BASENAME}-${MODE}-${AGENT}-${TIMESTAMP}.log"

# ─── Launch Agent ─────────────────────────────────────────────────────────────

if [ "$QUIET" -eq 1 ]; then
    echo "Launching implementation subagent (quiet). Log: $LOG_FILE"
else
    echo "=========================================="
    echo "Launching Implementation Subagent"
    echo "=========================================="
    echo "Story:   $STORY"
    echo "Agent:   $AGENT"
    echo "Mode:    $MODE"
    echo "Branch:  $BRANCH"
    echo "Log:     $LOG_FILE"
    echo "=========================================="
    echo ""
fi

# Write header to log file
{
    echo "=========================================="
    echo "Implementation Log"
    echo "=========================================="
    echo "Story:     $STORY"
    echo "Agent:     $AGENT"
    echo "Mode:      $MODE"
    echo "Branch:    $BRANCH"
    echo "Commit:    $COMMIT"
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
    # codex reads from stdin or takes prompt as argument
    if [ "$QUIET" -eq 1 ]; then
        codex exec --sandbox danger-full-access -m gpt-5.3-codex -c model_reasoning_effort="high" "$(cat "$TEMP_PROMPT")" >> "$LOG_FILE" 2>&1
    else
        codex exec --sandbox danger-full-access -m gpt-5.3-codex -c model_reasoning_effort="high" "$(cat "$TEMP_PROMPT")" 2>&1 | tee -a "$LOG_FILE"
    fi
fi

EXIT_CODE=$?

if [ "$QUIET" -eq 1 ]; then
    echo "Implementation complete. Exit code: $EXIT_CODE. Log: $LOG_FILE"
else
    echo ""
    echo "=========================================="
    echo "Implementation Subagent Complete"
    echo "=========================================="
    echo "Exit code: $EXIT_CODE"
    echo "Log saved: $LOG_FILE"
    echo ""

    # Report commit count
    COMMITS_AFTER=$(git log --oneline main..HEAD 2>/dev/null | wc -l | tr -d ' ')
    echo "Commits on branch: $COMMITS_AFTER"
fi

exit $EXIT_CODE

#!/bin/bash
#
# preflight.sh - Validate a story is ready for implementation
#
# Usage: ./preflight.sh <story-path>
#
# Exit codes:
#   0 = Ready to implement
#   1 = Blocked (dependencies not complete or missing required fields)
#   2 = Warnings only (can proceed with caution)
#
# This script checks:
#   1. Story file exists and has required metadata
#   2. Story is not already complete
#   3. All dependencies are complete
#   4. Acceptance criteria exist
#

set -e

QUIET=0
NO_COLOR=0
STORY=""

usage() {
    echo "Usage: $0 <story-path> [--quiet] [--no-color]"
    echo "Example: $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md --quiet"
}

for arg in "$@"; do
    case "$arg" in
        --quiet)
            QUIET=1
            NO_COLOR=1
            ;;
        --no-color|--plain)
            NO_COLOR=1
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            if [ -z "$STORY" ]; then
                STORY="$arg"
            fi
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

if [ "$NO_COLOR" -eq 1 ]; then
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# Validate input
if [ -z "$STORY" ]; then
    if [ "$QUIET" -eq 1 ]; then
        echo "PRE-FLIGHT: ERROR (missing story path)"
    else
        echo -e "${RED}Usage: $0 <story-path>${NC}"
        echo "Example: $0 docs/stories/foundations/F.07-OVERLAY-WINDOW.md"
    fi
    exit 1
fi

if [ ! -f "$STORY" ]; then
    if [ "$QUIET" -eq 1 ]; then
        echo "PRE-FLIGHT: ERROR (story not found: $STORY)"
    else
        echo -e "${RED}Error: Story file not found: $STORY${NC}"
    fi
    exit 1
fi

# Get project root (assumes script is in .claude/skills/implement-story/scripts/)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"
STORIES_DIR="$PROJECT_ROOT/docs/stories"

if [ "$QUIET" -eq 0 ]; then
    echo "=========================================="
    echo -e "${BLUE}Pre-Flight Check${NC}"
    echo "=========================================="
    echo "Story: $STORY"
    echo "=========================================="
    echo ""
fi

BLOCKERS=()
WARNINGS=()

# -----------------------------------------------------------------------------
# Helper: Extract metadata field from story
# -----------------------------------------------------------------------------
extract_field() {
    local field="$1"
    local file="$2"
    grep -E "^\*\*${field}:\*\*" "$file" | sed "s/\*\*${field}:\*\* *//" | head -1
}

# -----------------------------------------------------------------------------
# Check 1: Required Metadata
# -----------------------------------------------------------------------------
if [ "$QUIET" -eq 0 ]; then
    echo -e "${BLUE}[1/4] Checking required metadata...${NC}"
fi

STATUS=$(extract_field "Status" "$STORY")
PRIORITY=$(extract_field "Priority" "$STORY")
DEPENDENCIES=$(extract_field "Dependencies" "$STORY")

if [ -z "$STATUS" ]; then
    BLOCKERS+=("Missing required field: Status")
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${RED}✗ Status: MISSING${NC}"
    fi
else
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ Status: $STATUS${NC}"
    fi
fi

if [ -z "$PRIORITY" ]; then
    WARNINGS+=("Missing field: Priority")
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${YELLOW}! Priority: MISSING (warning)${NC}"
    fi
else
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ Priority: $PRIORITY${NC}"
    fi
fi

if [ -z "$DEPENDENCIES" ]; then
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ Dependencies: None${NC}"
    fi
else
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ Dependencies: $DEPENDENCIES${NC}"
    fi
fi

if [ "$QUIET" -eq 0 ]; then
    echo ""
fi

# -----------------------------------------------------------------------------
# Check 2: Story Status
# -----------------------------------------------------------------------------
if [ "$QUIET" -eq 0 ]; then
    echo -e "${BLUE}[2/4] Checking story status...${NC}"
fi

if [[ "$STATUS" == *"Complete"* ]] || [[ "$STATUS" == *"Implemented"* ]]; then
    BLOCKERS+=("Story is already marked as Complete")
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${RED}✗ Story is already Complete - nothing to implement${NC}"
    fi
elif [[ "$STATUS" == *"In Progress"* ]]; then
    WARNINGS+=("Story is marked In Progress - resuming previous work?")
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${YELLOW}! Story is In Progress - resuming?${NC}"
    fi
else
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ Story is ready to start ($STATUS)${NC}"
    fi
fi

if [ "$QUIET" -eq 0 ]; then
    echo ""
fi

# -----------------------------------------------------------------------------
# Check 3: Dependencies Complete
# -----------------------------------------------------------------------------
if [ "$QUIET" -eq 0 ]; then
    echo -e "${BLUE}[3/4] Checking dependencies...${NC}"
fi

if [ -z "$DEPENDENCIES" ] || [ "$DEPENDENCIES" == "None" ]; then
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ No dependencies${NC}"
    fi
else
    # Parse dependencies - they come in formats like:
    # "F.01 (App Shell), F.05 (Global Hotkey)"
    # "Parakeet S.01, S.03 (Core Engine, Streaming)"
    # "F.02 (Permissions)"
    
    # Extract story IDs (patterns like F.01, S.03, A.02, X.01, etc.)
    DEP_IDS=$(echo "$DEPENDENCIES" | grep -oE '[A-Z]{1,3}\.[0-9]+[A-Z]?' | sort -u)
    
    if [ -z "$DEP_IDS" ]; then
        if [ "$QUIET" -eq 0 ]; then
            echo -e "  ${YELLOW}! Could not parse dependency IDs from: $DEPENDENCIES${NC}"
        fi
        WARNINGS+=("Could not parse dependency IDs")
    else
        for DEP_ID in $DEP_IDS; do
            # Find the story file for this dependency
            DEP_FILE=$(find "$STORIES_DIR" -name "${DEP_ID}*.md" -type f 2>/dev/null | head -1)
            
            if [ -z "$DEP_FILE" ]; then
                WARNINGS+=("Dependency $DEP_ID: story file not found")
                if [ "$QUIET" -eq 0 ]; then
                    echo -e "  ${YELLOW}! $DEP_ID: Story file not found${NC}"
                fi
            else
                DEP_STATUS=$(extract_field "Status" "$DEP_FILE")
                DEP_NAME=$(basename "$DEP_FILE" .md)
                
                if [[ "$DEP_STATUS" == *"Complete"* ]] || [[ "$DEP_STATUS" == *"Implemented"* ]]; then
                    if [ "$QUIET" -eq 0 ]; then
                        echo -e "  ${GREEN}✓ $DEP_ID ($DEP_NAME): $DEP_STATUS${NC}"
                    fi
                else
                    BLOCKERS+=("Dependency $DEP_ID is not complete (Status: $DEP_STATUS)")
                    if [ "$QUIET" -eq 0 ]; then
                        echo -e "  ${RED}✗ $DEP_ID ($DEP_NAME): $DEP_STATUS${NC}"
                    fi
                fi
            fi
        done
    fi
fi

if [ "$QUIET" -eq 0 ]; then
    echo ""
fi

# -----------------------------------------------------------------------------
# Check 4: Acceptance Criteria
# -----------------------------------------------------------------------------
if [ "$QUIET" -eq 0 ]; then
    echo -e "${BLUE}[4/4] Checking acceptance criteria...${NC}"
fi

# Look for acceptance criteria section or checkbox list
if grep -qE "^## .*[Aa]cceptance|^- \[ \]|^- \[x\]" "$STORY"; then
    # Count checkboxes
    TOTAL_CRITERIA=$(grep -cE "^- \[ \]|^- \[x\]" "$STORY" 2>/dev/null | tr -d '\n' || echo "0")
    COMPLETED=$(grep -cE "^- \[x\]" "$STORY" 2>/dev/null | tr -d '\n' || echo "0")
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${GREEN}✓ Found acceptance criteria (${COMPLETED}/${TOTAL_CRITERIA} completed)${NC}"
    fi
else
    WARNINGS+=("No acceptance criteria checkboxes found")
    if [ "$QUIET" -eq 0 ]; then
        echo -e "  ${YELLOW}! No acceptance criteria checkboxes found${NC}"
    fi
fi

if [ "$QUIET" -eq 0 ]; then
    echo ""
fi

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
if [ "$QUIET" -eq 0 ]; then
    echo "=========================================="
    echo -e "${BLUE}Summary${NC}"
    echo "=========================================="
fi

if [ ${#BLOCKERS[@]} -gt 0 ]; then
    if [ "$QUIET" -eq 1 ]; then
        echo "PRE-FLIGHT: BLOCKED"
        for blocker in "${BLOCKERS[@]}"; do
            echo "BLOCKER: $blocker"
        done
        for warning in "${WARNINGS[@]}"; do
            echo "WARNING: $warning"
        done
    else
        echo -e "${RED}BLOCKED - Cannot proceed${NC}"
        echo ""
        echo "Blockers:"
        for blocker in "${BLOCKERS[@]}"; do
            echo -e "  ${RED}• $blocker${NC}"
        done
        if [ ${#WARNINGS[@]} -gt 0 ]; then
            echo ""
            echo "Warnings:"
            for warning in "${WARNINGS[@]}"; do
                echo -e "  ${YELLOW}• $warning${NC}"
            done
        fi
        echo ""
    fi
    exit 1
elif [ ${#WARNINGS[@]} -gt 0 ]; then
    if [ "$QUIET" -eq 1 ]; then
        echo "PRE-FLIGHT: WARNINGS"
        for warning in "${WARNINGS[@]}"; do
            echo "WARNING: $warning"
        done
    else
        echo -e "${YELLOW}READY WITH WARNINGS${NC}"
        echo ""
        echo "Warnings:"
        for warning in "${WARNINGS[@]}"; do
            echo -e "  ${YELLOW}• $warning${NC}"
        done
        echo ""
        echo -e "${GREEN}You may proceed, but review the warnings above.${NC}"
    fi
    exit 2
else
    if [ "$QUIET" -eq 1 ]; then
        echo "PRE-FLIGHT: READY"
    else
        echo -e "${GREEN}READY TO IMPLEMENT${NC}"
        echo ""
        echo "All checks passed. You may proceed with implementation."
    fi
    exit 0
fi

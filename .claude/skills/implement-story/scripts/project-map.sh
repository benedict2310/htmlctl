#!/bin/bash
#
# project-map.sh - Generate a map of the current project structure
#
# Usage: ./project-map.sh [--json]
#
# This script:
#   1. Uses git ls-files to get tracked files (respects .gitignore)
#   2. Groups Swift files by directory
#   3. Shows what components exist and their file counts
#   4. Identifies which ARCHITECTURE components are implemented
#
# Output:
#   - Human-readable summary to stdout
#   - Optional JSON output with --json flag
#

set -e

JSON_OUTPUT=false
SUMMARY_OUTPUT=false
NO_COLOR=false

usage() {
    echo "Usage: $0 [--json] [--summary] [--no-color]"
}

while [ $# -gt 0 ]; do
    case "$1" in
        --json)
            JSON_OUTPUT=true
            ;;
        --summary)
            SUMMARY_OUTPUT=true
            ;;
        --no-color|--plain)
            NO_COLOR=true
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Error: Unknown argument: $1"
            usage
            exit 1
            ;;
    esac
    shift
done

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
DIM='\033[2m'
NC='\033[0m' # No Color

if [ "$NO_COLOR" = true ]; then
    GREEN=''
    YELLOW=''
    BLUE=''
    DIM=''
    NC=''
fi

# Get project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"

cd "$PROJECT_ROOT"

# Check if we're in a git repo
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "Error: Not in a git repository"
    exit 1
fi

# -----------------------------------------------------------------------------
# Gather Data
# -----------------------------------------------------------------------------

# Get all tracked Swift files
SWIFT_FILES=$(git ls-files '*.swift' 2>/dev/null)

# Get unique directories under Ora/ (main source)
ORA_DIRS=$(echo "$SWIFT_FILES" | grep "^Ora/" | sed 's|/[^/]*$||' | sort -u)

# Get test files
TEST_FILES=$(echo "$SWIFT_FILES" | grep "^OraTests/" || true)

# Expected components from ARCHITECTURE.md
EXPECTED_COMPONENTS=(
    "Audio"
    "ASR"
    "LLM"
    "Tools"
    "TTS"
    "Orchestration"
    "Hotkey"
    "Models"
    "Overlay"
    "Permissions"
    "Persistence"
    "Preferences"
    "Setup"
    "UI"
)

# -----------------------------------------------------------------------------
# JSON Output
# -----------------------------------------------------------------------------

if [ "$JSON_OUTPUT" = true ]; then
    echo "{"
    echo "  \"generated_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
    echo "  \"project_root\": \"$PROJECT_ROOT\","
    echo "  \"components\": {"
    
    first=true
    for component in "${EXPECTED_COMPONENTS[@]}"; do
        dir="Ora/$component"
        files=$(echo "$SWIFT_FILES" | grep "^${dir}/" 2>/dev/null || true)
        if [ -n "$files" ]; then
            file_count=$(echo "$files" | wc -l | tr -d ' ')
        else
            file_count=0
        fi
        
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        
        echo -n "    \"$component\": {"
        echo -n "\"implemented\": $([ "$file_count" -gt 0 ] && echo "true" || echo "false"), "
        echo -n "\"file_count\": $file_count, "
        echo -n "\"files\": ["
        
        if [ -n "$files" ] && [ "$file_count" -gt 0 ]; then
            echo "$files" | awk -v dir="$dir" '{
                gsub(dir"/", ""); 
                if (NR > 1) printf ", ";
                printf "\"%s\"", $0
            }'
        fi
        echo -n "]}"
    done
    
    echo ""
    echo "  },"
    
    # Tests
    echo "  \"tests\": {"
    test_count=$(echo "$TEST_FILES" | grep -c . 2>/dev/null || echo "0")
    echo "    \"file_count\": $test_count,"
    echo -n "    \"files\": ["
    if [ -n "$TEST_FILES" ] && [ "$test_count" -gt 0 ]; then
        echo "$TEST_FILES" | awk '{
            gsub("OraTests/", ""); 
            if (NR > 1) printf ", ";
            printf "\"%s\"", $0
        }'
    fi
    echo "]"
    echo "  }"
    echo "}"
    exit 0
fi

# -----------------------------------------------------------------------------
# Summary Output
# -----------------------------------------------------------------------------

if [ "$SUMMARY_OUTPUT" = true ]; then
    TOTAL_SWIFT=$(echo "$SWIFT_FILES" | grep -c . 2>/dev/null || echo "0")
    ORA_SWIFT=$(echo "$SWIFT_FILES" | grep -c "^Ora/" 2>/dev/null || echo "0")
    TEST_SWIFT=$(echo "$SWIFT_FILES" | grep -c "^OraTests/" 2>/dev/null || echo "0")

    echo "Ora Project Map (summary)"
    echo "Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Project: $PROJECT_ROOT"
    echo "Swift files: total=$TOTAL_SWIFT, Ora/=$ORA_SWIFT, OraTests/=$TEST_SWIFT"

    IMPLEMENTED=()
    MISSING=()
    for component in "${EXPECTED_COMPONENTS[@]}"; do
        dir="Ora/$component"
        files=$(echo "$SWIFT_FILES" | grep "^${dir}/" 2>/dev/null || true)
        if [ -n "$files" ]; then
            file_count=$(echo "$files" | wc -l | tr -d ' ')
        else
            file_count=0
        fi

        if [ "$file_count" -gt 0 ]; then
            IMPLEMENTED+=("${component}(${file_count})")
        else
            MISSING+=("${component}")
        fi
    done

    if [ ${#IMPLEMENTED[@]} -gt 0 ]; then
        echo "Implemented components: ${IMPLEMENTED[*]}"
    else
        echo "Implemented components: none"
    fi

    if [ ${#MISSING[@]} -gt 0 ]; then
        echo "Missing components: ${MISSING[*]}"
    else
        echo "Missing components: none"
    fi

    exit 0
fi

# -----------------------------------------------------------------------------
# Human-Readable Output
# -----------------------------------------------------------------------------

echo "=========================================="
echo -e "${BLUE}Ora Project Map${NC}"
echo "=========================================="
echo -e "${DIM}Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)${NC}"
echo -e "${DIM}Project: $PROJECT_ROOT${NC}"
echo "=========================================="
echo ""

# Summary counts
TOTAL_SWIFT=$(echo "$SWIFT_FILES" | grep -c . 2>/dev/null || echo "0")
ORA_SWIFT=$(echo "$SWIFT_FILES" | grep -c "^Ora/" 2>/dev/null || echo "0")
TEST_SWIFT=$(echo "$SWIFT_FILES" | grep -c "^OraTests/" 2>/dev/null || echo "0")

echo -e "${BLUE}Summary${NC}"
echo "  Total Swift files: $TOTAL_SWIFT"
echo "  Source files (Ora/): $ORA_SWIFT"
echo "  Test files (OraTests/): $TEST_SWIFT"
echo ""

# Component status
echo -e "${BLUE}Components${NC}"
echo ""
printf "  %-20s %-12s %s\n" "COMPONENT" "STATUS" "FILES"
printf "  %-20s %-12s %s\n" "---------" "------" "-----"

for component in "${EXPECTED_COMPONENTS[@]}"; do
    dir="Ora/$component"
    files=$(echo "$SWIFT_FILES" | grep "^${dir}/" 2>/dev/null || true)
    if [ -n "$files" ]; then
        file_count=$(echo "$files" | wc -l | tr -d ' ')
    else
        file_count=0
    fi
    
    if [ "$file_count" -gt 0 ]; then
        status="${GREEN}✓ Implemented${NC}"
    else
        status="${DIM}○ Not started${NC}"
    fi
    
    printf "  %-20s ${status}   %s\n" "$component" "$file_count"
done

echo ""

# Detailed file list for implemented components
echo -e "${BLUE}Implemented Components (Detail)${NC}"
echo ""

for component in "${EXPECTED_COMPONENTS[@]}"; do
    dir="Ora/$component"
    files=$(echo "$SWIFT_FILES" | grep "^${dir}/" 2>/dev/null || true)
    if [ -n "$files" ]; then
        file_count=$(echo "$files" | wc -l | tr -d ' ')
    else
        file_count=0
    fi
    
    if [ "$file_count" -gt 0 ]; then
        echo -e "  ${GREEN}$component/${NC} ($file_count files)"
        echo "$files" | sed "s|${dir}/|    |" | while read -r file; do
            echo "    $file"
        done
        echo ""
    fi
done

# Root files
echo -e "${BLUE}Root Files (Ora/)${NC}"
ROOT_FILES=$(echo "$SWIFT_FILES" | grep "^Ora/[^/]*\.swift$" || true)
if [ -n "$ROOT_FILES" ]; then
    echo "$ROOT_FILES" | sed 's|Ora/|  |'
else
    echo "  (none)"
fi
echo ""

# Test files
echo -e "${BLUE}Test Files (OraTests/)${NC}"
if [ -n "$TEST_FILES" ]; then
    echo "$TEST_FILES" | sed 's|OraTests/|  |'
else
    echo "  (none)"
fi
echo ""

# Not-yet-implemented components
echo -e "${BLUE}Not Yet Implemented${NC}"
echo -e "${DIM}  These components are defined in ARCHITECTURE.md but have no files yet:${NC}"
for component in "${EXPECTED_COMPONENTS[@]}"; do
    dir="Ora/$component"
    files=$(echo "$SWIFT_FILES" | grep "^${dir}/" 2>/dev/null || true)
    if [ -n "$files" ]; then
        file_count=$(echo "$files" | wc -l | tr -d ' ')
    else
        file_count=0
    fi
    
    if [ "$file_count" -eq 0 ]; then
        echo "  • $component/"
    fi
done
echo ""

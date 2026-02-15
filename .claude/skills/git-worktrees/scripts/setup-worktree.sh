#!/bin/bash
# Setup a git worktree for parallel development
# Usage: ./setup-worktree.sh <branch-name> [--existing]
#   branch-name: Branch to create/use (e.g., feat/calendar or fix/audio)
#   --existing: Use existing branch instead of creating new one

set -e

BRANCH="$1"
EXISTING="$2"

if [ -z "$BRANCH" ]; then
    echo "Usage: $0 <branch-name> [--existing]"
    echo "  branch-name: e.g., feat/calendar-improvements"
    echo "  --existing: checkout existing branch instead of creating new"
    exit 1
fi

# Get repo root and worktree dir
REPO_ROOT="$(git rev-parse --show-toplevel)"
WORKTREE_BASE="$(dirname "$REPO_ROOT")/ora-worktrees"
WORKTREE_NAME="${BRANCH//\//-}"  # feat/xyz -> feat-xyz
WORKTREE_PATH="$WORKTREE_BASE/$WORKTREE_NAME"

echo "Setting up worktree for branch: $BRANCH"
echo "  Location: $WORKTREE_PATH"

# Ensure main is up to date
echo "Fetching latest from origin..."
git fetch origin

# Create worktree
mkdir -p "$WORKTREE_BASE"
if [ "$EXISTING" = "--existing" ]; then
    git worktree add "$WORKTREE_PATH" "$BRANCH"
else
    git worktree add "$WORKTREE_PATH" -b "$BRANCH"
fi

# Setup the worktree
cd "$WORKTREE_PATH"

echo "Generating Xcode project..."
xcodegen generate 2>/dev/null || echo "  (xcodegen not found or failed - run manually if needed)"

echo ""
echo "Worktree ready at: $WORKTREE_PATH"
echo ""
echo "Next steps:"
echo "  cd $WORKTREE_PATH"
echo "  ./build.sh run"
echo ""
echo "When done:"
echo "  git worktree remove $WORKTREE_PATH"
echo "  git branch -d $BRANCH"

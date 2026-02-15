# Fix Review Findings

You are fixing code review findings for a story in the **Ora** macOS voice assistant project.

## Your Role

- Fix ONLY the issues listed in the "Review Findings to Fix" section below
- Do not refactor, improve, or change anything outside the scope of these findings
- Build and test after fixing
- Commit fixes with descriptive messages

## Rules

1. Read the relevant code before making changes
2. Follow the project's coding style (4-space indent, explicit self, MARK organization)
3. Fix each issue precisely — do not introduce new patterns or abstractions
4. Run `./build.sh test` after all fixes — both build and tests MUST pass
5. Commit each logical fix separately:
   ```bash
   git add <specific-files>
   git commit -m "fix(<scope>): address review finding - <brief description>"
   ```

## Priority Handling

- **P0 (Critical):** Must be fixed. These are blocking issues.
- **P1 (Major):** Must be fixed. These affect correctness or completeness.
- **P2 (Minor):** Fix if straightforward. Skip if the fix would require significant refactoring.

## Constraints

**DO NOT:**
- Push to remote or create a PR
- Modify the story file
- Refactor unrelated code
- Add features beyond what the findings require
- Use `git stash`

**DO:**
- Read the code around each finding before fixing
- Preserve existing patterns
- Build and test after fixing
- Commit with clear messages referencing the finding

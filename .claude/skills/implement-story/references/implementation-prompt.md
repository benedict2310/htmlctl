# Implementation Agent Instructions

You are implementing a story for the **Ora** macOS voice assistant. Ora is a privacy-first, fully on-device voice assistant built with Swift 6.0 targeting macOS 26 (Tahoe) on Apple Silicon.

## Your Role

- Implement ALL acceptance criteria listed in the story document
- Write clean, well-tested Swift code following existing patterns
- Build and test your implementation
- Commit changes frequently with descriptive messages

## Project Overview

**Tech Stack:**
- Swift 6.0 with strict concurrency (AppKit + SwiftUI)
- Core frameworks: AVFoundation, EventKit, Contacts, Metal, Accelerate, SwiftData
- ASR: FluidAudio Parakeet | LLM: MLX Swift (Qwen 2.5) | TTS: Kokoro MLX
- Project management: XcodeGen (project.yml → .xcodeproj)

**Key directories:**
- `Ora/Audio/` - Capture, VAD, Buffers
- `Ora/ASR/` - Parakeet engine, transcription
- `Ora/LLM/` - Local LLM, tool calling, context
- `Ora/Tools/` - Calendar, Reminders, Contacts, System
- `Ora/TTS/` - Text-to-speech engine
- `Ora/UI/` - Overlay, Menu, Dialogs
- `Ora/Orchestration/` - AssistantController, Permissions, AuditLog
- `Ora/Utilities/` - Helpers
- `OraTests/` - Unit tests

## Coding Style (MANDATORY)

1. **4-space indentation** — no tabs
2. **Explicit `self`** — intentional style, do not remove
3. **MARK organization** — group related code with `// MARK: - Section Name`
4. **Small, typed structs/enums** — prefer value types
5. **Swift Concurrency** — use async/await, actors, AsyncSequence over GCD
6. **Descriptive symbols** — clear naming, match existing codebase tone
7. **No over-engineering** — only implement what the acceptance criteria require
8. **No unnecessary comments** — code should be self-documenting; only add comments where logic is non-obvious
9. **No `.public` privacy modifiers in Logger** — these are for temporary debugging only

## Build & Test Commands

```bash
# Build (MUST pass before you're done)
./build.sh

# Run tests (MUST pass before you're done)
./build.sh test
```

If either fails, fix the issues before finishing.

## Git Commit Conventions

Commit frequently — each logical unit of work gets its own commit:

```bash
git add <specific-files>
git commit -m "<type>(<scope>): <description>"
```

**Types:** `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

**Examples:**
```bash
git commit -m "feat(overlay): add transcript view with streaming updates"
git commit -m "test(tools): add CalendarTool unit tests"
```

**Rules:**
- Imperative mood: "Add", "Fix", "Update" (not "Added", "Fixed")
- Keep first line ≤ 72 characters
- Stage specific files, not `git add -A`

## Testing Requirements

- Add XCTest cases under `OraTests/` for all new logic
- File naming: `<Component>Tests.swift`
- Method naming: `test_<feature>_<scenario>_<expectedBehavior>()`
- Use Given/When/Then structure
- Cover: happy path, error cases, edge cases
- For async code, use `async throws` test methods
- Target ≥85% coverage for new code

## Important Constraints

**DO NOT:**
- Create a PR or push to remote (the orchestrator handles that)
- Modify the story file (the orchestrator handles that)
- Refactor code outside the story's scope
- Add features not in the acceptance criteria
- Add docstrings or type annotations to code you didn't change
- Use `git stash` — always commit to the branch
- Use `.public` privacy modifiers in Logger calls
- Introduce security vulnerabilities (command injection, XSS, etc.)

**DO:**
- Read existing code before modifying it
- Follow existing patterns in similar files
- Use existing helpers and utilities
- Keep changes minimal and focused
- Handle errors appropriately at system boundaries
- Build and test before finishing

## Tool Implementation Rules (if implementing tools)

- State-changing tools MUST implement the confirmation pattern (requiresConfirmation)
- Query/read tools do NOT need confirmation
- All tool executions must be logged via AuditLog
- Fetch only needed data (scope minimization)
- Text matching against user input MUST include fuzzy matching fallback (StringSimilarity)

## Permission Implementation Rules (if touching permissions)

- Permission requests MUST go through `PermissionsManager.shared.request()`
- Do NOT add PermissionPromptTracker calls to individual permission files
- Map `.authorized`/`.writeOnly` correctly
- Prompt accessibility before opening Settings

## Memory Rules

- Set `GPU.set(cacheLimit:)` on model load (512MB recommended)
- Call `GPU.clearCache()` after each LLM/TTS generation

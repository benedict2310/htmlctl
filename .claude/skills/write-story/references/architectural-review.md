# Architectural Review Checklist

Use this to validate the story against Ora's architecture.

## General Alignment

- [ ] Reuses existing components/helpers before adding new ones.
- [ ] Preserves pipeline boundaries (ASR → LLM → Tools → TTS).
- [ ] Swift Concurrency preferred over GCD; UI on `@MainActor`.
- [ ] Minimal Objective-C++ bridging; documented if required.
- [ ] Audit logging included for any tool execution.

## By Story Type

### UI / Foundations (F.*)

- [ ] Matches UI patterns in `Ora/UI` and existing controllers.
- [ ] Overlay/menu items avoid duplicate status items on shutdown.
- [ ] Accessibility and permission flows align with `Permissions` patterns.

### ASR / Audio (A.* / S.*)

- [ ] Meets performance targets (200-400ms partials, <=300ms finalization).
- [ ] Keeps audio thread real-time safe (no locks, no blocking I/O).
- [ ] Uses existing audio buffer/VAD patterns.

### LLM (L.*)

- [ ] Structured output and validation handled (`JSONValidator`).
- [ ] Prompting uses `SystemPromptBuilder` with date/time/timezone.
- [ ] Retries and error handling explicit (max attempts noted).

### Tools (X.*)

- [ ] Confirmation guardrails required for state-changing actions.
- [ ] Uses minimal data fetch (scope-limited queries).
- [ ] AuditLog entries defined (tool name, parameters, result, userConfirmed).

### TTS (T.*)

- [ ] Early TTS start via sentence chunking when applicable.
- [ ] Fallback path documented (AVSpeechSynthesizer).

### Orchestration (O.*)

- [ ] AssistantController state machine remains authoritative.
- [ ] Tool execution flow matches propose → confirm → execute.
- [ ] Notifications/state updates documented for UI.

### Reliability / Permissions (E.* / F.02)

- [ ] Error recovery paths defined (fallbacks, user messaging).
- [ ] Permission checks map to correct APIs and states.
- [ ] Posts notifications and updates shared state on changes.

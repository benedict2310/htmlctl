# E6-S7 - API Error Response Sanitization

**Epic:** Epic 6 — Security Hardening
**Status:** Pending
**Priority:** P1 (Medium-High — internal path and schema leakage)
**Estimated Effort:** 0.5 days
**Dependencies:** E2-S1 (server bootstrap), E2-S3 (bundle ingestion)
**Target:** htmlservd (all HTTP handlers)
**Design Reference:** Security Audit 2026-02-20, Vuln 12

---

## 1. Objective

All htmlservd API handlers currently pass raw Go `error` strings into the `details` field of JSON error responses. These strings can contain absolute filesystem paths (e.g., `/var/lib/htmlservd/websites/acme/envs/prod/releases/01JNX...`), SQLite error text with internal schema details, and ULID-format release identifiers. An attacker who probes the API learns the exact on-disk layout, enabling more precise path traversal, injection, and timing attacks.

This story sanitizes all API error responses so that internal details are logged server-side but only a safe, generic message is returned to the caller.

## 2. User Story

As a security-conscious operator, I want API error responses to contain only safe, generic messages so that an unauthenticated or unauthorized caller cannot map the server's internal filesystem layout or database schema by probing error responses.

## 3. Scope

### In Scope

- Audit every call to `writeAPIError` across all handler files and classify each as either:
  - **Client error (4xx):** The error message may be returned verbatim because it describes a caller mistake (e.g., "name must match pattern `^[a-zA-Z0-9_-]+$`"). These are already safe.
  - **Server error (5xx):** The error message must be sanitized. Log the full `err.Error()` at Error level internally; return only a generic string to the caller (e.g., `"internal server error"` or a stable code like `"apply_failed"`).
- Replace all 5xx `writeAPIError` `details` slices that currently contain `err.Error()` with static strings or stable error codes.
- Add structured server-side logging (if not already present) at the error sites that previously surfaced details to the caller, so operators retain observability.
- Update any tests that assert on the specific error detail string in 5xx responses.

### Out of Scope

- Changing the `writeAPIError` function signature or the JSON response schema (only the content of `details` changes for 5xx responses).
- Structured error codes / machine-readable error taxonomy (a future API design story).
- Changes to 4xx client-error responses (safe to return as-is).
- Log format changes beyond adding the internal error at existing log sites.

## 4. Architecture Alignment

- **Logging vs. response:** The pattern is: `log.Error("apply failed", "error", err); writeAPIError(w, 500, "apply failed", nil)`. The structured log captures the full error for operators; the HTTP response reveals nothing beyond the operation name.
- **Client vs. server errors:** Input validation errors (4xx) that result from caller mistakes are already correct to return verbatim because they describe the caller's input, not the server's internals. Only 5xx responses need sanitization.
- **Error codes:** Returning a stable string like `"apply_failed"` instead of `""` in the `message` field gives clients a machine-readable signal without exposing internals. This is consistent with the existing message field usage.
- **PRD references:** Technical Spec Section 4 (API error contract).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- None.

### 5.2 Files to Modify

The following handler files contain 5xx `writeAPIError` calls that expose `err.Error()`:

- `internal/server/apply.go` — replace `[]string{err.Error()}` in 5xx paths with `nil` (or a stable code); ensure the error is logged before the response is written.
- `internal/server/domains.go` — same pattern for domain list, create, delete, and verify handlers.
- `internal/server/release.go` — same for release list and history handlers.
- `internal/server/promote.go` — same for promote handler.
- `internal/server/rollback.go` — same for rollback handler.
- `internal/server/logs.go` — same for audit log query handler.
- `internal/server/resources.go` — same for resource query helpers.

For each site, the change is:
```go
// Before
writeAPIError(w, http.StatusInternalServerError, "apply failed", []string{err.Error()})

// After
slog.ErrorContext(r.Context(), "apply failed", "error", err)
writeAPIError(w, http.StatusInternalServerError, "apply failed", nil)
```

### 5.3 Tests to Add

- Update any existing test that asserts `response.details[0] == err.Error()` for a 5xx response to instead assert `response.details == nil` (or whatever the new empty value is).
- Add a test: simulate a DB error in the apply handler; confirm the HTTP response body does not contain the error string or any filesystem path.

### 5.4 Dependencies / Config

- No new dependencies.
- No config changes.

## 6. Acceptance Criteria

- [ ] AC-1: No 5xx API response body contains a Go `error.Error()` string, filesystem path, SQLite error text, or release ID.
- [ ] AC-2: Every 5xx error site logs the full internal error at Error level with structured context (operation name, website, environment where applicable) before writing the sanitized response.
- [ ] AC-3: 4xx client-error responses are unaffected — they continue to return human-readable validation messages.
- [ ] AC-4: All existing handler tests pass; tests that asserted on 5xx error detail strings are updated to reflect the new sanitized responses.
- [ ] AC-5: An operator can correlate a client-observed error with the server log using the request's trace context or timestamp.

## 7. Verification Plan

### Automated Tests

- [ ] Handler tests: simulate DB/filesystem errors; confirm response body `details` field is absent or empty.
- [ ] Handler tests: validation errors (4xx) still return descriptive messages.

### Manual Tests

- [ ] Cause a deliberate DB error (e.g., delete the SQLite file while the server is running); make an API request; inspect the JSON response to confirm no path or schema detail appears.
- [ ] Check server logs to confirm the full error is still captured.

## 8. Performance / Reliability Considerations

- Minimal — this is a string-replacement change at error-response sites. No performance impact.

## 9. Risks & Mitigations

- **Risk:** Removing error details from responses makes debugging harder for operators who relied on API response bodies for quick diagnosis. **Mitigation:** The structured log at Error level preserves full observability. Operators should use `htmlctl logs` or server log tailing instead of parsing HTTP error bodies.
- **Risk:** Tests that currently mock specific error strings and assert on response details break. **Mitigation:** Identified in §5.3; these tests are straightforward to update.

## 10. Open Questions

- Should 5xx responses include a stable error code (e.g., `"code": "apply_failed"`) to allow clients to distinguish error types without exposing internals? Yes — this is low risk and adds value. Include a stable `message` string (already present) and keep `details` empty for 5xx.

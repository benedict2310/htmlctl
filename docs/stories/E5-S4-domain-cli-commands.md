# E5-S4 - htmlctl domain add/verify CLI Commands

**Epic:** Epic 5 — Domains + TLS via Caddy
**Status:** Not Started
**Priority:** P1 (Critical Path)
**Estimated Effort:** 3 days
**Dependencies:** E5-S1 (DomainBinding resource), E5-S2 (Caddy config generation), E5-S3 (Caddy reload), E3-S3 (remote command framework)
**Target:** macOS/Linux CLI (htmlctl)
**Design Reference:** PRD Section 9.2 (CLI design — Domains), Technical Spec Section 9.2

---

## 1. Objective

Provide the operator-facing CLI commands for managing custom domains on htmlservd. This is the user interface layer for the entire Domains + TLS epic: operators use these commands to bind domains, verify DNS and TLS, list bindings, and remove domains. Each command communicates with the htmlservd API over the remote transport established in E3-S3.

## 2. User Story

As an operator, I want to manage custom domains for my website environments from the CLI so that I can bind domains, verify they resolve correctly with valid TLS, and remove bindings — all through the familiar kubectl-style workflow.

## 3. Scope

### In Scope

- `htmlctl domain add <domain> --context <ctx>` — create a DomainBinding on the server, which triggers Caddy config regeneration and reload
- `htmlctl domain verify <domain> --context <ctx>` — check that DNS resolves to the server and that Caddy serves a valid TLS certificate for the domain
- `htmlctl domain list --context <ctx>` — list all domain bindings for the context's website, showing domain, environment, and status
- `htmlctl domain remove <domain> --context <ctx>` — delete a domain binding, triggering Caddy config regeneration and reload
- DNS verification using Go `net.LookupHost` to check that the domain resolves to the expected server IP
- TLS verification by making an HTTPS request to the domain and checking the certificate validity
- Clear, structured CLI output for all commands (table format for list, status messages for add/remove/verify)
- Error messages for common failures (DNS not configured, TLS cert not yet issued, domain already bound, domain not found)

### Out of Scope

- DNS record creation or modification (operator must configure DNS manually)
- Certificate provisioning or management (Caddy handles ACME automatically)
- Wildcard domain support
- Domain transfer between environments (remove + add workflow is sufficient for v1)
- Web UI for domain management
- Domain health monitoring or alerting

## 4. Architecture Alignment

- **Component boundaries:** CLI commands live in `cmd/htmlctl/` following the same pattern as other remote commands (E3-S3). They call the htmlservd REST API over the SSH tunnel transport. No direct database or Caddy interaction from the CLI.
- **Remote transport:** Uses the context-based remote command framework from E3-S3. The `--context` flag determines which server and environment to target.
- **API contract:** CLI commands map directly to the server API endpoints defined in E5-S1:
  - `domain add` -> `POST /api/v1/domains`
  - `domain list` -> `GET /api/v1/domains`
  - `domain remove` -> `DELETE /api/v1/domains/{domain}`
  - `domain verify` is client-side (DNS lookup + TLS check) with an optional server-side status query
- **CLI patterns:** Follows the kubectl-style subcommand pattern established in E1-S4 and E3-S3. Uses the same output formatting conventions.
- **PRD references:** PRD Section 9.2 ("htmlctl domain add", "htmlctl domain verify"), Technical Spec Section 9.2.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `cmd/htmlctl/domain.go` — `domain` parent command registration and shared helpers
- `cmd/htmlctl/domain_add.go` — `domain add` subcommand implementation
- `cmd/htmlctl/domain_verify.go` — `domain verify` subcommand with DNS and TLS checks
- `cmd/htmlctl/domain_list.go` — `domain list` subcommand with table output
- `cmd/htmlctl/domain_remove.go` — `domain remove` subcommand implementation
- `internal/client/domain_client.go` — HTTP client methods for domain binding API endpoints
- `cmd/htmlctl/domain_test.go` — Unit tests for CLI argument parsing and output formatting
- `internal/client/domain_client_test.go` — Client tests with mock HTTP server
- `cmd/htmlctl/domain_verify_test.go` — Tests for DNS/TLS verification logic

### 5.2 Files to Modify

- `cmd/htmlctl/main.go` — Register the `domain` command group
- `internal/client/client.go` — Add domain-related methods to the client interface (if using a shared client)

### 5.3 Tests to Add

- `cmd/htmlctl/domain_test.go`
  - `domain add` with valid domain sends correct POST request
  - `domain add` without domain argument prints usage error
  - `domain add` with `--context` selects correct server
  - `domain remove` with valid domain sends correct DELETE request
  - `domain list` displays table output with domain, environment, created columns
  - `domain list` with no bindings displays empty message
- `cmd/htmlctl/domain_verify_test.go`
  - DNS lookup success: domain resolves to expected IP
  - DNS lookup failure: clear error message
  - TLS verification success: valid certificate for domain
  - TLS verification failure: certificate not yet issued or invalid
  - Combined verify output: both DNS and TLS status displayed
- `internal/client/domain_client_test.go`
  - CreateDomainBinding sends correct JSON payload and parses response
  - ListDomainBindings parses array response
  - DeleteDomainBinding sends DELETE and handles 204/404
  - HTTP error responses (400, 409, 500) produce descriptive errors

### 5.4 Dependencies/Config

- No new Go dependencies for DNS lookup (`net` standard library)
- No new Go dependencies for TLS verification (`crypto/tls`, `net/http` standard library)
- CLI framework dependency matches existing setup (likely `cobra` or similar from E1-S4)

## 6. Acceptance Criteria

- [ ] AC-1: `htmlctl domain add <domain> --context <ctx>` creates a domain binding on the server and prints a success message including the domain, website, and environment. Server-side config regeneration and Caddy reload happen automatically (via E5-S1/S2/S3).
- [ ] AC-2: `htmlctl domain add` with an invalid domain name prints the validation error returned by the server (400 response).
- [ ] AC-3: `htmlctl domain add` with an already-bound domain prints a conflict error (409 response).
- [ ] AC-4: `htmlctl domain list --context <ctx>` displays all domain bindings for the context's website in a table with columns: DOMAIN, ENVIRONMENT, CREATED.
- [ ] AC-5: `htmlctl domain list` with no bindings prints a message indicating no domains are configured.
- [ ] AC-6: `htmlctl domain remove <domain> --context <ctx>` deletes the domain binding and prints a success message. Server-side config regeneration and Caddy reload happen automatically.
- [ ] AC-7: `htmlctl domain remove` with a non-existent domain prints a not-found error (404 response).
- [ ] AC-8: `htmlctl domain verify <domain> --context <ctx>` performs a DNS lookup and reports whether the domain resolves (and to which IP addresses).
- [ ] AC-9: `htmlctl domain verify` performs a TLS check by connecting to the domain over HTTPS and reports whether the certificate is valid, its issuer, and expiry date.
- [ ] AC-10: `htmlctl domain verify` clearly reports each check's status (pass/fail) with actionable guidance on failures (e.g., "DNS not configured — add an A record pointing to your server").
- [ ] AC-11: All commands require the `--context` flag (or use the current-context from config) and fail with a clear error if no context is available.
- [ ] AC-12: All unit tests pass.

## 7. Verification Plan

### Automated Tests

- [ ] Unit tests for CLI argument parsing and validation for all four subcommands
- [ ] Unit tests for domain client HTTP methods with mock server
- [ ] Unit tests for DNS lookup logic with mock resolver (or test against known domains)
- [ ] Unit tests for TLS verification logic with mock TLS server
- [ ] Unit tests for table output formatting

### Manual Tests

- [ ] Run `htmlctl domain add futurelab.studio --context prod` against a running htmlservd, verify domain binding created
- [ ] Run `htmlctl domain list --context prod`, verify table output shows the binding
- [ ] Run `htmlctl domain verify futurelab.studio --context prod`, verify DNS and TLS checks report correctly
- [ ] Run `htmlctl domain remove futurelab.studio --context prod`, verify binding removed and list is empty
- [ ] Run `htmlctl domain add` without arguments, verify usage help is printed
- [ ] Run `htmlctl domain verify` for a domain with no DNS record, verify clear failure message

## 8. Performance / Reliability Considerations

- DNS lookups and TLS checks in `domain verify` involve network I/O and may be slow or fail due to network issues. Use reasonable timeouts (5 seconds for DNS, 10 seconds for TLS handshake).
- The `domain add` and `domain remove` commands are synchronous: they wait for the server to respond, which includes Caddy config regeneration and reload. Typical response time should be under 2 seconds.
- TLS certificate issuance by Caddy (via ACME/Let's Encrypt) may take seconds to minutes after a domain is added. The `domain verify` command should inform the operator if the cert is not yet available and suggest retrying.

## 9. Risks & Mitigations

- **Risk:** DNS propagation delay causes `domain verify` to report failure immediately after `domain add`. **Mitigation:** Document that DNS must be configured before adding the domain. The verify command prints a clear message suggesting the operator wait for DNS propagation.
- **Risk:** TLS certificate not yet issued when `domain verify` is run shortly after `domain add`. **Mitigation:** The verify command distinguishes between "connection refused" (Caddy not serving the domain), "certificate error" (cert not yet issued), and "valid certificate" states with appropriate messages.
- **Risk:** Server unreachable over SSH tunnel when running domain commands. **Mitigation:** Reuse the connection error handling from E3-S3 with clear error messages about context configuration and server availability.
- **Risk:** Operator adds domain before configuring DNS, causing Caddy ACME challenge to fail. **Mitigation:** Document the recommended workflow (configure DNS first, then add domain, then verify). Consider adding a `--skip-verify` or `--dry-run` flag post-v1.

## 10. Open Questions

- Should `domain add` automatically run a DNS check before creating the binding and warn the operator if DNS is not configured? This would add latency but prevent common mistakes. Default: No for v1 — keep add fast and use verify as a separate step.
- Should `domain verify` check against a specific expected server IP, and if so, where does that IP come from? Options: (a) the server reports its own public IP via API, (b) the operator specifies it in context config, (c) verify just reports what DNS resolves to. Default: Option (c) for v1.
- Should `domain list` show a STATUS column (e.g., DNS OK, TLS OK, Pending)? This would require the list command to perform DNS/TLS checks for each domain, which could be slow. Default: No for v1 — use `domain verify` for status checks on individual domains.
- Should there be a `domain update` command to re-bind a domain to a different environment? For v1, the workflow is `domain remove` + `domain add`.

---

## Appendix: Example CLI Output

### domain add
```
$ htmlctl domain add futurelab.studio --context prod
Domain binding created:
  Domain:      futurelab.studio
  Website:     futurelab
  Environment: prod

Caddy configuration updated and reloaded.
Next: run 'htmlctl domain verify futurelab.studio --context prod' to check DNS and TLS.
```

### domain list
```
$ htmlctl domain list --context prod
DOMAIN                        ENVIRONMENT   CREATED
futurelab.studio              prod          2026-02-15T10:30:00Z
staging.futurelab.studio      staging       2026-02-15T10:31:00Z
```

### domain verify
```
$ htmlctl domain verify futurelab.studio --context prod
Verifying futurelab.studio...

DNS Resolution:    PASS  (resolves to 203.0.113.10)
TLS Certificate:   PASS  (valid, issued by Let's Encrypt, expires 2026-05-16)
```

### domain verify (failure)
```
$ htmlctl domain verify futurelab.studio --context prod
Verifying futurelab.studio...

DNS Resolution:    FAIL  (no DNS records found)
  -> Add an A record for futurelab.studio pointing to your server IP.

TLS Certificate:   SKIP  (cannot check TLS without DNS resolution)
```

### domain remove
```
$ htmlctl domain remove futurelab.studio --context prod
Domain binding removed: futurelab.studio
Caddy configuration updated and reloaded.
```

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)

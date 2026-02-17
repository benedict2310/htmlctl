package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDomainAddCommand(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodPost || req.Path != "/api/v1/domains" {
				t.Fatalf("unexpected request: %#v", req)
			}
			body := string(req.Body)
			if !strings.Contains(body, `"domain":"futurelab.studio"`) {
				t.Fatalf("missing domain in request body: %s", body)
			}
			if !strings.Contains(body, `"website":"futurelab"`) {
				t.Fatalf("missing website in request body: %s", body)
			}
			if !strings.Contains(body, `"environment":"staging"`) {
				t.Fatalf("missing environment in request body: %s", body)
			}
			return jsonHTTPResponse(201, `{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"domain", "add", "futurelab.studio"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Domain binding created") || !strings.Contains(out, "futurelab.studio") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestDomainAddJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(201, `{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`), nil
		},
	}
	out, _, err := runCommandWithTransport(t, []string{"domain", "add", "futurelab.studio", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"domain": "futurelab.studio"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestDomainListAndRemoveCommands(t *testing.T) {
	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				if req.Method != http.MethodGet || req.Path != "/api/v1/domains" || req.Query != "website=futurelab" {
					t.Fatalf("unexpected list request: %#v", req)
				}
				return jsonHTTPResponse(200, `{"domains":[{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`), nil
			case 2:
				if req.Method != http.MethodDelete || req.Path != "/api/v1/domains/futurelab.studio" {
					t.Fatalf("unexpected remove request: %#v", req)
				}
				return jsonHTTPResponse(204, ``), nil
			default:
				t.Fatalf("unexpected request count %d", call)
				return nil, nil
			}
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"domain", "list"}, tr)
	if err != nil {
		t.Fatalf("list Execute() error = %v", err)
	}
	if !strings.Contains(out, "futurelab.studio") {
		t.Fatalf("unexpected list output: %s", out)
	}

	out, _, err = runCommandWithTransport(t, []string{"domain", "remove", "futurelab.studio"}, tr)
	if err != nil {
		t.Fatalf("remove Execute() error = %v", err)
	}
	if !strings.Contains(out, "Domain binding removed: futurelab.studio") {
		t.Fatalf("unexpected remove output: %s", out)
	}
}

func TestDomainListJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("unexpected method %s", req.Method)
			}
			return jsonHTTPResponse(200, `{"domains":[{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`), nil
		},
	}
	out, _, err := runCommandWithTransport(t, []string{"domain", "list", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"domains":`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestDomainListEmptyAndRemoveJSON(t *testing.T) {
	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				if req.Method != http.MethodGet || req.Path != "/api/v1/domains" {
					t.Fatalf("unexpected list request: %#v", req)
				}
				return jsonHTTPResponse(200, `{"domains":[]}`), nil
			case 2:
				if req.Method != http.MethodDelete || req.Path != "/api/v1/domains/futurelab.studio" {
					t.Fatalf("unexpected remove request: %#v", req)
				}
				return jsonHTTPResponse(204, ``), nil
			default:
				t.Fatalf("unexpected request count %d", call)
				return nil, nil
			}
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"domain", "list"}, tr)
	if err != nil {
		t.Fatalf("list Execute() error = %v", err)
	}
	if !strings.Contains(out, "No domains configured.") {
		t.Fatalf("unexpected empty list output: %s", out)
	}

	out, _, err = runCommandWithTransport(t, []string{"domain", "remove", "futurelab.studio", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("remove Execute() error = %v", err)
	}
	if !strings.Contains(out, `"removed": true`) {
		t.Fatalf("unexpected JSON remove output: %s", out)
	}
}

func TestDomainAddInvalidDomain(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"domain", "add", "bad domain"}, tr)
	if err == nil {
		t.Fatalf("expected invalid domain error")
	}
	if !strings.Contains(err.Error(), "domain must include at least one dot") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDomainVerifyCommandSuccess(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv("HTMLCTL_CONFIG", configPath)

	prevLookup := lookupDomainHost
	prevTLS := verifyDomainTLS
	lookupDomainHost = func(ctx context.Context, host string) ([]string, error) {
		return []string{"203.0.113.10"}, nil
	}
	verifyDomainTLS = func(ctx context.Context, host string) (tlsVerifyResult, error) {
		return tlsVerifyResult{Issuer: "Let's Encrypt", ExpiresAt: "2026-05-16T00:00:00Z"}, nil
	}
	t.Cleanup(func() {
		lookupDomainHost = prevLookup
		verifyDomainTLS = prevTLS
	})

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"domain", "verify", "futurelab.studio"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "DNS Resolution:    PASS") || !strings.Contains(out.String(), "TLS Certificate:   PASS") {
		t.Fatalf("unexpected verify output: %s", out.String())
	}
}

func TestDomainVerifyCommandFailure(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv("HTMLCTL_CONFIG", configPath)

	prevLookup := lookupDomainHost
	prevTLS := verifyDomainTLS
	lookupDomainHost = func(ctx context.Context, host string) ([]string, error) {
		return nil, errors.New("no such host")
	}
	verifyDomainTLS = func(ctx context.Context, host string) (tlsVerifyResult, error) {
		t.Fatalf("verifyDomainTLS should not run when DNS fails")
		return tlsVerifyResult{}, nil
	}
	t.Cleanup(func() {
		lookupDomainHost = prevLookup
		verifyDomainTLS = prevTLS
	})

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"domain", "verify", "futurelab.studio"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected verification failure")
	}
	if !strings.Contains(err.Error(), "domain verification failed") {
		t.Fatalf("unexpected verify error: %v", err)
	}
	if !strings.Contains(out.String(), "DNS Resolution:    FAIL") || !strings.Contains(out.String(), "TLS Certificate:   SKIP") {
		t.Fatalf("unexpected verify output: %s", out.String())
	}
}

func TestDomainVerifyJSONOutputAndTLSFailure(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv("HTMLCTL_CONFIG", configPath)

	prevLookup := lookupDomainHost
	prevTLS := verifyDomainTLS
	lookupDomainHost = func(ctx context.Context, host string) ([]string, error) {
		return []string{"203.0.113.10"}, nil
	}
	verifyDomainTLS = func(ctx context.Context, host string) (tlsVerifyResult, error) {
		return tlsVerifyResult{}, errors.New("tls handshake failed")
	}
	t.Cleanup(func() {
		lookupDomainHost = prevLookup
		verifyDomainTLS = prevTLS
	})

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"domain", "verify", "futurelab.studio", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected verification failure")
	}
	if !strings.Contains(out.String(), `"pass": false`) || !strings.Contains(out.String(), `"tls handshake failed"`) {
		t.Fatalf("unexpected JSON verify output: %s", out.String())
	}
}

func TestDomainRemoveInvalidDomain(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"domain", "remove", "bad domain"}, tr)
	if err == nil {
		t.Fatalf("expected invalid domain error")
	}
}

func TestDomainVerifyInvalidDomain(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv("HTMLCTL_CONFIG", configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"domain", "verify", "invalid domain"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid domain error")
	}
}

func TestDomainAddInvalidOutputFormat(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"domain", "add", "futurelab.studio", "--output", "bogus"}, tr)
	if err == nil {
		t.Fatalf("expected invalid output format error")
	}
}

func TestDomainListInvalidOutputFormat(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"domain", "list", "--output", "bogus"}, tr)
	if err == nil {
		t.Fatalf("expected invalid output format error")
	}
}

func TestDomainVerifyInvalidOutputFormat(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv("HTMLCTL_CONFIG", configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"domain", "verify", "futurelab.studio", "--output", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid output format error")
	}
}

func TestDomainParentCommandShowsHelp(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv("HTMLCTL_CONFIG", configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help output without error, got %v", err)
	}
	if !strings.Contains(out.String(), "Manage custom domain bindings") {
		t.Fatalf("unexpected domain help output: %s", out.String())
	}
}

func TestLookupDomainHostDefaultImplementation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	addresses, err := lookupDomainHost(ctx, "localhost")
	if err != nil {
		t.Fatalf("lookupDomainHost(localhost) error = %v", err)
	}
	if len(addresses) == 0 {
		t.Fatalf("expected at least one localhost address")
	}
}

func TestVerifyDomainTLSDefaultImplementationError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := verifyDomainTLS(ctx, "127.0.0.1")
	if err == nil {
		t.Fatalf("expected TLS verification error when no TLS listener is present")
	}
}

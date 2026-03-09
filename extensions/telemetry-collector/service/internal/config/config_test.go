package config

import (
	"strings"
	"testing"
)

func TestLoadServeFromEnv_DefaultsByEnvironment(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "staging")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "https://staging.example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL", "")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "secret")
	t.Setenv("TELEMETRY_COLLECTOR_ALLOWED_EVENTS", "")
	cfg, err := LoadServeFromEnv()
	if err != nil {
		t.Fatalf("LoadServeFromEnv() error = %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9601" {
		t.Fatalf("expected staging addr 127.0.0.1:9601, got %q", cfg.HTTPAddr)
	}
	if cfg.HTMLSERVDBaseURL != "http://127.0.0.1:9400" {
		t.Fatalf("unexpected htmlservd base URL: %q", cfg.HTMLSERVDBaseURL)
	}
	if len(cfg.AllowedEvents) != 4 {
		t.Fatalf("expected default events, got %v", cfg.AllowedEvents)
	}
}

func TestLoadServeFromEnv_RejectsNonLoopbackHTTPAddr(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "prod")
	t.Setenv("TELEMETRY_COLLECTOR_HTTP_ADDR", "0.0.0.0:9602")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "secret")
	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsInvalidPublicBaseURL(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "prod")
	t.Setenv("TELEMETRY_COLLECTOR_HTTP_ADDR", "127.0.0.1:9602")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "ftp://example.com/path")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "secret")
	_, err := LoadServeFromEnv()
	if err == nil || (!strings.Contains(err.Error(), "http or https") && !strings.Contains(err.Error(), "path")) {
		t.Fatalf("expected public base validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsInvalidHTMLSERVDBaseURL(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "prod")
	t.Setenv("TELEMETRY_COLLECTOR_HTTP_ADDR", "127.0.0.1:9602")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL", "https://example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "secret")
	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "must use http scheme") {
		t.Fatalf("expected htmlservd base validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RequiresToken(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "prod")
	t.Setenv("TELEMETRY_COLLECTOR_HTTP_ADDR", "127.0.0.1:9602")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "")
	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN is required") {
		t.Fatalf("expected token validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsBadAllowedEvents(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "prod")
	t.Setenv("TELEMETRY_COLLECTOR_HTTP_ADDR", "127.0.0.1:9602")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "secret")
	t.Setenv("TELEMETRY_COLLECTOR_ALLOWED_EVENTS", "page_view,,cta_click")
	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "must not contain empty values") {
		t.Fatalf("expected allowed events validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsInvalidMaxValues(t *testing.T) {
	t.Setenv("TELEMETRY_COLLECTOR_ENV", "prod")
	t.Setenv("TELEMETRY_COLLECTOR_HTTP_ADDR", "127.0.0.1:9602")
	t.Setenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN", "secret")
	t.Setenv("TELEMETRY_COLLECTOR_MAX_BODY_BYTES", "0")
	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "TELEMETRY_COLLECTOR_MAX_BODY_BYTES must be a positive integer") {
		t.Fatalf("expected max body bytes validation error, got %v", err)
	}
}

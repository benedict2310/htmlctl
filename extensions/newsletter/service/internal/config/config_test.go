package config

import (
	"strings"
	"testing"
)

const validLinkSecret = "0123456789abcdef0123456789abcdef"

func TestLoadServeFromEnv_DefaultsHTTPAddressByEnvironment(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "staging")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://staging")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://staging.example.com")
	t.Setenv("NEWSLETTER_RESEND_API_KEY", "staging-key")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)

	cfg, err := LoadServeFromEnv()
	if err != nil {
		t.Fatalf("LoadServeFromEnv() error = %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9501" {
		t.Fatalf("expected staging addr 127.0.0.1:9501, got %q", cfg.HTTPAddr)
	}
	if cfg.PublicBaseURL != "https://staging.example.com" {
		t.Fatalf("unexpected staging base URL: %q", cfg.PublicBaseURL)
	}
	if cfg.ResendAPIKey != "staging-key" {
		t.Fatalf("unexpected api key: %q", cfg.ResendAPIKey)
	}
	if cfg.LinkSecret != validLinkSecret {
		t.Fatalf("unexpected link secret: %q", cfg.LinkSecret)
	}
}

func TestLoadServeFromEnv_RejectsNonLoopback(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "0.0.0.0:9502")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)

	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsInvalidHTTPPort(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)

	t.Setenv("NEWSLETTER_HTTP_ADDR", "localhost:http")
	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "port must be numeric and between 1 and 65535") {
		t.Fatalf("expected invalid port validation error, got %v", err)
	}

	t.Setenv("NEWSLETTER_HTTP_ADDR", "127.0.0.1:0")
	_, err = LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "port must be numeric and between 1 and 65535") {
		t.Fatalf("expected out-of-range port validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsInvalidBaseURL(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "127.0.0.1:9502")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "http://example.com/app")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)

	_, err := LoadServeFromEnv()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "https") && !strings.Contains(err.Error(), "path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadServeFromEnv_RequiresPublicBaseURLAndLinkSecret(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "127.0.0.1:9502")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "")
	t.Setenv("NEWSLETTER_LINK_SECRET", "")
	t.Setenv("NEWSLETTER_RESEND_FROM", "")

	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "NEWSLETTER_PUBLIC_BASE_URL is required") {
		t.Fatalf("expected public base URL required error, got %v", err)
	}

	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://example.com")
	_, err = LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "NEWSLETTER_LINK_SECRET is required") {
		t.Fatalf("expected link secret required error, got %v", err)
	}

	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)
	_, err = LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "NEWSLETTER_RESEND_FROM is required") {
		t.Fatalf("expected resend from required error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsQueryAndFragmentInPublicURL(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "127.0.0.1:9502")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://example.com?token=abc#frag")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)

	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "query or fragment") {
		t.Fatalf("expected query/fragment validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsMissingHostnameAndInvalidPort(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "127.0.0.1:9502")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://:443")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)

	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "hostname is required") {
		t.Fatalf("expected hostname validation error, got %v", err)
	}

	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://example.com:99999")
	_, err = LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "port must be between 1 and 65535") {
		t.Fatalf("expected port range validation error, got %v", err)
	}
}

func TestLoadServeFromEnv_RejectsWeakLinkSecretAndInvalidFromAddress(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "prod")
	t.Setenv("NEWSLETTER_DATABASE_URL", "postgres://prod")
	t.Setenv("NEWSLETTER_HTTP_ADDR", "127.0.0.1:9502")
	t.Setenv("NEWSLETTER_PUBLIC_BASE_URL", "https://example.com")
	t.Setenv("NEWSLETTER_RESEND_FROM", "Newsletter <newsletter@example.com>")
	t.Setenv("NEWSLETTER_LINK_SECRET", "too-short")

	_, err := LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "at least 32 characters") {
		t.Fatalf("expected weak secret validation error, got %v", err)
	}

	t.Setenv("NEWSLETTER_LINK_SECRET", validLinkSecret)
	t.Setenv("NEWSLETTER_RESEND_FROM", "invalid-from-address")
	_, err = LoadServeFromEnv()
	if err == nil || !strings.Contains(err.Error(), "invalid address") {
		t.Fatalf("expected invalid from validation error, got %v", err)
	}
}

func TestLoadMigrateFromEnv_RequiresEnvAndDB(t *testing.T) {
	t.Setenv("NEWSLETTER_ENV", "")
	t.Setenv("NEWSLETTER_DATABASE_URL", "")

	_, err := LoadMigrateFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

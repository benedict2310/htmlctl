package server

import (
	"context"
	"strings"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestGenerateCaddyConfigFromDomainBindings(t *testing.T) {
	srv := startTestServer(t)
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	stagingID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	prodID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "prod"})
	if err != nil {
		t.Fatalf("InsertEnvironment(prod) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "staging.example.com",
		EnvironmentID: stagingID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding(staging) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: prodID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding(prod) error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	first := strings.Index(cfg, "example.com {")
	second := strings.Index(cfg, "staging.example.com {")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected sorted domain blocks, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "/websites/sample/envs/prod/current") {
		t.Fatalf("expected prod root path in config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "/websites/sample/envs/staging/current") {
		t.Fatalf("expected staging root path in config, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigIncludesEnvironmentBackends(t *testing.T) {
	srv := startTestServer(t)
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "prod"})
	if err != nil {
		t.Fatalf("InsertEnvironment(prod) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}
	if err := q.UpsertBackend(ctx, dbpkg.BackendRow{
		EnvironmentID: envID,
		PathPrefix:    "/api/*",
		Upstream:      "https://api.example.com",
	}); err != nil {
		t.Fatalf("UpsertBackend(/api/*) error = %v", err)
	}
	if err := q.UpsertBackend(ctx, dbpkg.BackendRow{
		EnvironmentID: envID,
		PathPrefix:    "/auth/*",
		Upstream:      "https://auth.example.com",
	}); err != nil {
		t.Fatalf("UpsertBackend(/auth/*) error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	apiIndex := strings.Index(cfg, "\thandle /api/* {\n\t\treverse_proxy https://api.example.com\n\t}")
	authIndex := strings.Index(cfg, "\thandle /auth/* {\n\t\treverse_proxy https://auth.example.com\n\t}")
	fileServerIndex := strings.Index(cfg, "\thandle {\n\t\troot * ")
	if apiIndex == -1 || authIndex == -1 || fileServerIndex == -1 {
		t.Fatalf("expected backend directives and file_server, got:\n%s", cfg)
	}
	if !(apiIndex < authIndex && authIndex < fileServerIndex) {
		t.Fatalf("expected sorted backend directives before file_server, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigSharesEnvironmentBackendsAcrossDomains(t *testing.T) {
	srv := startTestServer(t)
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "prod"})
	if err != nil {
		t.Fatalf("InsertEnvironment(prod) error = %v", err)
	}
	for _, domain := range []string{"example.com", "www.example.com"} {
		if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
			Domain:        domain,
			EnvironmentID: envID,
		}); err != nil {
			t.Fatalf("InsertDomainBinding(%s) error = %v", domain, err)
		}
	}
	if err := q.UpsertBackend(ctx, dbpkg.BackendRow{
		EnvironmentID: envID,
		PathPrefix:    "/api/*",
		Upstream:      "https://api.example.com",
	}); err != nil {
		t.Fatalf("UpsertBackend() error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	if strings.Count(cfg, "\thandle /api/* {\n\t\treverse_proxy https://api.example.com\n\t}") != 2 {
		t.Fatalf("expected backend directive in both site blocks, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigIncludesTelemetryProxyWhenEnabled(t *testing.T) {
	srv := startTestServer(t)
	srv.cfg.Telemetry.Enabled = true
	srv.cfg.Port = 9400

	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	if !strings.Contains(cfg, "handle /collect/v1/events*") {
		t.Fatalf("expected telemetry handle stanza in config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "reverse_proxy 127.0.0.1:9400") {
		t.Fatalf("expected telemetry reverse proxy in config, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigIncludesPreviewSites(t *testing.T) {
	srv := startTestServer(t)
	srv.cfg.Preview.Enabled = true
	srv.cfg.Preview.BaseDomain = "preview.example.com"
	srv.cfg.Telemetry.Enabled = true
	srv.cfg.Port = 9400

	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "staging.example.com",
		EnvironmentID: envID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}
	if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            "R1",
		EnvironmentID: envID,
		ManifestJSON:  `{}`,
		OutputHashes:  `{}`,
		BuildLog:      "ok",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease() error = %v", err)
	}
	if _, err := q.InsertReleasePreview(ctx, dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R1",
		Hostname:      "abc123--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2099-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("InsertReleasePreview() error = %v", err)
	}
	if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            "R0",
		EnvironmentID: envID,
		ManifestJSON:  `{}`,
		OutputHashes:  `{}`,
		BuildLog:      "ok",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(expired) error = %v", err)
	}
	if _, err := q.InsertReleasePreview(ctx, dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R0",
		Hostname:      "expired--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2000-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("InsertReleasePreview(expired) error = %v", err)
	}
	if err := q.UpsertBackend(ctx, dbpkg.BackendRow{
		EnvironmentID: envID,
		PathPrefix:    "/api/*",
		Upstream:      "https://api.example.com",
	}); err != nil {
		t.Fatalf("UpsertBackend() error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	if !strings.Contains(cfg, "abc123--staging--sample.preview.example.com {") {
		t.Fatalf("expected preview site block, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "*.preview.example.com {") {
		t.Fatalf("expected preview wildcard fallback block, got:\n%s", cfg)
	}
	if strings.Contains(cfg, "expired--staging--sample.preview.example.com {") {
		t.Fatalf("did not expect expired preview site block, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "/websites/sample/envs/staging/releases/R1") {
		t.Fatalf("expected preview site to point at release root, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, `header X-Robots-Tag "noindex, nofollow, noarchive"`) {
		t.Fatalf("expected robots header in preview site, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "\trespond 404") {
		t.Fatalf("expected wildcard preview fallback to respond 404, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "\thandle /api/* {\n\t\treverse_proxy https://api.example.com\n\t}") {
		t.Fatalf("expected preview site to keep backend routing, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "handle /collect/v1/events*") {
		t.Fatalf("expected preview site to keep telemetry proxying, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigOmitsTelemetryProxyWhenDisabled(t *testing.T) {
	srv := startTestServer(t)
	srv.cfg.Telemetry.Enabled = false

	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	if strings.Contains(cfg, "handle /collect/v1/events*") {
		t.Fatalf("did not expect telemetry stanza when disabled, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigRequiresDB(t *testing.T) {
	s := &Server{}
	if _, err := s.generateCaddyConfig(context.Background()); err == nil {
		t.Fatalf("expected error when db is nil")
	}
}

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
		Name:               "futurelab",
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
		Domain:        "staging.futurelab.studio",
		EnvironmentID: stagingID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding(staging) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{
		Domain:        "futurelab.studio",
		EnvironmentID: prodID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding(prod) error = %v", err)
	}

	cfg, err := srv.generateCaddyConfig(ctx)
	if err != nil {
		t.Fatalf("generateCaddyConfig() error = %v", err)
	}
	first := strings.Index(cfg, "futurelab.studio {")
	second := strings.Index(cfg, "staging.futurelab.studio {")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected sorted domain blocks, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "/websites/futurelab/envs/prod/current") {
		t.Fatalf("expected prod root path in config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "/websites/futurelab/envs/staging/current") {
		t.Fatalf("expected staging root path in config, got:\n%s", cfg)
	}
}

func TestGenerateCaddyConfigRequiresDB(t *testing.T) {
	s := &Server{}
	if _, err := s.generateCaddyConfig(context.Background()); err == nil {
		t.Fatalf("expected error when db is nil")
	}
}

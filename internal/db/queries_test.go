package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func setupDB(t *testing.T) (*Queries, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := RunMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return NewQueries(db), func() { _ = db.Close() }
}

func TestQueriesInsertAndFetchCoreRows(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	website, err := q.GetWebsiteByName(ctx, "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if website.ID != websiteID {
		t.Fatalf("unexpected website id: got=%d want=%d", website.ID, websiteID)
	}

	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "index", Route: "/", Title: "Home", Description: "desc", LayoutJSON: `["header"]`, ContentHash: "hash-page"}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}
	if _, err := q.InsertComponent(ctx, ComponentRow{WebsiteID: websiteID, Name: "header", Scope: "global", ContentHash: "hash-comp"}); err != nil {
		t.Fatalf("InsertComponent() error = %v", err)
	}
	if _, err := q.InsertStyleBundle(ctx, StyleBundleRow{WebsiteID: websiteID, Name: "default", FilesJSON: `[{"filename":"tokens.css","hash":"x"}]`}); err != nil {
		t.Fatalf("InsertStyleBundle() error = %v", err)
	}
	if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: websiteID, Filename: "logo.svg", ContentType: "image/svg+xml", SizeBytes: 64, ContentHash: "hash-asset"}); err != nil {
		t.Fatalf("InsertAsset() error = %v", err)
	}
	if err := q.InsertRelease(ctx, ReleaseRow{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", EnvironmentID: envID, ManifestJSON: `{}`, OutputHashes: `{}`, BuildLog: "ok", Status: "active"}); err != nil {
		t.Fatalf("InsertRelease() error = %v", err)
	}
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{Actor: "test", EnvironmentID: &envID, Operation: "apply", ResourceSummary: "changed", MetadataJSON: `{}`}); err != nil {
		t.Fatalf("InsertAuditLog() error = %v", err)
	}
}

func TestQueriesForeignKeyConstraint(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	if _, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: 9999, Name: "staging"}); err == nil {
		t.Fatalf("expected foreign key violation for invalid website id")
	}
}

func TestQueriesUniqueConstraints(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := q.InsertWebsite(ctx, WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"}); err == nil {
		t.Fatalf("expected duplicate website name violation")
	}

	if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "index", Route: "/", LayoutJSON: "[]", ContentHash: "a"}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}
	if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "home", Route: "/", LayoutJSON: "[]", ContentHash: "b"}); err == nil {
		t.Fatalf("expected duplicate route violation")
	}
}

func TestQueriesErrorBranches(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	if _, err := q.GetWebsiteByName(ctx, "missing"); err == nil {
		t.Fatalf("expected not-found error for website")
	}

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	if _, err := q.InsertComponent(ctx, ComponentRow{WebsiteID: 9999, Name: "header", Scope: "global", ContentHash: "x"}); err == nil {
		t.Fatalf("expected component FK error")
	}
	if _, err := q.InsertStyleBundle(ctx, StyleBundleRow{WebsiteID: 9999, Name: "default", FilesJSON: "[]"}); err == nil {
		t.Fatalf("expected style bundle FK error")
	}
	if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: 9999, Filename: "x", ContentType: "text/plain", SizeBytes: 1, ContentHash: "x"}); err == nil {
		t.Fatalf("expected asset FK error")
	}
	if err := q.InsertRelease(ctx, ReleaseRow{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAW", EnvironmentID: 9999, ManifestJSON: "{}", OutputHashes: "{}", BuildLog: "", Status: "active"}); err == nil {
		t.Fatalf("expected release FK error")
	}

	badEnv := int64(9999)
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{Actor: "x", EnvironmentID: &badEnv, Operation: "apply", ResourceSummary: "x", MetadataJSON: "{}"}); err == nil {
		t.Fatalf("expected audit log FK error")
	}

	releaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	if err := q.InsertRelease(ctx, ReleaseRow{ID: releaseID, EnvironmentID: envID, ManifestJSON: "{}", OutputHashes: "{}", BuildLog: "", Status: "active"}); err != nil {
		t.Fatalf("InsertRelease valid error = %v", err)
	}
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{Actor: "x", EnvironmentID: &envID, Operation: "apply", ResourceSummary: "x", ReleaseID: &releaseID, MetadataJSON: "{}"}); err != nil {
		t.Fatalf("InsertAuditLog valid error = %v", err)
	}

	// Check GetWebsiteByName surfaces sql.ErrNoRows through wrapped error.
	_, err = q.GetWebsiteByName(ctx, "still-missing")
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected wrapped sql.ErrNoRows, got %v", err)
	}
}

func TestDeleteAssetsNotInHandlesLargeKeepSet(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	for _, name := range []string{"assets/a.svg", "assets/b.svg", "assets/c.svg"} {
		if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: websiteID, Filename: name, ContentType: "image/svg+xml", SizeBytes: 10, ContentHash: "hash-" + name}); err != nil {
			t.Fatalf("InsertAsset(%q) error = %v", name, err)
		}
	}

	keep := make([]string, 0, 1000)
	for i := 0; i < 998; i++ {
		keep = append(keep, fmt.Sprintf("missing-%d", i))
	}
	keep = append(keep, "assets/a.svg", "assets/b.svg")

	deleted, err := q.DeleteAssetsNotIn(ctx, websiteID, keep)
	if err != nil {
		t.Fatalf("DeleteAssetsNotIn() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted row, got %d", deleted)
	}

	rows, err := q.ListAssetsByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListAssetsByWebsite() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 assets remaining, got %d", len(rows))
	}
}

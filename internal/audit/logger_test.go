package audit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestSQLiteLoggerLogAndQuery(t *testing.T) {
	db := openAuditTestDB(t)
	defer db.Close()
	q := dbpkg.NewQueries(db)
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("NewSQLiteLogger() error = %v", err)
	}
	now := time.Now().UTC().Add(-time.Minute)
	if err := logger.Log(ctx, Entry{Actor: "bene", EnvironmentID: &envID, Operation: OperationApply, ResourceSummary: "updated header", Metadata: map[string]any{"mode": "partial"}, Timestamp: now}); err != nil {
		t.Fatalf("Log(apply) error = %v", err)
	}
	releaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{ID: releaseID, EnvironmentID: envID, ManifestJSON: "{}", OutputHashes: "{}", BuildLog: "", Status: "active"}); err != nil {
		t.Fatalf("InsertRelease() error = %v", err)
	}
	if err := logger.Log(ctx, Entry{Actor: "bene", EnvironmentID: &envID, Operation: OperationReleaseBuild, ResourceSummary: "built release", ReleaseID: &releaseID, Timestamp: now.Add(time.Second)}); err != nil {
		t.Fatalf("Log(release.build) error = %v", err)
	}

	res, err := logger.Query(ctx, Filter{WebsiteID: websiteID, EnvironmentID: &envID, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.Total != 2 || len(res.Entries) != 2 {
		t.Fatalf("expected 2 rows, got total=%d len=%d", res.Total, len(res.Entries))
	}
	if res.Entries[0].Operation != OperationReleaseBuild {
		t.Fatalf("expected newest operation first, got %q", res.Entries[0].Operation)
	}

	filtered, err := logger.Query(ctx, Filter{WebsiteID: websiteID, EnvironmentID: &envID, Operation: OperationApply, Limit: 10})
	if err != nil {
		t.Fatalf("Query(filtered) error = %v", err)
	}
	if filtered.Total != 1 || len(filtered.Entries) != 1 {
		t.Fatalf("expected one filtered row, got total=%d len=%d", filtered.Total, len(filtered.Entries))
	}
	if filtered.Entries[0].Operation != OperationApply {
		t.Fatalf("unexpected filtered operation %q", filtered.Entries[0].Operation)
	}
}

func openAuditTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return db
}

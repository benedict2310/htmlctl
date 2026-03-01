package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunMigrationsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("first RunMigrations() error = %v", err)
	}
	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("second RunMigrations() error = %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations count: %v", err)
	}
	if count != 6 {
		t.Fatalf("expected 6 applied migrations, got %d", count)
	}
}

func TestRunMigrationsNilDB(t *testing.T) {
	if err := RunMigrations(context.Background(), nil); err == nil {
		t.Fatalf("expected nil db error")
	}
}

func TestRunMigrationsClosedDBError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := RunMigrations(context.Background(), db); err == nil {
		t.Fatalf("expected migration failure for closed db")
	}
}

func TestAppliedVersionsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := appliedVersions(context.Background(), db); err == nil {
		t.Fatalf("expected error when schema_migrations table does not exist")
	}
}

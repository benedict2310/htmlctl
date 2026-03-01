package migrations_test

import (
	"context"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestEnvironmentBackendsMigrationAddsTableAndIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(environment_backends)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(environment_backends) error = %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultV *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultV, &pk); err != nil {
			t.Fatalf("scan pragma row: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma rows: %v", err)
	}

	for _, name := range []string{"id", "environment_id", "path_prefix", "upstream", "created_at", "updated_at"} {
		if !columns[name] {
			t.Fatalf("expected environment_backends.%s column", name)
		}
	}

	var indexName string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_environment_backends_environment_id").Scan(&indexName)
	if err != nil {
		t.Fatalf("lookup environment_backends index: %v", err)
	}
	if indexName != "idx_environment_backends_environment_id" {
		t.Fatalf("unexpected index name %q", indexName)
	}
}

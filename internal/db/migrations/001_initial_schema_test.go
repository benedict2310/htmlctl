package migrations_test

import (
	"context"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestInitialSchemaCreatesAllCoreTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	expectedTables := []string{
		"schema_migrations",
		"websites",
		"environments",
		"pages",
		"components",
		"style_bundles",
		"assets",
		"releases",
		"audit_log",
		"domain_bindings",
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %q to exist: %v", table, err)
		}
	}
}

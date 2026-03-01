package migrations_test

import (
	"context"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestWebsiteSEOMigrationAddsSEOJSONColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(websites)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(websites) error = %v", err)
	}
	defer rows.Close()

	seen := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultV *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultV, &pk); err != nil {
			t.Fatalf("scan pragma row: %v", err)
		}
		if name == "seo_json" {
			seen = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate websites pragma rows: %v", err)
	}
	if !seen {
		t.Fatalf("expected websites.seo_json column")
	}
}

package migrations_test

import (
	"context"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestPagesHeadMetadataMigrationAddsHeadJSONColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(pages)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(pages): %v", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultV   *string
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultV, &pk); err != nil {
			t.Fatalf("scan pragma row: %v", err)
		}
		if name != "head_json" {
			continue
		}
		found = true
		if notNull != 1 {
			t.Fatalf("expected head_json to be NOT NULL")
		}
		if defaultV == nil || *defaultV != "'{}'" {
			t.Fatalf("unexpected head_json default: %#v", defaultV)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma rows: %v", err)
	}
	if !found {
		t.Fatalf("expected pages.head_json column to exist")
	}
}

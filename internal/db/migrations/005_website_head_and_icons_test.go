package migrations_test

import (
	"context"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestWebsiteHeadAndIconsMigrationAddsColumnsAndTable(t *testing.T) {
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

	seenHead := false
	seenHash := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultV *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultV, &pk); err != nil {
			t.Fatalf("scan pragma row: %v", err)
		}
		if name == "head_json" {
			seenHead = true
		}
		if name == "content_hash" {
			seenHash = true
		}
	}
	if !seenHead || !seenHash {
		t.Fatalf("expected websites.head_json and websites.content_hash columns")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate websites pragma rows: %v", err)
	}

	iconRows, err := db.Query(`PRAGMA table_info(website_icons)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(website_icons) error = %v", err)
	}
	defer iconRows.Close()
	if !iconRows.Next() {
		t.Fatalf("expected website_icons table to exist")
	}
}

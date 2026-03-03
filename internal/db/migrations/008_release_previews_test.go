package migrations_test

import (
	"context"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestReleasePreviewsMigrationAddsTableAndIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(release_previews)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(release_previews) error = %v", err)
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

	for _, name := range []string{"id", "environment_id", "release_id", "hostname", "created_by", "expires_at", "created_at"} {
		if !columns[name] {
			t.Fatalf("expected release_previews.%s column", name)
		}
	}

	for _, indexName := range []string{"idx_release_previews_env", "idx_release_previews_expires_at"} {
		var got string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&got); err != nil {
			t.Fatalf("lookup release_previews index %s: %v", indexName, err)
		}
		if got != indexName {
			t.Fatalf("unexpected index name %q", got)
		}
	}
}

package migrations_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestTelemetryEventsMigrationSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(telemetry_events)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(telemetry_events): %v", err)
	}
	defer rows.Close()

	var foundReceivedAt bool
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
		if name != "received_at" {
			continue
		}
		foundReceivedAt = true
		if notNull != 1 {
			t.Fatalf("expected telemetry_events.received_at to be NOT NULL")
		}
		if defaultV == nil {
			t.Fatalf("expected telemetry_events.received_at default expression")
		}
		if !strings.Contains(*defaultV, "strftime(") || !strings.Contains(*defaultV, "'now'") {
			t.Fatalf("unexpected telemetry_events.received_at default: %q", *defaultV)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma rows: %v", err)
	}
	if !foundReceivedAt {
		t.Fatalf("expected telemetry_events.received_at column to exist")
	}

	fkRows, err := db.Query(`PRAGMA foreign_key_list(telemetry_events)`)
	if err != nil {
		t.Fatalf("PRAGMA foreign_key_list(telemetry_events): %v", err)
	}
	defer fkRows.Close()

	var foundEnvFK bool
	for fkRows.Next() {
		var (
			id       int
			seq      int
			table    string
			from     string
			to       string
			onUpdate string
			onDelete string
			match    string
		)
		if err := fkRows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan foreign key row: %v", err)
		}
		if table == "environments" && from == "environment_id" && to == "id" && strings.EqualFold(onDelete, "CASCADE") {
			foundEnvFK = true
			break
		}
	}
	if err := fkRows.Err(); err != nil {
		t.Fatalf("iterate foreign key rows: %v", err)
	}
	if !foundEnvFK {
		t.Fatalf("expected telemetry_events.environment_id foreign key to environments(id) with ON DELETE CASCADE")
	}
}

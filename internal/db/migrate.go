package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/benedict2310/htmlctl/internal/db/migrations"
)

func RunMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	all := migrations.All()
	sort.Slice(all, func(i, j int) bool { return all[i].Version < all[j].Version })

	for _, m := range all {
		if applied[m.Version] {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.Version, err)
		}
		if _, err := tx.ExecContext(ctx, m.UpSQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name) VALUES(?, ?)`, m.Version, m.Name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d (%s): %w", m.Version, m.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan applied migration version: %w", err)
		}
		out[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return out, nil
}

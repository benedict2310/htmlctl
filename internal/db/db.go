package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Options struct {
	Path          string
	EnableWAL     bool
	BusyTimeoutMS int
	MaxOpenConns  int
	MaxIdleConns  int
}

func DefaultOptions(path string) Options {
	return Options{
		Path:          path,
		EnableWAL:     true,
		BusyTimeoutMS: 5000,
		MaxOpenConns:  5,
		MaxIdleConns:  5,
	}
}

func Open(opts Options) (*sql.DB, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if opts.BusyTimeoutMS <= 0 {
		opts.BusyTimeoutMS = 5000
	}
	if opts.MaxOpenConns <= 0 {
		opts.MaxOpenConns = 5
	}
	if opts.MaxIdleConns < 0 {
		opts.MaxIdleConns = 0
	}

	cleanPath := filepath.Clean(opts.Path)
	dsnParts := []string{
		fmt.Sprintf("_pragma=foreign_keys(%d)", 1),
		fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeoutMS),
	}
	if opts.EnableWAL {
		dsnParts = append(dsnParts, "_pragma=journal_mode(WAL)")
	}
	dsn := fmt.Sprintf("file:%s?%s", cleanPath, strings.Join(dsnParts, "&"))

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", cleanPath, err)
	}

	db.SetMaxOpenConns(opts.MaxOpenConns)
	db.SetMaxIdleConns(opts.MaxIdleConns)
	db.SetConnMaxIdleTime(30 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", cleanPath, err)
	}

	return db, nil
}

func JournalMode(ctx context.Context, db *sql.DB) (string, error) {
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&mode); err != nil {
		return "", fmt.Errorf("query journal_mode pragma: %w", err)
	}
	return strings.ToLower(mode), nil
}

func ForeignKeysEnabled(ctx context.Context, db *sql.DB) (bool, error) {
	var enabled int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys;").Scan(&enabled); err != nil {
		return false, fmt.Errorf("query foreign_keys pragma: %w", err)
	}
	return enabled == 1, nil
}

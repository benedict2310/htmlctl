package db

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenSetsWALAndForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	mode, err := JournalMode(ctx, db)
	if err != nil {
		t.Fatalf("JournalMode() error = %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("expected WAL mode, got %q", mode)
	}

	fk, err := ForeignKeysEnabled(ctx, db)
	if err != nil {
		t.Fatalf("ForeignKeysEnabled() error = %v", err)
	}
	if !fk {
		t.Fatalf("expected foreign_keys pragma enabled")
	}
}

func TestOpenEmptyPathError(t *testing.T) {
	_, err := Open(DefaultOptions(""))
	if err == nil {
		t.Fatalf("expected empty path error")
	}
}

func TestOpenWithoutWALAndDefaultFallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	opts := Options{
		Path:          path,
		EnableWAL:     false,
		BusyTimeoutMS: 0,
		MaxOpenConns:  0,
		MaxIdleConns:  -1,
	}
	db, err := Open(opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	mode, err := JournalMode(context.Background(), db)
	if err != nil {
		t.Fatalf("JournalMode() error = %v", err)
	}
	if strings.EqualFold(mode, "wal") {
		t.Fatalf("expected non-WAL journal mode when WAL disabled")
	}
}

func TestConcurrentReadDuringWriteWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	queries := NewQueries(db)
	websiteID, err := queries.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := queries.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "index", Route: "/", Title: "x", Description: "x", LayoutJSON: "[]", ContentHash: "abc"}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE pages SET title = ? WHERE website_id = ?`, "updated", websiteID); err != nil {
		t.Fatalf("UPDATE in tx error = %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	readErr := make(chan error, 1)
	go func() {
		defer wg.Done()
		ctxRead, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var count int
		err := db.QueryRowContext(ctxRead, `SELECT COUNT(*) FROM pages WHERE website_id = ?`, websiteID).Scan(&count)
		readErr <- err
	}()
	wg.Wait()
	if err := <-readErr; err != nil {
		t.Fatalf("concurrent read failed during write tx: %v", err)
	}
}

func TestPragmaHelpersErrorOnClosedDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := JournalMode(context.Background(), db); err == nil {
		t.Fatalf("expected JournalMode error on closed db")
	}
	if _, err := ForeignKeysEnabled(context.Background(), db); err == nil {
		t.Fatalf("expected ForeignKeysEnabled error on closed db")
	}
}

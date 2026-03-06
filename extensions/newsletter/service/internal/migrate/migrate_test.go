package migrate

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApply_AppliesPendingMigrationAndSkipsAlreadyApplied(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(regexp.QuoteMeta(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`)).
		WithArgs("001_foundation.sql").
		WillReturnResult(sqlmock.NewResult(1, 1))

	migrationBytes, err := migrationFS.ReadFile("sql/001_foundation.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, stmt := range splitStatements(string(migrationBytes)) {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()

	applied, err := Apply(context.Background(), db)
	if err != nil {
		t.Fatalf("Apply() first run error = %v", err)
	}
	if len(applied) != 1 || applied[0] != "001_foundation.sql" {
		t.Fatalf("unexpected applied versions from first run: %v", applied)
	}

	mock.ExpectExec(regexp.QuoteMeta(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`)).
		WithArgs("001_foundation.sql").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	applied, err = Apply(context.Background(), db)
	if err != nil {
		t.Fatalf("Apply() second run error = %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("expected no applied versions on second run, got %v", applied)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestFoundationSQLDeclaresRequiredTablesAndTokenHash(t *testing.T) {
	bytes, err := migrationFS.ReadFile("sql/001_foundation.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sqlText := string(bytes)

	required := []string{
		"CREATE TABLE IF NOT EXISTS subscribers",
		"CREATE TABLE IF NOT EXISTS verification_tokens",
		"CREATE TABLE IF NOT EXISTS campaigns",
		"CREATE TABLE IF NOT EXISTS campaign_sends",
		"token_hash BYTEA NOT NULL",
	}
	for _, clause := range required {
		if !strings.Contains(sqlText, clause) {
			t.Fatalf("migration SQL missing required clause %q", clause)
		}
	}

	if strings.Contains(strings.ToLower(sqlText), "raw_token") {
		t.Fatal("migration SQL must not contain raw token storage columns")
	}
}

func TestSplitStatements(t *testing.T) {
	statements := splitStatements("  SELECT 1; \n\n ; SELECT 2;;")
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d (%v)", len(statements), statements)
	}
	if statements[0] != "SELECT 1" || statements[1] != "SELECT 2" {
		t.Fatalf("unexpected split result: %v", statements)
	}
}

func TestSplitStatements_IgnoresSemicolonsInStringsCommentsAndDollarQuotes(t *testing.T) {
	sqlText := `
CREATE TABLE demo (value TEXT);
INSERT INTO demo(value) VALUES ('alpha;beta');
-- this is a comment with ; semicolon
/* block comment ; semicolon */
CREATE FUNCTION demo_fn() RETURNS void AS $fn$
BEGIN
  RAISE NOTICE 'x;y';
END;
$fn$ LANGUAGE plpgsql;
`

	statements := splitStatements(sqlText)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %d (%v)", len(statements), statements)
	}
	if !strings.Contains(statements[1], "'alpha;beta'") {
		t.Fatalf("expected semicolon in string literal to remain intact: %q", statements[1])
	}
	if !strings.Contains(statements[2], "RAISE NOTICE 'x;y';") {
		t.Fatalf("expected semicolon in dollar-quoted function body to remain intact: %q", statements[2])
	}
}

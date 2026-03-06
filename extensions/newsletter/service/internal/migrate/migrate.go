package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode"
)

//go:embed sql/*.sql
var migrationFS embed.FS

func Apply(ctx context.Context, db *sql.DB) ([]string, error) {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.Glob(migrationFS, "sql/*.sql")
	if err != nil {
		return nil, fmt.Errorf("discover migrations: %w", err)
	}
	sort.Strings(entries)

	applied := make([]string, 0)
	for _, file := range entries {
		version := path.Base(file)

		sqlBytes, err := migrationFS.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", version, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("begin migration %s: %w", version, err)
		}

		insertResult, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`,
			version,
		)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("claim migration %s: %w", version, err)
		}
		rowsAffected, err := insertResult.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("check migration claim %s: %w", version, err)
		}
		if rowsAffected == 0 {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				return nil, fmt.Errorf("rollback skipped migration %s: %w", version, err)
			}
			continue
		}

		for _, stmt := range splitStatements(string(sqlBytes)) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("apply migration %s: %w", version, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit migration %s: %w", version, err)
		}

		applied = append(applied, version)
	}

	return applied, nil
}

func splitStatements(sqlText string) []string {
	statements := make([]string, 0, 8)
	var current strings.Builder

	inSingleQuoted := false
	inDoubleQuoted := false
	inLineComment := false
	inBlockComment := false
	dollarTag := ""

	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		var next byte
		if i+1 < len(sqlText) {
			next = sqlText[i+1]
		}

		if inLineComment {
			current.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment {
			current.WriteByte(ch)
			if ch == '*' && next == '/' {
				current.WriteByte(next)
				i++
				inBlockComment = false
			}
			continue
		}

		if dollarTag != "" {
			if ch == '$' && strings.HasPrefix(sqlText[i:], dollarTag) {
				current.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			current.WriteByte(ch)
			continue
		}

		if inSingleQuoted {
			current.WriteByte(ch)
			if ch == '\'' {
				if next == '\'' {
					current.WriteByte(next)
					i++
					continue
				}
				inSingleQuoted = false
			}
			continue
		}

		if inDoubleQuoted {
			current.WriteByte(ch)
			if ch == '"' {
				if next == '"' {
					current.WriteByte(next)
					i++
					continue
				}
				inDoubleQuoted = false
			}
			continue
		}

		if ch == '-' && next == '-' {
			current.WriteByte(ch)
			current.WriteByte(next)
			i++
			inLineComment = true
			continue
		}

		if ch == '/' && next == '*' {
			current.WriteByte(ch)
			current.WriteByte(next)
			i++
			inBlockComment = true
			continue
		}

		if ch == '\'' {
			current.WriteByte(ch)
			inSingleQuoted = true
			continue
		}

		if ch == '"' {
			current.WriteByte(ch)
			inDoubleQuoted = true
			continue
		}

		if ch == '$' {
			if tag, consumed := parseDollarTag(sqlText[i:]); consumed > 0 {
				current.WriteString(tag)
				dollarTag = tag
				i += consumed - 1
				continue
			}
		}

		if ch == ';' {
			if stmt := strings.TrimSpace(current.String()); stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

func parseDollarTag(input string) (string, int) {
	if len(input) < 2 || input[0] != '$' {
		return "", 0
	}

	if input[1] == '$' {
		return "$$", 2
	}

	i := 1
	for i < len(input) {
		if input[i] == '$' {
			tag := input[1:i]
			if len(tag) == 0 {
				return "", 0
			}
			if !isDollarTagStart(rune(tag[0])) {
				return "", 0
			}
			for _, r := range tag[1:] {
				if !isDollarTagContinue(r) {
					return "", 0
				}
			}
			return input[:i+1], i + 1
		}
		i++
	}

	return "", 0
}

func isDollarTagStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isDollarTagContinue(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

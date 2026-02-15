package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

const (
	defaultLimit         = 50
	maxLimit             = 1000
	auditTimestampLayout = "2006-01-02T15:04:05.000000000Z"
)

type SQLiteLogger struct {
	db *sql.DB
}

func NewSQLiteLogger(db *sql.DB) (*SQLiteLogger, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &SQLiteLogger{db: db}, nil
}

func (l *SQLiteLogger) Log(ctx context.Context, entry Entry) error {
	operation := strings.TrimSpace(entry.Operation)
	if operation == "" {
		return fmt.Errorf("operation is required")
	}
	if entry.EnvironmentID == nil {
		return fmt.Errorf("environment id is required")
	}
	actor := strings.TrimSpace(entry.Actor)
	if actor == "" {
		actor = "local"
	}
	ts := entry.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	metadataJSON := "{}"
	if entry.Metadata != nil {
		b, err := json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	q := dbpkg.NewQueries(l.db)
	_, err := q.InsertAuditLog(ctx, dbpkg.AuditLogRow{
		Actor:           actor,
		Timestamp:       ts.UTC().Format(auditTimestampLayout),
		EnvironmentID:   entry.EnvironmentID,
		Operation:       operation,
		ResourceSummary: entry.ResourceSummary,
		ReleaseID:       entry.ReleaseID,
		MetadataJSON:    metadataJSON,
	})
	if err != nil {
		return fmt.Errorf("insert audit log entry: %w", err)
	}
	return nil
}

func (l *SQLiteLogger) Query(ctx context.Context, filter Filter) (QueryResult, error) {
	if filter.WebsiteID <= 0 {
		return QueryResult{}, fmt.Errorf("website id is required")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	clauses := []string{"w.id = ?"}
	args := []any{filter.WebsiteID}
	if filter.EnvironmentID != nil {
		clauses = append(clauses, "e.id = ?")
		args = append(args, *filter.EnvironmentID)
	}
	if strings.TrimSpace(filter.Operation) != "" {
		clauses = append(clauses, "a.operation = ?")
		args = append(args, strings.TrimSpace(filter.Operation))
	}
	if filter.Since != nil {
		clauses = append(clauses, "a.timestamp >= ?")
		args = append(args, filter.Since.UTC().Format(auditTimestampLayout))
	}
	if filter.Until != nil {
		clauses = append(clauses, "a.timestamp <= ?")
		args = append(args, filter.Until.UTC().Format(auditTimestampLayout))
	}
	where := strings.Join(clauses, " AND ")

	countQuery := `
SELECT COUNT(*)
FROM audit_log a
JOIN environments e ON e.id = a.environment_id
JOIN websites w ON w.id = e.website_id
WHERE ` + where
	var total int
	if err := l.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return QueryResult{}, fmt.Errorf("count audit log rows: %w", err)
	}

	query := `
SELECT a.id, a.actor, a.timestamp, a.environment_id, a.operation, a.resource_summary, a.release_id, a.metadata_json
FROM audit_log a
JOIN environments e ON e.id = a.environment_id
JOIN websites w ON w.id = e.website_id
WHERE ` + where + `
ORDER BY a.timestamp DESC, a.id DESC
LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := l.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return QueryResult{}, fmt.Errorf("query audit log rows: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var row Entry
		var ts string
		var metadataRaw string
		if err := rows.Scan(&row.ID, &row.Actor, &ts, &row.EnvironmentID, &row.Operation, &row.ResourceSummary, &row.ReleaseID, &metadataRaw); err != nil {
			return QueryResult{}, fmt.Errorf("scan audit log row: %w", err)
		}
		parsedTS, err := parseAuditTimestamp(ts)
		if err != nil {
			return QueryResult{}, fmt.Errorf("parse audit timestamp %q: %w", ts, err)
		}
		row.Timestamp = parsedTS
		if strings.TrimSpace(metadataRaw) != "" {
			meta := map[string]any{}
			if err := json.Unmarshal([]byte(metadataRaw), &meta); err != nil {
				return QueryResult{}, fmt.Errorf("parse audit metadata json: %w", err)
			}
			row.Metadata = meta
		}
		entries = append(entries, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, fmt.Errorf("iterate audit log rows: %w", err)
	}
	return QueryResult{Entries: entries, Total: total, Limit: limit, Offset: offset}, nil
}

func parseAuditTimestamp(raw string) (time.Time, error) {
	if ts, err := time.Parse(auditTimestampLayout, raw); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339Nano, raw)
}

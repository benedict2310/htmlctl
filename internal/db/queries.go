package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Queries struct {
	db queryer
}

const sqliteMaxVariables = 999

// allowedDeleteTargets defines the only table/column identifier pairs that may
// be interpolated into SQL delete helpers.
//
// Identifiers here use canonical lowercase names and are matched
// case-sensitively. Do not mutate this map after init.
var allowedDeleteTargets = map[string]map[string]bool{
	"pages": {
		"name": true,
	},
	"components": {
		"name": true,
	},
	"style_bundles": {
		"name": true,
	},
	"assets": {
		"filename": true,
	},
}

func NewQueries(db queryer) *Queries {
	return &Queries{db: db}
}

func (q *Queries) InsertWebsite(ctx context.Context, in WebsiteRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO websites(name, default_style_bundle, base_template) VALUES(?, ?, ?)`, in.Name, in.DefaultStyleBundle, in.BaseTemplate)
	if err != nil {
		return 0, fmt.Errorf("insert website: %w", err)
	}
	return lastInsertID("insert website", res)
}

func (q *Queries) GetWebsiteByName(ctx context.Context, name string) (WebsiteRow, error) {
	var out WebsiteRow
	err := q.db.QueryRowContext(ctx, `SELECT id, name, default_style_bundle, base_template, created_at, updated_at FROM websites WHERE name = ?`, name).
		Scan(&out.ID, &out.Name, &out.DefaultStyleBundle, &out.BaseTemplate, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return out, fmt.Errorf("get website by name: %w", err)
	}
	return out, nil
}

func (q *Queries) InsertEnvironment(ctx context.Context, in EnvironmentRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO environments(website_id, name, active_release_id) VALUES(?, ?, ?)`, in.WebsiteID, in.Name, in.ActiveReleaseID)
	if err != nil {
		return 0, fmt.Errorf("insert environment: %w", err)
	}
	return lastInsertID("insert environment", res)
}

func (q *Queries) GetEnvironmentByName(ctx context.Context, websiteID int64, name string) (EnvironmentRow, error) {
	var out EnvironmentRow
	err := q.db.QueryRowContext(ctx, `SELECT id, website_id, name, active_release_id, created_at, updated_at FROM environments WHERE website_id = ? AND name = ?`, websiteID, name).
		Scan(&out.ID, &out.WebsiteID, &out.Name, &out.ActiveReleaseID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return out, fmt.Errorf("get environment by name: %w", err)
	}
	return out, nil
}

func (q *Queries) InsertPage(ctx context.Context, in PageRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO pages(website_id, name, route, title, description, layout_json, head_json, content_hash) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`, in.WebsiteID, in.Name, in.Route, in.Title, in.Description, in.LayoutJSON, in.HeadJSONOrDefault(), in.ContentHash)
	if err != nil {
		return 0, fmt.Errorf("insert page: %w", err)
	}
	return lastInsertID("insert page", res)
}

func (q *Queries) UpsertPage(ctx context.Context, in PageRow) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO pages(website_id, name, route, title, description, layout_json, head_json, content_hash)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(website_id, name) DO UPDATE SET
  route=excluded.route,
  title=excluded.title,
  description=excluded.description,
  layout_json=excluded.layout_json,
  head_json=excluded.head_json,
  content_hash=excluded.content_hash,
  updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
`, in.WebsiteID, in.Name, in.Route, in.Title, in.Description, in.LayoutJSON, in.HeadJSONOrDefault(), in.ContentHash)
	if err != nil {
		return fmt.Errorf("upsert page: %w", err)
	}
	return nil
}

func (q *Queries) ListPagesByWebsite(ctx context.Context, websiteID int64) ([]PageRow, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, website_id, name, route, title, description, layout_json, head_json, content_hash, created_at, updated_at FROM pages WHERE website_id = ? ORDER BY name`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list pages by website: %w", err)
	}
	defer rows.Close()

	out := []PageRow{}
	for rows.Next() {
		var row PageRow
		if err := rows.Scan(&row.ID, &row.WebsiteID, &row.Name, &row.Route, &row.Title, &row.Description, &row.LayoutJSON, &row.HeadJSON, &row.ContentHash, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan page row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate page rows: %w", err)
	}
	return out, nil
}

func (q *Queries) DeletePagesNotIn(ctx context.Context, websiteID int64, names []string) (int64, error) {
	return q.deleteByWebsiteNotIn(ctx, "pages", "name", websiteID, names)
}

func (q *Queries) DeletePageByName(ctx context.Context, websiteID int64, name string) (int64, error) {
	return q.deleteByWebsiteAndKey(ctx, "pages", "name", websiteID, name)
}

func (q *Queries) InsertComponent(ctx context.Context, in ComponentRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO components(website_id, name, scope, content_hash) VALUES(?, ?, ?, ?)`, in.WebsiteID, in.Name, in.Scope, in.ContentHash)
	if err != nil {
		return 0, fmt.Errorf("insert component: %w", err)
	}
	return lastInsertID("insert component", res)
}

func (q *Queries) UpsertComponent(ctx context.Context, in ComponentRow) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO components(website_id, name, scope, content_hash)
VALUES(?, ?, ?, ?)
ON CONFLICT(website_id, name) DO UPDATE SET
  scope=excluded.scope,
  content_hash=excluded.content_hash,
  updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
`, in.WebsiteID, in.Name, in.Scope, in.ContentHash)
	if err != nil {
		return fmt.Errorf("upsert component: %w", err)
	}
	return nil
}

func (q *Queries) ListComponentsByWebsite(ctx context.Context, websiteID int64) ([]ComponentRow, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, website_id, name, scope, content_hash, created_at, updated_at FROM components WHERE website_id = ? ORDER BY name`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list components by website: %w", err)
	}
	defer rows.Close()

	out := []ComponentRow{}
	for rows.Next() {
		var row ComponentRow
		if err := rows.Scan(&row.ID, &row.WebsiteID, &row.Name, &row.Scope, &row.ContentHash, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan component row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate component rows: %w", err)
	}
	return out, nil
}

func (q *Queries) DeleteComponentsNotIn(ctx context.Context, websiteID int64, names []string) (int64, error) {
	return q.deleteByWebsiteNotIn(ctx, "components", "name", websiteID, names)
}

func (q *Queries) DeleteComponentByName(ctx context.Context, websiteID int64, name string) (int64, error) {
	return q.deleteByWebsiteAndKey(ctx, "components", "name", websiteID, name)
}

func (q *Queries) InsertStyleBundle(ctx context.Context, in StyleBundleRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO style_bundles(website_id, name, files_json) VALUES(?, ?, ?)`, in.WebsiteID, in.Name, in.FilesJSON)
	if err != nil {
		return 0, fmt.Errorf("insert style bundle: %w", err)
	}
	return lastInsertID("insert style bundle", res)
}

func (q *Queries) UpsertStyleBundle(ctx context.Context, in StyleBundleRow) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO style_bundles(website_id, name, files_json)
VALUES(?, ?, ?)
ON CONFLICT(website_id, name) DO UPDATE SET
  files_json=excluded.files_json,
  updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
`, in.WebsiteID, in.Name, in.FilesJSON)
	if err != nil {
		return fmt.Errorf("upsert style bundle: %w", err)
	}
	return nil
}

func (q *Queries) ListStyleBundlesByWebsite(ctx context.Context, websiteID int64) ([]StyleBundleRow, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, website_id, name, files_json, created_at, updated_at FROM style_bundles WHERE website_id = ? ORDER BY name`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list style bundles by website: %w", err)
	}
	defer rows.Close()

	out := []StyleBundleRow{}
	for rows.Next() {
		var row StyleBundleRow
		if err := rows.Scan(&row.ID, &row.WebsiteID, &row.Name, &row.FilesJSON, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan style bundle row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate style bundle rows: %w", err)
	}
	return out, nil
}

func (q *Queries) DeleteStyleBundlesNotIn(ctx context.Context, websiteID int64, names []string) (int64, error) {
	return q.deleteByWebsiteNotIn(ctx, "style_bundles", "name", websiteID, names)
}

func (q *Queries) DeleteStyleBundleByName(ctx context.Context, websiteID int64, name string) (int64, error) {
	return q.deleteByWebsiteAndKey(ctx, "style_bundles", "name", websiteID, name)
}

func (q *Queries) InsertAsset(ctx context.Context, in AssetRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO assets(website_id, filename, content_type, size_bytes, content_hash) VALUES(?, ?, ?, ?, ?)`, in.WebsiteID, in.Filename, in.ContentType, in.SizeBytes, in.ContentHash)
	if err != nil {
		return 0, fmt.Errorf("insert asset: %w", err)
	}
	return lastInsertID("insert asset", res)
}

func (q *Queries) UpsertAsset(ctx context.Context, in AssetRow) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO assets(website_id, filename, content_type, size_bytes, content_hash)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(website_id, filename) DO UPDATE SET
  content_type=excluded.content_type,
  size_bytes=excluded.size_bytes,
  content_hash=excluded.content_hash
`, in.WebsiteID, in.Filename, in.ContentType, in.SizeBytes, in.ContentHash)
	if err != nil {
		return fmt.Errorf("upsert asset: %w", err)
	}
	return nil
}

func (q *Queries) ListAssetsByWebsite(ctx context.Context, websiteID int64) ([]AssetRow, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, website_id, filename, content_type, size_bytes, content_hash, created_at FROM assets WHERE website_id = ? ORDER BY filename`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list assets by website: %w", err)
	}
	defer rows.Close()

	out := []AssetRow{}
	for rows.Next() {
		var row AssetRow
		if err := rows.Scan(&row.ID, &row.WebsiteID, &row.Filename, &row.ContentType, &row.SizeBytes, &row.ContentHash, &row.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan asset row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate asset rows: %w", err)
	}
	return out, nil
}

func (q *Queries) DeleteAssetsNotIn(ctx context.Context, websiteID int64, filenames []string) (int64, error) {
	return q.deleteByWebsiteNotIn(ctx, "assets", "filename", websiteID, filenames)
}

func (q *Queries) DeleteAssetByFilename(ctx context.Context, websiteID int64, filename string) (int64, error) {
	return q.deleteByWebsiteAndKey(ctx, "assets", "filename", websiteID, filename)
}

func (q *Queries) InsertRelease(ctx context.Context, in ReleaseRow) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO releases(id, environment_id, manifest_json, output_hashes, build_log, status) VALUES(?, ?, ?, ?, ?, ?)`, in.ID, in.EnvironmentID, in.ManifestJSON, in.OutputHashes, in.BuildLog, in.Status)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}
	return nil
}

func (q *Queries) GetReleaseByID(ctx context.Context, id string) (ReleaseRow, error) {
	var out ReleaseRow
	err := q.db.QueryRowContext(ctx, `SELECT id, environment_id, manifest_json, output_hashes, build_log, status, created_at FROM releases WHERE id = ?`, id).
		Scan(&out.ID, &out.EnvironmentID, &out.ManifestJSON, &out.OutputHashes, &out.BuildLog, &out.Status, &out.CreatedAt)
	if err != nil {
		return out, fmt.Errorf("get release by id: %w", err)
	}
	return out, nil
}

func (q *Queries) ListReleasesByEnvironment(ctx context.Context, environmentID int64) ([]ReleaseRow, error) {
	return q.listReleasesByEnvironment(ctx, environmentID, nil, nil)
}

func (q *Queries) ListReleasesByEnvironmentPage(ctx context.Context, environmentID int64, limit, offset int) ([]ReleaseRow, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	return q.listReleasesByEnvironment(ctx, environmentID, &limit, &offset)
}

func (q *Queries) listReleasesByEnvironment(ctx context.Context, environmentID int64, limit, offset *int) ([]ReleaseRow, error) {
	query := `SELECT id, environment_id, manifest_json, output_hashes, build_log, status, created_at FROM releases WHERE environment_id = ? ORDER BY created_at DESC, id DESC`
	args := []any{environmentID}
	if limit != nil && offset != nil {
		query += " LIMIT ? OFFSET ?"
		args = append(args, *limit, *offset)
	}

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list releases by environment: %w", err)
	}
	defer rows.Close()

	out := []ReleaseRow{}
	for rows.Next() {
		var row ReleaseRow
		if err := rows.Scan(&row.ID, &row.EnvironmentID, &row.ManifestJSON, &row.OutputHashes, &row.BuildLog, &row.Status, &row.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan release row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate release rows: %w", err)
	}
	return out, nil
}

func (q *Queries) ListLatestReleaseActors(ctx context.Context, environmentID int64, releaseIDs []string) (map[string]string, error) {
	actors := map[string]string{}
	if len(releaseIDs) == 0 {
		return actors, nil
	}

	args := make([]any, 0, len(releaseIDs)+1)
	args = append(args, environmentID)
	for _, id := range releaseIDs {
		args = append(args, id)
	}

	query := `
SELECT a.release_id, a.actor
FROM audit_log a
WHERE a.environment_id = ?
  AND a.release_id IN (` + placeholders(len(releaseIDs)) + `)
  AND a.id = (
    SELECT a2.id
    FROM audit_log a2
    WHERE a2.environment_id = a.environment_id AND a2.release_id = a.release_id
    ORDER BY a2.timestamp DESC, a2.id DESC
    LIMIT 1
  )`

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest release actors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var releaseID string
		var actor string
		if err := rows.Scan(&releaseID, &actor); err != nil {
			return nil, fmt.Errorf("scan latest release actor row: %w", err)
		}
		actors[releaseID] = actor
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest release actor rows: %w", err)
	}

	return actors, nil
}

func (q *Queries) UpdateEnvironmentActiveRelease(ctx context.Context, environmentID int64, releaseID *string) error {
	_, err := q.db.ExecContext(ctx, `
	UPDATE environments
	SET active_release_id = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
	WHERE id = ?
	`, releaseID, environmentID)
	if err != nil {
		return fmt.Errorf("update environment active release: %w", err)
	}
	return nil
}

func (q *Queries) InsertAuditLog(ctx context.Context, in AuditLogRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO audit_log(actor, environment_id, operation, resource_summary, release_id, metadata_json) VALUES(?, ?, ?, ?, ?, ?)`, in.Actor, in.EnvironmentID, in.Operation, in.ResourceSummary, in.ReleaseID, in.MetadataJSON)
	if err != nil {
		return 0, fmt.Errorf("insert audit log: %w", err)
	}
	return lastInsertID("insert audit log", res)
}

func (q *Queries) InsertDomainBinding(ctx context.Context, in DomainBindingRow) (int64, error) {
	res, err := q.db.ExecContext(
		ctx,
		`INSERT INTO domain_bindings(domain, environment_id) VALUES(?, ?)`,
		in.Domain,
		in.EnvironmentID,
	)
	if err != nil {
		return 0, fmt.Errorf("insert domain binding: %w", err)
	}
	return lastInsertID("insert domain binding", res)
}

func (q *Queries) RestoreDomainBinding(ctx context.Context, in DomainBindingRow) error {
	_, err := q.db.ExecContext(
		ctx,
		`INSERT INTO domain_bindings(id, domain, environment_id, created_at, updated_at) VALUES(?, ?, ?, ?, ?)`,
		in.ID,
		in.Domain,
		in.EnvironmentID,
		in.CreatedAt,
		in.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("restore domain binding: %w", err)
	}
	return nil
}

func (q *Queries) GetDomainBindingByDomain(ctx context.Context, domain string) (DomainBindingResolvedRow, error) {
	var out DomainBindingResolvedRow
	err := q.db.QueryRowContext(ctx, `
SELECT
  d.id,
  d.domain,
  d.environment_id,
  w.name AS website_name,
  e.name AS environment_name,
  d.created_at,
  d.updated_at
FROM domain_bindings d
JOIN environments e ON e.id = d.environment_id
JOIN websites w ON w.id = e.website_id
WHERE d.domain = ?
`, domain).Scan(
		&out.ID,
		&out.Domain,
		&out.EnvironmentID,
		&out.WebsiteName,
		&out.EnvironmentName,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return out, fmt.Errorf("get domain binding by domain: %w", err)
	}
	return out, nil
}

func (q *Queries) ListDomainBindings(ctx context.Context, websiteName, environmentName string) ([]DomainBindingResolvedRow, error) {
	query := `
SELECT
  d.id,
  d.domain,
  d.environment_id,
  w.name AS website_name,
  e.name AS environment_name,
  d.created_at,
  d.updated_at
FROM domain_bindings d
JOIN environments e ON e.id = d.environment_id
JOIN websites w ON w.id = e.website_id
`
	where := []string{}
	args := []any{}
	if strings.TrimSpace(websiteName) != "" {
		where = append(where, "w.name = ?")
		args = append(args, strings.TrimSpace(websiteName))
	}
	if strings.TrimSpace(environmentName) != "" {
		where = append(where, "e.name = ?")
		args = append(args, strings.TrimSpace(environmentName))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY d.domain ASC"

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list domain bindings: %w", err)
	}
	defer rows.Close()

	out := []DomainBindingResolvedRow{}
	for rows.Next() {
		var row DomainBindingResolvedRow
		if err := rows.Scan(
			&row.ID,
			&row.Domain,
			&row.EnvironmentID,
			&row.WebsiteName,
			&row.EnvironmentName,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain binding row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domain binding rows: %w", err)
	}
	return out, nil
}

func (q *Queries) DeleteDomainBindingByDomain(ctx context.Context, domain string) (bool, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM domain_bindings WHERE domain = ?`, domain)
	if err != nil {
		return false, fmt.Errorf("delete domain binding: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("domain binding rows affected: %w", err)
	}
	return affected > 0, nil
}

// deleteByWebsiteNotIn deletes rows from an allowlisted table/column pair for a
// website where the key column is not present in values.
//
// To support additional table/column pairs, update allowedDeleteTargets.
func (q *Queries) deleteByWebsiteNotIn(ctx context.Context, table, column string, websiteID int64, values []string) (int64, error) {
	if err := validateDeleteTarget(table, column); err != nil {
		return 0, err
	}

	// SQLite limits query variables. Fall back to set-difference deletes when
	// the keep-list would exceed the variable limit.
	if len(values) >= sqliteMaxVariables {
		return q.deleteByWebsiteSetDifference(ctx, table, column, websiteID, values)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE website_id = ?", table)
	args := []any{websiteID}
	if len(values) > 0 {
		query += fmt.Sprintf(" AND %s NOT IN (%s)", column, placeholders(len(values)))
		for _, v := range values {
			args = append(args, v)
		}
	}
	res, err := q.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete %s rows not in set: %w", table, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected for delete %s rows: %w", table, err)
	}
	return n, nil
}

// deleteByWebsiteSetDifference deletes non-keep values by reading keys from an
// allowlisted table/column pair and deleting row-by-row.
//
// To support additional table/column pairs, update allowedDeleteTargets.
func (q *Queries) deleteByWebsiteSetDifference(ctx context.Context, table, column string, websiteID int64, keepValues []string) (int64, error) {
	if err := validateDeleteTarget(table, column); err != nil {
		return 0, err
	}

	keep := make(map[string]struct{}, len(keepValues))
	for _, v := range keepValues {
		keep[v] = struct{}{}
	}

	rows, err := q.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE website_id = ?", column, table), websiteID)
	if err != nil {
		return 0, fmt.Errorf("list %s keys for deletion: %w", table, err)
	}
	defer rows.Close()

	var deleted int64
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return 0, fmt.Errorf("scan %s key for deletion: %w", table, err)
		}
		if _, ok := keep[key]; ok {
			continue
		}
		n, err := q.deleteByWebsiteAndKey(ctx, table, column, websiteID, key)
		if err != nil {
			return 0, err
		}
		deleted += n
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate %s keys for deletion: %w", table, err)
	}
	return deleted, nil
}

// deleteByWebsiteAndKey deletes a single row for an allowlisted table/column
// pair scoped to websiteID.
//
// To support additional table/column pairs, update allowedDeleteTargets.
func (q *Queries) deleteByWebsiteAndKey(ctx context.Context, table, column string, websiteID int64, value string) (int64, error) {
	if err := validateDeleteTarget(table, column); err != nil {
		return 0, err
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE website_id = ? AND %s = ?", table, column)
	res, err := q.db.ExecContext(ctx, query, websiteID, value)
	if err != nil {
		return 0, fmt.Errorf("delete %s row by %s: %w", table, column, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected for delete %s row by %s: %w", table, column, err)
	}
	return n, nil
}

func validateDeleteTarget(table, column string) error {
	columns, ok := allowedDeleteTargets[table]
	if !ok || !columns[column] {
		return fmt.Errorf("invalid table/column: %q/%q", table, column)
	}
	return nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func lastInsertID(op string, res sql.Result) (int64, error) {
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s last insert id: %w", op, err)
	}
	return id, nil
}

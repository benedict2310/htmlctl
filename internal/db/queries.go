package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Queries struct {
	db *sql.DB
}

func NewQueries(db *sql.DB) *Queries {
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

func (q *Queries) InsertPage(ctx context.Context, in PageRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO pages(website_id, name, route, title, description, layout_json, content_hash) VALUES(?, ?, ?, ?, ?, ?, ?)`, in.WebsiteID, in.Name, in.Route, in.Title, in.Description, in.LayoutJSON, in.ContentHash)
	if err != nil {
		return 0, fmt.Errorf("insert page: %w", err)
	}
	return lastInsertID("insert page", res)
}

func (q *Queries) InsertComponent(ctx context.Context, in ComponentRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO components(website_id, name, scope, content_hash) VALUES(?, ?, ?, ?)`, in.WebsiteID, in.Name, in.Scope, in.ContentHash)
	if err != nil {
		return 0, fmt.Errorf("insert component: %w", err)
	}
	return lastInsertID("insert component", res)
}

func (q *Queries) InsertStyleBundle(ctx context.Context, in StyleBundleRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO style_bundles(website_id, name, files_json) VALUES(?, ?, ?)`, in.WebsiteID, in.Name, in.FilesJSON)
	if err != nil {
		return 0, fmt.Errorf("insert style bundle: %w", err)
	}
	return lastInsertID("insert style bundle", res)
}

func (q *Queries) InsertAsset(ctx context.Context, in AssetRow) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO assets(website_id, filename, content_type, size_bytes, content_hash) VALUES(?, ?, ?, ?, ?)`, in.WebsiteID, in.Filename, in.ContentType, in.SizeBytes, in.ContentHash)
	if err != nil {
		return 0, fmt.Errorf("insert asset: %w", err)
	}
	return lastInsertID("insert asset", res)
}

func (q *Queries) InsertRelease(ctx context.Context, in ReleaseRow) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO releases(id, environment_id, manifest_json, output_hashes, build_log, status) VALUES(?, ?, ?, ?, ?, ?)`, in.ID, in.EnvironmentID, in.ManifestJSON, in.OutputHashes, in.BuildLog, in.Status)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
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

func lastInsertID(op string, res sql.Result) (int64, error) {
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s last insert id: %w", op, err)
	}
	return id, nil
}

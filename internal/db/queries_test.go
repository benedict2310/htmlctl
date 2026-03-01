package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func setupDB(t *testing.T) (*Queries, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := Open(DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := RunMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return NewQueries(db), func() { _ = db.Close() }
}

func TestQueriesInsertAndFetchCoreRows(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	website, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if website.ID != websiteID {
		t.Fatalf("unexpected website id: got=%d want=%d", website.ID, websiteID)
	}
	if website.HeadJSON != "{}" {
		t.Fatalf("expected default website head_json {}, got %q", website.HeadJSON)
	}

	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "index", Route: "/", Title: "Home", Description: "desc", LayoutJSON: `["header"]`, ContentHash: "hash-page"}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}
	if _, err := q.InsertComponent(ctx, ComponentRow{WebsiteID: websiteID, Name: "header", Scope: "global", ContentHash: "hash-comp"}); err != nil {
		t.Fatalf("InsertComponent() error = %v", err)
	}
	if _, err := q.InsertStyleBundle(ctx, StyleBundleRow{WebsiteID: websiteID, Name: "default", FilesJSON: `[{"filename":"tokens.css","hash":"x"}]`}); err != nil {
		t.Fatalf("InsertStyleBundle() error = %v", err)
	}
	if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: websiteID, Filename: "logo.svg", ContentType: "image/svg+xml", SizeBytes: 64, ContentHash: "hash-asset"}); err != nil {
		t.Fatalf("InsertAsset() error = %v", err)
	}
	if err := q.InsertRelease(ctx, ReleaseRow{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", EnvironmentID: envID, ManifestJSON: `{}`, OutputHashes: `{}`, BuildLog: "ok", Status: "active"}); err != nil {
		t.Fatalf("InsertRelease() error = %v", err)
	}
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{Actor: "test", EnvironmentID: &envID, Operation: "apply", ResourceSummary: "changed", MetadataJSON: `{}`}); err != nil {
		t.Fatalf("InsertAuditLog() error = %v", err)
	}
}

func TestQueriesForeignKeyConstraint(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	if _, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: 9999, Name: "staging"}); err == nil {
		t.Fatalf("expected foreign key violation for invalid website id")
	}
}

func TestUpsertPagePersistsHeadJSON(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	headJSON := `{"canonicalURL":"https://example.com/","meta":{"robots":"index,follow"}}`
	if err := q.UpsertPage(ctx, PageRow{
		WebsiteID:   websiteID,
		Name:        "index",
		Route:       "/",
		Title:       "Home",
		Description: "Home page",
		LayoutJSON:  "[]",
		HeadJSON:    headJSON,
		ContentHash: "sha256:page",
	}); err != nil {
		t.Fatalf("UpsertPage() error = %v", err)
	}

	rows, err := q.ListPagesByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListPagesByWebsite() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one page row, got %d", len(rows))
	}
	if rows[0].HeadJSON != headJSON {
		t.Fatalf("unexpected head_json: got %q want %q", rows[0].HeadJSON, headJSON)
	}
}

func TestInsertPageDefaultsHeadJSONToObject(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := q.InsertPage(ctx, PageRow{
		WebsiteID:   websiteID,
		Name:        "index",
		Route:       "/",
		Title:       "Home",
		Description: "Home page",
		LayoutJSON:  "[]",
		ContentHash: "sha256:page",
	}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}

	rows, err := q.ListPagesByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListPagesByWebsite() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one page row, got %d", len(rows))
	}
	if rows[0].HeadJSON != "{}" {
		t.Fatalf("expected default head_json {}, got %q", rows[0].HeadJSON)
	}
}

func TestUpdateWebsiteSpecPersistsHeadJSONAndContentHash(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if err := q.UpdateWebsiteSpec(ctx, WebsiteRow{
		ID:                 websiteID,
		DefaultStyleBundle: "brand",
		BaseTemplate:       "marketing",
		HeadJSON:           `{"icons":{"svg":"branding/favicon.svg"}}`,
		ContentHash:        "sha256:website",
	}); err != nil {
		t.Fatalf("UpdateWebsiteSpec() error = %v", err)
	}

	row, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if row.DefaultStyleBundle != "brand" || row.BaseTemplate != "marketing" {
		t.Fatalf("unexpected website row: %#v", row)
	}
	if row.HeadJSON != `{"icons":{"svg":"branding/favicon.svg"}}` {
		t.Fatalf("unexpected website head_json: %q", row.HeadJSON)
	}
	if row.ContentHash != "sha256:website" {
		t.Fatalf("unexpected website content hash: %q", row.ContentHash)
	}
}

func TestWebsiteIconsCRUD(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if err := q.UpsertWebsiteIcon(ctx, WebsiteIconRow{
		WebsiteID:   websiteID,
		Slot:        "svg",
		SourcePath:  "branding/favicon.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   12,
		ContentHash: "sha256:svg",
	}); err != nil {
		t.Fatalf("UpsertWebsiteIcon(svg) error = %v", err)
	}
	if err := q.UpsertWebsiteIcon(ctx, WebsiteIconRow{
		WebsiteID:   websiteID,
		Slot:        "ico",
		SourcePath:  "branding/favicon.ico",
		ContentType: "image/x-icon",
		SizeBytes:   8,
		ContentHash: "sha256:ico",
	}); err != nil {
		t.Fatalf("UpsertWebsiteIcon(ico) error = %v", err)
	}
	rows, err := q.ListWebsiteIconsByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListWebsiteIconsByWebsite() error = %v", err)
	}
	if len(rows) != 2 || rows[0].Slot != "ico" || rows[1].Slot != "svg" {
		t.Fatalf("unexpected website icon rows: %#v", rows)
	}
	deleted, err := q.DeleteWebsiteIconsNotIn(ctx, websiteID, []string{"svg"})
	if err != nil {
		t.Fatalf("DeleteWebsiteIconsNotIn() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one deleted website icon row, got %d", deleted)
	}
	rows, err = q.ListWebsiteIconsByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListWebsiteIconsByWebsite(second) error = %v", err)
	}
	if len(rows) != 1 || rows[0].Slot != "svg" {
		t.Fatalf("unexpected website icon rows after delete: %#v", rows)
	}
}

func TestQueriesUniqueConstraints(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"}); err == nil {
		t.Fatalf("expected duplicate website name violation")
	}

	if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "index", Route: "/", LayoutJSON: "[]", ContentHash: "a"}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}
	if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: "home", Route: "/", LayoutJSON: "[]", ContentHash: "b"}); err == nil {
		t.Fatalf("expected duplicate route violation")
	}
}

func TestQueriesErrorBranches(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	if _, err := q.GetWebsiteByName(ctx, "missing"); err == nil {
		t.Fatalf("expected not-found error for website")
	}

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	if _, err := q.InsertComponent(ctx, ComponentRow{WebsiteID: 9999, Name: "header", Scope: "global", ContentHash: "x"}); err == nil {
		t.Fatalf("expected component FK error")
	}
	if _, err := q.InsertStyleBundle(ctx, StyleBundleRow{WebsiteID: 9999, Name: "default", FilesJSON: "[]"}); err == nil {
		t.Fatalf("expected style bundle FK error")
	}
	if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: 9999, Filename: "x", ContentType: "text/plain", SizeBytes: 1, ContentHash: "x"}); err == nil {
		t.Fatalf("expected asset FK error")
	}
	if err := q.InsertRelease(ctx, ReleaseRow{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAW", EnvironmentID: 9999, ManifestJSON: "{}", OutputHashes: "{}", BuildLog: "", Status: "active"}); err == nil {
		t.Fatalf("expected release FK error")
	}

	badEnv := int64(9999)
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{Actor: "x", EnvironmentID: &badEnv, Operation: "apply", ResourceSummary: "x", MetadataJSON: "{}"}); err == nil {
		t.Fatalf("expected audit log FK error")
	}

	releaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	if err := q.InsertRelease(ctx, ReleaseRow{ID: releaseID, EnvironmentID: envID, ManifestJSON: "{}", OutputHashes: "{}", BuildLog: "", Status: "active"}); err != nil {
		t.Fatalf("InsertRelease valid error = %v", err)
	}
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{Actor: "x", EnvironmentID: &envID, Operation: "apply", ResourceSummary: "x", ReleaseID: &releaseID, MetadataJSON: "{}"}); err != nil {
		t.Fatalf("InsertAuditLog valid error = %v", err)
	}

	// Check GetWebsiteByName surfaces sql.ErrNoRows through wrapped error.
	_, err = q.GetWebsiteByName(ctx, "still-missing")
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected wrapped sql.ErrNoRows, got %v", err)
	}
}

func TestDeleteAssetsNotInHandlesLargeKeepSet(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	for _, name := range []string{"assets/a.svg", "assets/b.svg", "assets/c.svg"} {
		if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: websiteID, Filename: name, ContentType: "image/svg+xml", SizeBytes: 10, ContentHash: "hash-" + name}); err != nil {
			t.Fatalf("InsertAsset(%q) error = %v", name, err)
		}
	}

	keep := make([]string, 0, 1000)
	for i := 0; i < 998; i++ {
		keep = append(keep, fmt.Sprintf("missing-%d", i))
	}
	keep = append(keep, "assets/a.svg", "assets/b.svg")

	deleted, err := q.DeleteAssetsNotIn(ctx, websiteID, keep)
	if err != nil {
		t.Fatalf("DeleteAssetsNotIn() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted row, got %d", deleted)
	}

	rows, err := q.ListAssetsByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListAssetsByWebsite() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 assets remaining, got %d", len(rows))
	}
}

type noSQLQueryer struct {
	execCalls  int
	queryCalls int
}

func (q *noSQLQueryer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	q.execCalls++
	return nil, errors.New("unexpected ExecContext call")
}

func (q *noSQLQueryer) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	q.queryCalls++
	return nil, errors.New("unexpected QueryContext call")
}

func (q *noSQLQueryer) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("unexpected QueryRowContext call")
}

func seedDeleteTargetRows(t *testing.T, q *Queries, ctx context.Context, table string, websiteID int64, keep, drop string) {
	t.Helper()
	switch table {
	case "pages":
		if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: keep, Route: "/" + keep, LayoutJSON: "[]", ContentHash: "hash-" + keep}); err != nil {
			t.Fatalf("InsertPage(keep) error = %v", err)
		}
		if _, err := q.InsertPage(ctx, PageRow{WebsiteID: websiteID, Name: drop, Route: "/" + drop, LayoutJSON: "[]", ContentHash: "hash-" + drop}); err != nil {
			t.Fatalf("InsertPage(drop) error = %v", err)
		}
	case "components":
		if _, err := q.InsertComponent(ctx, ComponentRow{WebsiteID: websiteID, Name: keep, Scope: "global", ContentHash: "hash-" + keep}); err != nil {
			t.Fatalf("InsertComponent(keep) error = %v", err)
		}
		if _, err := q.InsertComponent(ctx, ComponentRow{WebsiteID: websiteID, Name: drop, Scope: "global", ContentHash: "hash-" + drop}); err != nil {
			t.Fatalf("InsertComponent(drop) error = %v", err)
		}
	case "style_bundles":
		if _, err := q.InsertStyleBundle(ctx, StyleBundleRow{WebsiteID: websiteID, Name: keep, FilesJSON: "[]"}); err != nil {
			t.Fatalf("InsertStyleBundle(keep) error = %v", err)
		}
		if _, err := q.InsertStyleBundle(ctx, StyleBundleRow{WebsiteID: websiteID, Name: drop, FilesJSON: "[]"}); err != nil {
			t.Fatalf("InsertStyleBundle(drop) error = %v", err)
		}
	case "assets":
		if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: websiteID, Filename: keep, ContentType: "text/plain", SizeBytes: 1, ContentHash: "hash-" + keep}); err != nil {
			t.Fatalf("InsertAsset(keep) error = %v", err)
		}
		if _, err := q.InsertAsset(ctx, AssetRow{WebsiteID: websiteID, Filename: drop, ContentType: "text/plain", SizeBytes: 1, ContentHash: "hash-" + drop}); err != nil {
			t.Fatalf("InsertAsset(drop) error = %v", err)
		}
	default:
		t.Fatalf("unsupported table %q", table)
	}
}

func listDeleteTargetValues(t *testing.T, q *Queries, ctx context.Context, table string, websiteID int64) []string {
	t.Helper()
	switch table {
	case "pages":
		rows, err := q.ListPagesByWebsite(ctx, websiteID)
		if err != nil {
			t.Fatalf("ListPagesByWebsite() error = %v", err)
		}
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.Name)
		}
		return out
	case "components":
		rows, err := q.ListComponentsByWebsite(ctx, websiteID)
		if err != nil {
			t.Fatalf("ListComponentsByWebsite() error = %v", err)
		}
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.Name)
		}
		return out
	case "style_bundles":
		rows, err := q.ListStyleBundlesByWebsite(ctx, websiteID)
		if err != nil {
			t.Fatalf("ListStyleBundlesByWebsite() error = %v", err)
		}
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.Name)
		}
		return out
	case "assets":
		rows, err := q.ListAssetsByWebsite(ctx, websiteID)
		if err != nil {
			t.Fatalf("ListAssetsByWebsite() error = %v", err)
		}
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.Filename)
		}
		return out
	default:
		t.Fatalf("unsupported table %q", table)
		return nil
	}
}

func TestValidateDeleteTargetAllowlistedPairs(t *testing.T) {
	cases := []struct {
		table  string
		column string
	}{
		{table: "pages", column: "name"},
		{table: "components", column: "name"},
		{table: "style_bundles", column: "name"},
		{table: "assets", column: "filename"},
	}

	for _, tc := range cases {
		t.Run(tc.table+"."+tc.column, func(t *testing.T) {
			if err := validateDeleteTarget(tc.table, tc.column); err != nil {
				t.Fatalf("validateDeleteTarget(%q, %q) error = %v", tc.table, tc.column, err)
			}
		})
	}
}

func TestDeleteHelpersRejectInvalidTableOrColumn(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name          string
		table         string
		column        string
		call          func(*Queries) (int64, error)
		expectedError string
	}{
		{
			name:   "deleteByWebsiteNotIn invalid table",
			table:  "sqlite_master",
			column: "name",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteNotIn(ctx, "sqlite_master", "name", 1, []string{"index"})
			},
			expectedError: `invalid table/column: "sqlite_master"/"name"`,
		},
		{
			name:   "deleteByWebsiteNotIn invalid column",
			table:  "pages",
			column: "1=1; DROP TABLE pages; --",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteNotIn(ctx, "pages", "1=1; DROP TABLE pages; --", 1, []string{"index"})
			},
			expectedError: `invalid table/column: "pages"/"1=1; DROP TABLE pages; --"`,
		},
		{
			name:   "deleteByWebsiteSetDifference invalid table",
			table:  "sqlite_master",
			column: "name",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteSetDifference(ctx, "sqlite_master", "name", 1, []string{"index"})
			},
			expectedError: `invalid table/column: "sqlite_master"/"name"`,
		},
		{
			name:   "deleteByWebsiteSetDifference invalid column",
			table:  "pages",
			column: "1=1; DROP TABLE pages; --",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteSetDifference(ctx, "pages", "1=1; DROP TABLE pages; --", 1, []string{"index"})
			},
			expectedError: `invalid table/column: "pages"/"1=1; DROP TABLE pages; --"`,
		},
		{
			name:   "deleteByWebsiteAndKey invalid table",
			table:  "sqlite_master",
			column: "name",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteAndKey(ctx, "sqlite_master", "name", 1, "index")
			},
			expectedError: `invalid table/column: "sqlite_master"/"name"`,
		},
		{
			name:   "deleteByWebsiteAndKey invalid column",
			table:  "pages",
			column: "1=1; DROP TABLE pages; --",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteAndKey(ctx, "pages", "1=1; DROP TABLE pages; --", 1, "index")
			},
			expectedError: `invalid table/column: "pages"/"1=1; DROP TABLE pages; --"`,
		},
		{
			name:   "deleteByWebsiteNotIn empty table and column",
			table:  "",
			column: "",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteNotIn(ctx, "", "", 1, []string{"index"})
			},
			expectedError: `invalid table/column: ""/""`,
		},
		{
			name:   "deleteByWebsiteSetDifference empty column",
			table:  "pages",
			column: "",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteSetDifference(ctx, "pages", "", 1, []string{"index"})
			},
			expectedError: `invalid table/column: "pages"/""`,
		},
		{
			name:   "deleteByWebsiteAndKey empty table",
			table:  "",
			column: "name",
			call: func(q *Queries) (int64, error) {
				return q.deleteByWebsiteAndKey(ctx, "", "name", 1, "index")
			},
			expectedError: `invalid table/column: ""/"name"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &noSQLQueryer{}
			q := NewQueries(spy)
			n, err := tc.call(q)
			if err == nil {
				t.Fatalf("expected error for %q/%q", tc.table, tc.column)
			}
			if err.Error() != tc.expectedError {
				t.Fatalf("error = %q, want %q", err.Error(), tc.expectedError)
			}
			if n != 0 {
				t.Fatalf("deleted rows = %d, want 0", n)
			}
			if spy.execCalls != 0 || spy.queryCalls != 0 {
				t.Fatalf("expected no SQL execution, got execCalls=%d queryCalls=%d", spy.execCalls, spy.queryCalls)
			}
		})
	}
}

func TestDeleteByWebsiteNotInAllowlistedTargets(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		table  string
		column string
		keep   string
		drop   string
	}{
		{table: "pages", column: "name", keep: "index", drop: "about"},
		{table: "components", column: "name", keep: "header", drop: "footer"},
		{table: "style_bundles", column: "name", keep: "default", drop: "landing"},
		{table: "assets", column: "filename", keep: "assets/keep.txt", drop: "assets/drop.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.table+"."+tc.column, func(t *testing.T) {
			q, cleanup := setupDB(t)
			defer cleanup()

			websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
			if err != nil {
				t.Fatalf("InsertWebsite() error = %v", err)
			}
			seedDeleteTargetRows(t, q, ctx, tc.table, websiteID, tc.keep, tc.drop)

			deleted, err := q.deleteByWebsiteNotIn(ctx, tc.table, tc.column, websiteID, []string{tc.keep})
			if err != nil {
				t.Fatalf("deleteByWebsiteNotIn() error = %v", err)
			}
			if deleted != 1 {
				t.Fatalf("deleted rows = %d, want 1", deleted)
			}

			got := listDeleteTargetValues(t, q, ctx, tc.table, websiteID)
			if len(got) != 1 || got[0] != tc.keep {
				t.Fatalf("remaining values = %#v, want [%q]", got, tc.keep)
			}
		})
	}
}

func TestDeleteByWebsiteSetDifferenceAllowlistedTargets(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		table  string
		column string
		keep   string
		drop   string
	}{
		{table: "pages", column: "name", keep: "index", drop: "about"},
		{table: "components", column: "name", keep: "header", drop: "footer"},
		{table: "style_bundles", column: "name", keep: "default", drop: "landing"},
		{table: "assets", column: "filename", keep: "assets/keep.txt", drop: "assets/drop.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.table+"."+tc.column, func(t *testing.T) {
			q, cleanup := setupDB(t)
			defer cleanup()

			websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
			if err != nil {
				t.Fatalf("InsertWebsite() error = %v", err)
			}
			seedDeleteTargetRows(t, q, ctx, tc.table, websiteID, tc.keep, tc.drop)

			deleted, err := q.deleteByWebsiteSetDifference(ctx, tc.table, tc.column, websiteID, []string{tc.keep})
			if err != nil {
				t.Fatalf("deleteByWebsiteSetDifference() error = %v", err)
			}
			if deleted != 1 {
				t.Fatalf("deleted rows = %d, want 1", deleted)
			}

			got := listDeleteTargetValues(t, q, ctx, tc.table, websiteID)
			if len(got) != 1 || got[0] != tc.keep {
				t.Fatalf("remaining values = %#v, want [%q]", got, tc.keep)
			}
		})
	}
}

func TestDeleteByWebsiteAndKeyAllowlistedTargets(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		table  string
		column string
		keep   string
		drop   string
	}{
		{table: "pages", column: "name", keep: "index", drop: "about"},
		{table: "components", column: "name", keep: "header", drop: "footer"},
		{table: "style_bundles", column: "name", keep: "default", drop: "landing"},
		{table: "assets", column: "filename", keep: "assets/keep.txt", drop: "assets/drop.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.table+"."+tc.column, func(t *testing.T) {
			q, cleanup := setupDB(t)
			defer cleanup()

			websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
			if err != nil {
				t.Fatalf("InsertWebsite() error = %v", err)
			}
			seedDeleteTargetRows(t, q, ctx, tc.table, websiteID, tc.keep, tc.drop)

			deleted, err := q.deleteByWebsiteAndKey(ctx, tc.table, tc.column, websiteID, tc.drop)
			if err != nil {
				t.Fatalf("deleteByWebsiteAndKey() error = %v", err)
			}
			if deleted != 1 {
				t.Fatalf("deleted rows = %d, want 1", deleted)
			}

			got := listDeleteTargetValues(t, q, ctx, tc.table, websiteID)
			if len(got) != 1 || got[0] != tc.keep {
				t.Fatalf("remaining values = %#v, want [%q]", got, tc.keep)
			}
		})
	}
}

func TestListReleasesByEnvironmentPage(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	ids := []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA",
		"01ARZ3NDEKTSV4RRFFQ69G5FAB",
		"01ARZ3NDEKTSV4RRFFQ69G5FAC",
	}
	for _, id := range ids {
		if err := q.InsertRelease(ctx, ReleaseRow{
			ID:            id,
			EnvironmentID: envID,
			ManifestJSON:  "{}",
			OutputHashes:  "{}",
			BuildLog:      "",
			Status:        "active",
		}); err != nil {
			t.Fatalf("InsertRelease(%q) error = %v", id, err)
		}
	}

	page, err := q.ListReleasesByEnvironmentPage(ctx, envID, 2, 1)
	if err != nil {
		t.Fatalf("ListReleasesByEnvironmentPage() error = %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(page))
	}
	if page[0].ID != "01ARZ3NDEKTSV4RRFFQ69G5FAB" || page[1].ID != "01ARZ3NDEKTSV4RRFFQ69G5FAA" {
		t.Fatalf("unexpected page order: %#v", page)
	}
}

func TestListLatestReleaseActors(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	releaseA := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	releaseB := "01ARZ3NDEKTSV4RRFFQ69G5FAB"
	for _, id := range []string{releaseA, releaseB} {
		if err := q.InsertRelease(ctx, ReleaseRow{
			ID:            id,
			EnvironmentID: envID,
			ManifestJSON:  "{}",
			OutputHashes:  "{}",
			BuildLog:      "",
			Status:        "active",
		}); err != nil {
			t.Fatalf("InsertRelease(%q) error = %v", id, err)
		}
	}

	if _, err := q.InsertAuditLog(ctx, AuditLogRow{
		Actor:           "alice",
		EnvironmentID:   &envID,
		Operation:       "release.activate",
		ResourceSummary: "activated release A",
		ReleaseID:       &releaseA,
		MetadataJSON:    "{}",
	}); err != nil {
		t.Fatalf("InsertAuditLog(releaseA/alice) error = %v", err)
	}
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{
		Actor:           "bob",
		EnvironmentID:   &envID,
		Operation:       "rollback",
		ResourceSummary: "activated release A via rollback",
		ReleaseID:       &releaseA,
		MetadataJSON:    "{}",
	}); err != nil {
		t.Fatalf("InsertAuditLog(releaseA/bob) error = %v", err)
	}
	if _, err := q.InsertAuditLog(ctx, AuditLogRow{
		Actor:           "ci",
		EnvironmentID:   &envID,
		Operation:       "release.activate",
		ResourceSummary: "activated release B",
		ReleaseID:       &releaseB,
		MetadataJSON:    "{}",
	}); err != nil {
		t.Fatalf("InsertAuditLog(releaseB/ci) error = %v", err)
	}

	actors, err := q.ListLatestReleaseActors(ctx, envID, []string{releaseA, releaseB, "missing"})
	if err != nil {
		t.Fatalf("ListLatestReleaseActors() error = %v", err)
	}
	if actors[releaseA] != "bob" {
		t.Fatalf("expected releaseA actor bob, got %q", actors[releaseA])
	}
	if actors[releaseB] != "ci" {
		t.Fatalf("expected releaseB actor ci, got %q", actors[releaseB])
	}
	if _, ok := actors["missing"]; ok {
		t.Fatalf("did not expect actor for missing release id")
	}
}

func TestDomainBindingCRUD(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	stagingID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	if _, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "prod"}); err != nil {
		t.Fatalf("InsertEnvironment(prod) error = %v", err)
	}

	if _, err := q.InsertDomainBinding(ctx, DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: stagingID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}

	got, err := q.GetDomainBindingByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetDomainBindingByDomain() error = %v", err)
	}
	if got.Domain != "example.com" || got.WebsiteName != "sample" || got.EnvironmentName != "staging" {
		t.Fatalf("unexpected domain binding: %#v", got)
	}

	all, err := q.ListDomainBindings(ctx, "", "")
	if err != nil {
		t.Fatalf("ListDomainBindings(all) error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 domain binding, got %d", len(all))
	}

	websiteFiltered, err := q.ListDomainBindings(ctx, "sample", "")
	if err != nil {
		t.Fatalf("ListDomainBindings(website) error = %v", err)
	}
	if len(websiteFiltered) != 1 {
		t.Fatalf("expected 1 website-filtered domain binding, got %d", len(websiteFiltered))
	}

	envFiltered, err := q.ListDomainBindings(ctx, "", "staging")
	if err != nil {
		t.Fatalf("ListDomainBindings(environment) error = %v", err)
	}
	if len(envFiltered) != 1 {
		t.Fatalf("expected 1 env-filtered domain binding, got %d", len(envFiltered))
	}

	deleted, err := q.DeleteDomainBindingByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("DeleteDomainBindingByDomain() error = %v", err)
	}
	if !deleted {
		t.Fatalf("expected deleted=true")
	}
	deleted, err = q.DeleteDomainBindingByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("DeleteDomainBindingByDomain(second) error = %v", err)
	}
	if deleted {
		t.Fatalf("expected deleted=false for second delete")
	}
}

func TestRestoreDomainBindingPreservesIdentity(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}

	before, err := q.GetDomainBindingByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetDomainBindingByDomain(before) error = %v", err)
	}
	deleted, err := q.DeleteDomainBindingByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("DeleteDomainBindingByDomain() error = %v", err)
	}
	if !deleted {
		t.Fatalf("expected deleted=true")
	}

	if err := q.RestoreDomainBinding(ctx, DomainBindingRow{
		ID:            before.ID,
		Domain:        before.Domain,
		EnvironmentID: before.EnvironmentID,
		CreatedAt:     before.CreatedAt,
		UpdatedAt:     before.UpdatedAt,
	}); err != nil {
		t.Fatalf("RestoreDomainBinding() error = %v", err)
	}

	after, err := q.GetDomainBindingByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetDomainBindingByDomain(after) error = %v", err)
	}
	if after.ID != before.ID {
		t.Fatalf("expected stable id %d, got %d", before.ID, after.ID)
	}
	if after.CreatedAt != before.CreatedAt || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("expected stable timestamps before=(%s,%s) after=(%s,%s)", before.CreatedAt, before.UpdatedAt, after.CreatedAt, after.UpdatedAt)
	}
}

func TestDomainBindingUniqueConstraint(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	if _, err := q.InsertDomainBinding(ctx, DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding(first) error = %v", err)
	}
	if _, err := q.InsertDomainBinding(ctx, DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envID,
	}); err == nil {
		t.Fatalf("expected unique constraint error on duplicate domain")
	}
}

func TestTelemetryEventsInsertListAndFilters(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	idA, err := q.InsertTelemetryEvent(ctx, TelemetryEventRow{
		EnvironmentID: envID,
		EventName:     "page_view",
		Path:          "/",
		AttrsJSON:     `{"source":"home"}`,
	})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(a) error = %v", err)
	}
	idB, err := q.InsertTelemetryEvent(ctx, TelemetryEventRow{
		EnvironmentID: envID,
		EventName:     "cta_click",
		Path:          "/pricing",
		AttrsJSON:     `{"button":"buy"}`,
	})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(b) error = %v", err)
	}
	idC, err := q.InsertTelemetryEvent(ctx, TelemetryEventRow{
		EnvironmentID: envID,
		EventName:     "page_view",
		Path:          "/docs",
		AttrsJSON:     `{"source":"docs"}`,
	})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(c) error = %v", err)
	}

	if _, err := q.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2026-02-20T10:00:00Z", idA); err != nil {
		t.Fatalf("update telemetry received_at(a): %v", err)
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2026-02-21T10:00:00Z", idB); err != nil {
		t.Fatalf("update telemetry received_at(b): %v", err)
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2026-02-22T10:00:00Z", idC); err != nil {
		t.Fatalf("update telemetry received_at(c): %v", err)
	}

	all, err := q.ListTelemetryEvents(ctx, ListTelemetryEventsParams{
		EnvironmentID: envID,
		Limit:         10,
		Offset:        0,
	})
	if err != nil {
		t.Fatalf("ListTelemetryEvents(all) error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 telemetry rows, got %d", len(all))
	}
	if all[0].ID != idC || all[1].ID != idB || all[2].ID != idA {
		t.Fatalf("unexpected telemetry order: got ids [%d,%d,%d], want [%d,%d,%d]", all[0].ID, all[1].ID, all[2].ID, idC, idB, idA)
	}

	since := "2026-02-21T00:00:00Z"
	until := "2026-02-22T00:00:00Z"
	filtered, err := q.ListTelemetryEvents(ctx, ListTelemetryEventsParams{
		EnvironmentID: envID,
		EventName:     "cta_click",
		Since:         &since,
		Until:         &until,
		Limit:         10,
		Offset:        0,
	})
	if err != nil {
		t.Fatalf("ListTelemetryEvents(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != idB {
		t.Fatalf("expected filtered telemetry id %d, got %#v", idB, filtered)
	}
}

func TestDeleteTelemetryEventsOlderThanDays(t *testing.T) {
	q, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	websiteID, err := q.InsertWebsite(ctx, WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	oldID, err := q.InsertTelemetryEvent(ctx, TelemetryEventRow{
		EnvironmentID: envID,
		EventName:     "page_view",
		Path:          "/old",
		AttrsJSON:     `{}`,
	})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(old) error = %v", err)
	}
	newID, err := q.InsertTelemetryEvent(ctx, TelemetryEventRow{
		EnvironmentID: envID,
		EventName:     "page_view",
		Path:          "/new",
		AttrsJSON:     `{}`,
	})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(new) error = %v", err)
	}

	if _, err := q.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", oldID); err != nil {
		t.Fatalf("update telemetry old received_at: %v", err)
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2099-01-01T00:00:00Z", newID); err != nil {
		t.Fatalf("update telemetry new received_at: %v", err)
	}

	deleted, err := q.DeleteTelemetryEventsOlderThanDays(ctx, 30)
	if err != nil {
		t.Fatalf("DeleteTelemetryEventsOlderThanDays() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted telemetry row, got %d", deleted)
	}

	rows, err := q.ListTelemetryEvents(ctx, ListTelemetryEventsParams{
		EnvironmentID: envID,
		Limit:         100,
		Offset:        0,
	})
	if err != nil {
		t.Fatalf("ListTelemetryEvents() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != newID {
		t.Fatalf("expected only new telemetry row %d to remain, got %#v", newID, rows)
	}

	deleted, err = q.DeleteTelemetryEventsOlderThanDays(ctx, 0)
	if err != nil {
		t.Fatalf("DeleteTelemetryEventsOlderThanDays(0) error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected no-op delete count for retentionDays=0, got %d", deleted)
	}
}

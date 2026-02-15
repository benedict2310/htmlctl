package release

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/blob"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestBuilderBuildSuccess(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))

	websiteID, envID := seedReleaseState(t, ctx, queries, blobStore)
	_ = websiteID

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	res, err := builder.Build(ctx, "futurelab", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if res.ReleaseID == "" {
		t.Fatalf("expected release id")
	}
	if len(res.OutputHashes) == 0 {
		t.Fatalf("expected non-empty output hashes")
	}

	releaseDir := filepath.Join(dataDir, "websites", "futurelab", "envs", "staging", "releases", res.ReleaseID)
	for _, rel := range []string{"index.html", "styles/tokens.css", "styles/default.css", "assets/logo.svg", ".manifest.json", ".build-log.txt", ".output-hashes.json"} {
		if _, err := os.Stat(filepath.Join(releaseDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected release file %s to exist: %v", rel, err)
		}
	}

	currentTarget, err := os.Readlink(filepath.Join(dataDir, "websites", "futurelab", "envs", "staging", "current"))
	if err != nil {
		t.Fatalf("Readlink(current) error = %v", err)
	}
	if currentTarget != filepath.ToSlash(filepath.Join("releases", res.ReleaseID)) {
		t.Fatalf("unexpected current symlink target %q", currentTarget)
	}

	releaseRow, err := queries.GetReleaseByID(ctx, res.ReleaseID)
	if err != nil {
		t.Fatalf("GetReleaseByID() error = %v", err)
	}
	if releaseRow.Status != "active" {
		t.Fatalf("expected active release status, got %q", releaseRow.Status)
	}
	envRow, err := queries.GetEnvironmentByName(ctx, websiteID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if envRow.ActiveReleaseID == nil || *envRow.ActiveReleaseID != res.ReleaseID {
		t.Fatalf("expected active release id %q, got %#v", res.ReleaseID, envRow.ActiveReleaseID)
	}

	if rows, err := queries.ListReleasesByEnvironment(ctx, envID); err != nil {
		t.Fatalf("ListReleasesByEnvironment() error = %v", err)
	} else if len(rows) != 1 {
		t.Fatalf("expected one release row, got %d", len(rows))
	}
}

func TestBuilderBuildFailureRecordsFailedRelease(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))

	websiteID, envID := seedReleaseState(t, ctx, queries, blobStore)

	rows, err := queries.ListComponentsByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListComponentsByWebsite() error = %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected seeded component")
	}
	componentHashHex := strings.TrimPrefix(rows[0].ContentHash, "sha256:")
	if err := os.Remove(blobStore.Path(componentHashHex)); err != nil {
		t.Fatalf("remove component blob: %v", err)
	}

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	if _, err := builder.Build(ctx, "futurelab", "staging"); err == nil {
		t.Fatalf("expected build failure")
	}

	releases, err := queries.ListReleasesByEnvironment(ctx, envID)
	if err != nil {
		t.Fatalf("ListReleasesByEnvironment() error = %v", err)
	}
	if len(releases) == 0 {
		t.Fatalf("expected failed release row")
	}
	if releases[0].Status != "failed" {
		t.Fatalf("expected latest release status failed, got %q", releases[0].Status)
	}
}

func openReleaseTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return db
}

func seedReleaseState(t *testing.T, ctx context.Context, q *dbpkg.Queries, blobs *blob.Store) (websiteID int64, envID int64) {
	t.Helper()
	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err = q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	component := []byte("<section id=\"header\">Header</section>\n")
	pageYAML := []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  description: Home\n  layout:\n    - include: header\n")
	tokens := []byte(":root { --brand: blue; }\n")
	defaults := []byte("body { margin: 0; }\n")
	asset := []byte("<svg></svg>")

	componentHash := writeBlob(t, ctx, blobs, component)
	pageHash := writeBlob(t, ctx, blobs, pageYAML)
	tokensHash := writeBlob(t, ctx, blobs, tokens)
	defaultsHash := writeBlob(t, ctx, blobs, defaults)
	assetHash := writeBlob(t, ctx, blobs, asset)

	if err := q.UpsertComponent(ctx, dbpkg.ComponentRow{WebsiteID: websiteID, Name: "header", Scope: "global", ContentHash: componentHash}); err != nil {
		t.Fatalf("UpsertComponent() error = %v", err)
	}
	layoutJSON, _ := json.Marshal([]map[string]string{{"include": "header"}})
	if err := q.UpsertPage(ctx, dbpkg.PageRow{WebsiteID: websiteID, Name: "index", Route: "/", Title: "Home", Description: "Home", LayoutJSON: string(layoutJSON), ContentHash: pageHash}); err != nil {
		t.Fatalf("UpsertPage() error = %v", err)
	}
	styleFilesJSON, _ := json.Marshal([]map[string]string{{"file": "styles/tokens.css", "hash": tokensHash}, {"file": "styles/default.css", "hash": defaultsHash}})
	if err := q.UpsertStyleBundle(ctx, dbpkg.StyleBundleRow{WebsiteID: websiteID, Name: "default", FilesJSON: string(styleFilesJSON)}); err != nil {
		t.Fatalf("UpsertStyleBundle() error = %v", err)
	}
	if err := q.UpsertAsset(ctx, dbpkg.AssetRow{WebsiteID: websiteID, Filename: "assets/logo.svg", ContentType: "image/svg+xml", SizeBytes: int64(len(asset)), ContentHash: assetHash}); err != nil {
		t.Fatalf("UpsertAsset() error = %v", err)
	}

	return websiteID, envID
}

func writeBlob(t *testing.T, ctx context.Context, store *blob.Store, content []byte) string {
	t.Helper()
	sum := sha256.Sum256(content)
	hashHex := hex.EncodeToString(sum[:])
	if _, err := store.Put(ctx, hashHex, content); err != nil {
		t.Fatalf("Put(%s) error = %v", hashHex, err)
	}
	return "sha256:" + hashHex
}

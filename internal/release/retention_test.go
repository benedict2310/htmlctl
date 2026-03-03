package release

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/benedict2310/htmlctl/internal/blob"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestRunRetentionDryRunPreservesActiveRollbackAndPreviewPins(t *testing.T) {
	fix := setupRetentionFixture(t)
	defer fix.cleanup()

	envID := seedRetentionEnvironment(t, fix.ctx, fix.db, fix.q, fix.websitesRoot, "sample", "staging", []retentionSeedRelease{
		{ID: "05", Status: "active"},
		{ID: "04", Status: "failed"},
		{ID: "03", Status: "active"},
		{ID: "02", Status: "active"},
		{ID: "01", Status: "active"},
	}, "05")
	if _, err := fix.q.InsertReleasePreview(fix.ctx, dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "01",
		Hostname:      "pin--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     time.Now().UTC().Add(24 * time.Hour).Format(retentionPreviewTimestampLayout),
	}); err != nil {
		t.Fatalf("InsertReleasePreview() error = %v", err)
	}

	result, err := RunRetention(fix.ctx, fix.db, nil, fix.websitesRoot, "sample", "staging", RetentionOptions{
		Keep:   1,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("RunRetention() error = %v", err)
	}

	if got, want := strings.Join(result.RetainedReleaseIDs, ","), "05,03,01"; got != want {
		t.Fatalf("unexpected retained release ids: got %q want %q", got, want)
	}
	if got, want := strings.Join(result.PrunableReleaseIDs, ","), "04,02"; got != want {
		t.Fatalf("unexpected prunable release ids: got %q want %q", got, want)
	}
	if got, want := strings.Join(result.PreviewPinnedReleaseIDs, ","), "01"; got != want {
		t.Fatalf("unexpected preview pinned ids: got %q want %q", got, want)
	}
	if len(result.PrunedReleaseIDs) != 0 {
		t.Fatalf("did not expect pruned release ids in dry run: %#v", result.PrunedReleaseIDs)
	}
	for _, releaseID := range []string{"05", "04", "03", "02", "01"} {
		if _, err := os.Stat(filepath.Join(fix.websitesRoot, "sample", "envs", "staging", "releases", releaseID)); err != nil {
			t.Fatalf("expected release directory %s to remain during dry run: %v", releaseID, err)
		}
	}
}

func TestRunRetentionRestoresQuarantineWhenDeleteFails(t *testing.T) {
	fix := setupRetentionFixture(t)
	defer fix.cleanup()

	seedRetentionEnvironment(t, fix.ctx, fix.db, fix.q, fix.websitesRoot, "sample", "staging", []retentionSeedRelease{
		{ID: "03", Status: "active"},
		{ID: "02", Status: "active"},
		{ID: "01", Status: "active"},
	}, "03")
	prevDeleteFn := deleteReleasesByEnvironment
	deleteReleasesByEnvironment = func(ctx context.Context, q *dbpkg.Queries, environmentID int64, releaseIDs []string) (int64, error) {
		return 0, fmt.Errorf("forced delete failure")
	}
	t.Cleanup(func() {
		deleteReleasesByEnvironment = prevDeleteFn
	})

	_, err := RunRetention(fix.ctx, fix.db, nil, fix.websitesRoot, "sample", "staging", RetentionOptions{
		Keep: 1,
	})
	if err == nil {
		t.Fatalf("expected retention delete failure")
	}
	if !strings.Contains(err.Error(), "delete release rows") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fix.websitesRoot, "sample", "envs", "staging", "releases", "01")); err != nil {
		t.Fatalf("expected release directory restoration after delete failure: %v", err)
	}
	if _, err := fix.q.GetReleaseByID(fix.ctx, "01"); err != nil {
		t.Fatalf("expected release row restoration after delete failure: %v", err)
	}
}

func TestRunRetentionDeletesOldReleasesAndOrphanBlobs(t *testing.T) {
	fix := setupRetentionFixture(t)
	defer fix.cleanup()

	envID := seedRetentionEnvironment(t, fix.ctx, fix.db, fix.q, fix.websitesRoot, "sample", "staging", []retentionSeedRelease{
		{ID: "03", Status: "active"},
		{ID: "02", Status: "active"},
		{ID: "01", Status: "active"},
	}, "03")
	seedRetentionDesiredState(t, fix.ctx, fix.q, fix.blobStore)
	orphanHash := retentionHashHex("orphan")
	dirHash := retentionHashHex("dir-orphan")
	if err := os.MkdirAll(fix.blobStore.Root(), 0o755); err != nil {
		t.Fatalf("MkdirAll(blob root) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fix.blobStore.Root(), orphanHash), []byte("orphan"), 0o644); err != nil {
		t.Fatalf("WriteFile(orphan blob) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fix.blobStore.Root(), "notes.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(non-hash blob note) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fix.blobStore.Root(), dirHash), 0o755); err != nil {
		t.Fatalf("MkdirAll(hash-named dir) error = %v", err)
	}

	result, err := RunRetention(fix.ctx, fix.db, fix.blobStore, fix.websitesRoot, "sample", "staging", RetentionOptions{
		Keep:   1,
		BlobGC: true,
	})
	if err != nil {
		t.Fatalf("RunRetention() error = %v", err)
	}
	if got, want := strings.Join(result.PrunedReleaseIDs, ","), "01"; got != want {
		t.Fatalf("unexpected pruned release ids: got %q want %q", got, want)
	}
	if got, want := strings.Join(result.DeletedBlobHashes, ","), orphanHash; got != want {
		t.Fatalf("unexpected deleted blob hashes: got %q want %q", got, want)
	}
	if result.MarkedBlobCount != 7 {
		t.Fatalf("expected 7 marked blob hashes, got %d", result.MarkedBlobCount)
	}
	if _, err := os.Stat(filepath.Join(fix.websitesRoot, "sample", "envs", "staging", "releases", "01")); !os.IsNotExist(err) {
		t.Fatalf("expected pruned release directory to be removed, err=%v", err)
	}
	releases, err := fix.q.ListReleasesByEnvironment(fix.ctx, envID)
	if err != nil {
		t.Fatalf("ListReleasesByEnvironment() error = %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 remaining release rows, got %d", len(releases))
	}
	if _, err := os.Stat(filepath.Join(fix.blobStore.Root(), orphanHash)); !os.IsNotExist(err) {
		t.Fatalf("expected orphan blob deletion, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(fix.blobStore.Root(), "notes.txt")); err != nil {
		t.Fatalf("expected non-hash blob file to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fix.blobStore.Root(), dirHash)); err != nil {
		t.Fatalf("expected hash-named directory to remain untouched: %v", err)
	}
	for _, hashHex := range result.BlobDeleteCandidates {
		if hashHex == dirHash {
			t.Fatalf("did not expect hash-named directory in blob delete candidates")
		}
	}

	rollback, err := Rollback(fix.ctx, fix.db, fix.websitesRoot, "sample", "staging")
	if err != nil {
		t.Fatalf("Rollback() after retention error = %v", err)
	}
	if rollback.ToReleaseID != "02" {
		t.Fatalf("unexpected rollback target after retention: %#v", rollback)
	}
}

type retentionFixture struct {
	ctx          context.Context
	db           *sql.DB
	q            *dbpkg.Queries
	blobStore    *blob.Store
	websitesRoot string
	cleanup      func()
}

type retentionSeedRelease struct {
	ID     string
	Status string
}

func setupRetentionFixture(t *testing.T) retentionFixture {
	t.Helper()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "db.sqlite")
	db, err := dbpkg.Open(dbpkg.DefaultOptions(dbPath))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return retentionFixture{
		ctx:          context.Background(),
		db:           db,
		q:            dbpkg.NewQueries(db),
		blobStore:    blob.NewStore(filepath.Join(dataDir, "blobs", "sha256")),
		websitesRoot: filepath.Join(dataDir, "websites"),
		cleanup: func() {
			_ = db.Close()
		},
	}
}

func seedRetentionEnvironment(t *testing.T, ctx context.Context, db *sql.DB, q *dbpkg.Queries, websitesRoot, website, env string, releases []retentionSeedRelease, activeReleaseID string) int64 {
	t.Helper()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: website, DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: env})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	baseTime := time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	for i, release := range releases {
		if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
			ID:            release.ID,
			EnvironmentID: envID,
			ManifestJSON:  "{}",
			OutputHashes:  "{}",
			BuildLog:      "",
			Status:        release.Status,
		}); err != nil {
			t.Fatalf("InsertRelease(%q) error = %v", release.ID, err)
		}
		releaseDir := filepath.Join(websitesRoot, website, "envs", env, "releases", release.ID)
		if err := os.MkdirAll(releaseDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", releaseDir, err)
		}
		createdAt := baseTime.Add(-time.Duration(i) * time.Minute).Format(retentionPreviewTimestampLayout)
		if _, err := db.ExecContext(ctx, `UPDATE releases SET created_at = ? WHERE id = ?`, createdAt, release.ID); err != nil {
			t.Fatalf("UPDATE releases.created_at(%s) error = %v", release.ID, err)
		}
	}
	if err := q.UpdateEnvironmentActiveRelease(ctx, envID, &activeReleaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease() error = %v", err)
	}
	envDir := filepath.Join(websitesRoot, website, "envs", env)
	if err := SwitchCurrentSymlink(envDir, activeReleaseID); err != nil {
		t.Fatalf("SwitchCurrentSymlink(%q) error = %v", activeReleaseID, err)
	}
	return envID
}

func seedRetentionDesiredState(t *testing.T, ctx context.Context, q *dbpkg.Queries, blobs *blob.Store) {
	t.Helper()

	websiteRow, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	websiteID := websiteRow.ID
	if err := q.UpdateWebsiteSpec(ctx, dbpkg.WebsiteRow{
		ID:                 websiteID,
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
		ContentHash:        retentionHashRef("website"),
	}); err != nil {
		t.Fatalf("UpdateWebsiteSpec() error = %v", err)
	}
	if _, err := q.InsertPage(ctx, dbpkg.PageRow{
		WebsiteID:   websiteID,
		Name:        "index",
		Route:       "/",
		Title:       "Home",
		Description: "Home",
		LayoutJSON:  `[]`,
		ContentHash: retentionHashRef("page"),
	}); err != nil {
		t.Fatalf("InsertPage() error = %v", err)
	}
	if _, err := q.InsertComponent(ctx, dbpkg.ComponentRow{
		WebsiteID:   websiteID,
		Name:        "header",
		Scope:       "global",
		ContentHash: retentionHashRef("component"),
	}); err != nil {
		t.Fatalf("InsertComponent() error = %v", err)
	}
	if _, err := q.InsertStyleBundle(ctx, dbpkg.StyleBundleRow{
		WebsiteID: websiteID,
		Name:      "default",
		FilesJSON: fmt.Sprintf(`[{"file":"styles/tokens.css","hash":"%s"},{"file":"styles/default.css","hash":"%s"}]`, retentionHashRef("tokens"), retentionHashRef("default")),
	}); err != nil {
		t.Fatalf("InsertStyleBundle() error = %v", err)
	}
	if _, err := q.InsertAsset(ctx, dbpkg.AssetRow{
		WebsiteID:   websiteID,
		Filename:    "logo.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   8,
		ContentHash: retentionHashRef("asset"),
	}); err != nil {
		t.Fatalf("InsertAsset() error = %v", err)
	}
	if err := q.UpsertWebsiteIcon(ctx, dbpkg.WebsiteIconRow{
		WebsiteID:   websiteID,
		Slot:        "svg",
		SourcePath:  "branding/favicon.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   4,
		ContentHash: retentionHashRef("icon"),
	}); err != nil {
		t.Fatalf("UpsertWebsiteIcon() error = %v", err)
	}
	for _, label := range []string{"website", "page", "component", "tokens", "default", "asset", "icon"} {
		hashHex := retentionHashHex(label)
		if err := os.MkdirAll(blobs.Root(), 0o755); err != nil {
			t.Fatalf("MkdirAll(blob root) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(blobs.Root(), hashHex), []byte(label), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", hashHex, err)
		}
	}
}

func retentionHashRef(label string) string {
	return "sha256:" + retentionHashHex(label)
}

func retentionHashHex(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

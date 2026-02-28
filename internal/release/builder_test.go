package release

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/blob"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/internal/ogimage"
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

	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if res.ReleaseID == "" {
		t.Fatalf("expected release id")
	}
	if len(res.OutputHashes) == 0 {
		t.Fatalf("expected non-empty output hashes")
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	for _, rel := range []string{"index.html", "styles/tokens.css", "styles/default.css", "assets/logo.svg", "og/index.png", ".manifest.json", ".build-log.txt", ".output-hashes.json"} {
		if _, err := os.Stat(filepath.Join(releaseDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected release file %s to exist: %v", rel, err)
		}
	}
	indexHTML, err := os.ReadFile(filepath.Join(releaseDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	indexText := string(indexHTML)
	for _, needle := range []string{
		`<link rel="canonical" href="https://example.com/">`,
		`<meta property="og:title" content="Sample Home">`,
		`<meta property="og:image" content="https://example.com/og/index.png">`,
		`<meta property="twitter:card" content="summary_large_image">`,
		`<meta property="twitter:image" content="https://example.com/og/index.png">`,
		`<script type="application/ld+json">`,
	} {
		if !strings.Contains(indexText, needle) {
			t.Fatalf("expected index.html to contain %q", needle)
		}
	}

	currentTarget, err := os.Readlink(filepath.Join(dataDir, "websites", "sample", "envs", "staging", "current"))
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
	if _, err := builder.Build(ctx, "sample", "staging"); err == nil {
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

func TestBuilderBuildRejectsInvalidStoredResourceNames(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		updateQuery  string
		updateName   string
		expectSubstr string
	}{
		{
			name:         "invalid component name",
			updateQuery:  `UPDATE components SET name = ?`,
			updateName:   "../evil",
			expectSubstr: "invalid component name",
		},
		{
			name:         "invalid page name",
			updateQuery:  `UPDATE pages SET name = ?`,
			updateName:   "index\nadmin",
			expectSubstr: "invalid page name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
			defer db.Close()
			queries := dbpkg.NewQueries(db)
			blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))

			seedReleaseState(t, ctx, queries, blobStore)
			if _, err := db.Exec(tc.updateQuery, tc.updateName); err != nil {
				t.Fatalf("inject invalid name: %v", err)
			}

			builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err != nil {
				t.Fatalf("NewBuilder() error = %v", err)
			}

			_, err = builder.Build(ctx, "sample", "staging")
			if err == nil {
				t.Fatalf("expected build failure")
			}
			if !strings.Contains(err.Error(), tc.expectSubstr) {
				t.Fatalf("expected error containing %q, got %v", tc.expectSubstr, err)
			}
		})
	}
}

func TestBuildOGImageGenerated(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); err != nil {
		t.Fatalf("expected generated og image: %v", err)
	}
	indexHTML, err := os.ReadFile(filepath.Join(releaseDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	text := string(indexHTML)
	if !strings.Contains(text, `property="og:image" content="https://example.com/og/index.png"`) {
		t.Fatalf("expected auto-injected og:image tag")
	}
	if !strings.Contains(text, `property="twitter:image" content="https://example.com/og/index.png"`) {
		t.Fatalf("expected auto-injected twitter:image tag")
	}
}

func TestBuildOGImageCacheHit(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	generateCalls := 0
	builder.generateFn = func(card ogimage.Card) ([]byte, error) {
		generateCalls++
		return ogimage.Generate(card)
	}

	first, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build(first) error = %v", err)
	}
	second, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build(second) error = %v", err)
	}
	if generateCalls != 1 {
		t.Fatalf("expected one Generate call across two builds (cache hit on second), got %d", generateCalls)
	}
	for _, releaseID := range []string{first.ReleaseID, second.ReleaseID} {
		releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", releaseID)
		if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); err != nil {
			t.Fatalf("expected og image for release %s: %v", releaseID, err)
		}
	}
}

func TestBuildOGImageNoInjectionWithoutAbsoluteCanonical(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)
	setPageHeadJSON(t, db, `{"canonicalURL":"/","openGraph":{"title":"Sample Home"},"twitter":{"card":"summary_large_image"}}`)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); err != nil {
		t.Fatalf("expected og image file to still be materialized: %v", err)
	}
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if strings.Contains(indexText, "property=\"og:image\"") {
		t.Fatalf("did not expect og:image injection for relative canonical URL")
	}
	if strings.Contains(indexText, "property=\"twitter:image\"") {
		t.Fatalf("did not expect twitter:image injection for relative canonical URL")
	}
}

func TestBuildOGImagePreservesExplicitOpenGraphImage(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)
	setPageHeadJSON(t, db, `{"canonicalURL":"https://example.com/","openGraph":{"title":"Sample Home","image":"https://cdn.example.com/manual-og.png"},"twitter":{"card":"summary_large_image"}}`)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if !strings.Contains(indexText, `property="og:image" content="https://cdn.example.com/manual-og.png"`) {
		t.Fatalf("expected explicit openGraph.image to be preserved")
	}
	if !strings.Contains(indexText, `property="twitter:image" content="https://example.com/og/index.png"`) {
		t.Fatalf("expected twitter.image to be auto-injected")
	}
}

func TestBuildOGImagePreservesExplicitTwitterImage(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)
	setPageHeadJSON(t, db, `{"canonicalURL":"https://example.com/","openGraph":{"title":"Sample Home"},"twitter":{"card":"summary_large_image","image":"https://cdn.example.com/manual-twitter.png"}}`)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if !strings.Contains(indexText, `property="og:image" content="https://example.com/og/index.png"`) {
		t.Fatalf("expected og:image to be auto-injected")
	}
	if !strings.Contains(indexText, `property="twitter:image" content="https://cdn.example.com/manual-twitter.png"`) {
		t.Fatalf("expected explicit twitter.image to be preserved")
	}
}

func TestBuildOGImageWarnsAndContinuesOnGenerationError(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	builder.generateFn = func(card ogimage.Card) ([]byte, error) {
		return nil, errors.New("forced generation failure")
	}

	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(res.BuildLog, "warning: og image generation failed page=index") {
		t.Fatalf("expected warning in build log, got:\n%s", res.BuildLog)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); !os.IsNotExist(err) {
		t.Fatalf("expected og image to be absent on generation failure, err=%v", err)
	}
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if strings.Contains(indexText, "property=\"og:image\"") || strings.Contains(indexText, "property=\"twitter:image\"") {
		t.Fatalf("expected no auto-injected image tags when generation fails")
	}
}

func TestBuildOGImageWarnsAndContinuesOnPutError(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)

	if err := os.Chmod(blobStore.Root(), 0o555); err != nil {
		t.Fatalf("Chmod(blob root) error = %v", err)
	}
	defer func() { _ = os.Chmod(blobStore.Root(), 0o755) }()

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(res.BuildLog, "warning: og image generation failed page=index: put blob") {
		t.Fatalf("expected put warning in build log, got:\n%s", res.BuildLog)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); !os.IsNotExist(err) {
		t.Fatalf("expected og image to be absent on put failure, err=%v", err)
	}
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if strings.Contains(indexText, "property=\"og:image\"") || strings.Contains(indexText, "property=\"twitter:image\"") {
		t.Fatalf("expected no auto-injected image tags on put failure")
	}
}

func TestBuildOGImageFallsBackToCopyWhenHardlinkFails(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	builder.linkFileFn = func(oldname, newname string) error {
		return errors.New("forced link failure")
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); err != nil {
		t.Fatalf("expected og image with copy fallback: %v", err)
	}
	if strings.Contains(res.BuildLog, "warning: og image materialization failed") {
		t.Fatalf("did not expect materialization warning when copy fallback succeeds:\n%s", res.BuildLog)
	}
}

func TestBuildOGImageWarnsAndContinuesOnMaterializeFailure(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	builder.linkFileFn = func(oldname, newname string) error {
		return errors.New("forced link failure")
	}
	builder.copyFileFn = func(sourcePath, targetPath string) error {
		return errors.New("forced copy failure")
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(res.BuildLog, "warning: og image materialization failed page=index") {
		t.Fatalf("expected materialization warning in build log, got:\n%s", res.BuildLog)
	}

	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); !os.IsNotExist(err) {
		t.Fatalf("expected og image to be absent on materialization failure, err=%v", err)
	}
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if strings.Contains(indexText, "property=\"og:image\"") || strings.Contains(indexText, "property=\"twitter:image\"") {
		t.Fatalf("expected no auto-injected image tags on materialization failure")
	}
}

func TestBuildOGImageInfoLoggedWhenInjectionSkipped(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)
	// Page has explicit openGraph.image and twitter.image — both should be preserved with info log.
	setPageHeadJSON(t, db, `{"canonicalURL":"https://example.com/","openGraph":{"title":"Sample","image":"https://cdn.example.com/og.png"},"twitter":{"card":"summary_large_image","image":"https://cdn.example.com/tw.png"}}`)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if !strings.Contains(res.BuildLog, `info: page=index og card not injected into openGraph.image`) {
		t.Fatalf("expected info log for openGraph.image skip, got:\n%s", res.BuildLog)
	}
	if !strings.Contains(res.BuildLog, `info: page=index og card not injected into twitter.image`) {
		t.Fatalf("expected info log for twitter.image skip, got:\n%s", res.BuildLog)
	}

	// OG file must still be materialized in the release (card is generated, just not linked).
	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); err != nil {
		t.Fatalf("expected og/index.png to be materialized even when injection is skipped: %v", err)
	}

	// Explicit images must still appear in rendered output.
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if !strings.Contains(indexText, `content="https://cdn.example.com/og.png"`) {
		t.Fatalf("expected explicit openGraph.image preserved in HTML")
	}
	if !strings.Contains(indexText, `content="https://cdn.example.com/tw.png"`) {
		t.Fatalf("expected explicit twitter.image preserved in HTML")
	}
}

func TestBuildOGImageSkipsInjectionForLocalhostCanonicalURL(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)
	// No explicit image fields — injection would normally auto-populate them.
	// But canonicalURL is localhost, so the derived OG URL would be localhost too.
	setPageHeadJSON(t, db, `{"canonicalURL":"http://127.0.0.1:18080/","openGraph":{"title":"Sample Home"},"twitter":{"card":"summary_large_image"}}`)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() must succeed despite localhost canonicalURL: %v", err)
	}

	// Warning about localhost canonicalURL must appear in build log.
	if !strings.Contains(res.BuildLog, "warning: page=index field=canonicalURL contains local host URL") {
		t.Fatalf("expected localhost warning in build log, got:\n%s", res.BuildLog)
	}

	// OG file is still materialized (generated + placed) even though URL is not injected.
	releaseDir := filepath.Join(dataDir, "websites", "sample", "envs", "staging", "releases", res.ReleaseID)
	if _, err := os.Stat(filepath.Join(releaseDir, "og", "index.png")); err != nil {
		t.Fatalf("expected og/index.png to be materialized even for localhost canonical: %v", err)
	}

	// No og:image or twitter:image tags must be injected — the localhost-derived
	// OG URL must be suppressed. The canonical link tag itself still renders (we
	// warn about it but do not strip it from the page spec).
	indexText := readReleaseFileString(t, releaseDir, "index.html")
	if strings.Contains(indexText, `property="og:image"`) {
		t.Fatalf("expected no og:image injection for localhost canonical URL, got:\n%s", indexText)
	}
	if strings.Contains(indexText, `property="twitter:image"`) {
		t.Fatalf("expected no twitter:image injection for localhost canonical URL, got:\n%s", indexText)
	}
}

func TestBuildWarnLocalHostMetadataURL(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	queries := dbpkg.NewQueries(db)
	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	seedReleaseState(t, ctx, queries, blobStore)
	// Simulate a page left over from local Docker dev with localhost URLs.
	setPageHeadJSON(t, db, `{"canonicalURL":"http://127.0.0.1:18080/","openGraph":{"url":"http://127.0.0.1:18080/","image":"http://127.0.0.1:18080/assets/og.png"},"twitter":{"card":"summary_large_image","url":"http://127.0.0.1:18080/","image":"http://127.0.0.1:18080/assets/og.png"}}`)

	builder, err := NewBuilder(db, blobStore, filepath.Join(dataDir, "websites"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	res, err := builder.Build(ctx, "sample", "staging")
	if err != nil {
		t.Fatalf("Build() must succeed despite localhost URLs, got: %v", err)
	}

	for _, field := range []string{"canonicalURL", "openGraph.url", "openGraph.image", "twitter.url", "twitter.image"} {
		needle := "warning: page=index field=" + field + " contains local host URL"
		if !strings.Contains(res.BuildLog, needle) {
			t.Fatalf("expected warning for field %s in build log, got:\n%s", field, res.BuildLog)
		}
	}
}

func setPageHeadJSON(t *testing.T, db *sql.DB, headJSON string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE pages SET head_json = ? WHERE name = ?`, headJSON, "index"); err != nil {
		t.Fatalf("UPDATE pages head_json error = %v", err)
	}
}

func readReleaseFileString(t *testing.T, releaseDir, relPath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(releaseDir, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return string(content)
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
	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err = q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	component := []byte("<section id=\"header\">Header</section>\n")
	tokens := []byte(":root { --brand: blue; }\n")
	defaults := []byte("body { margin: 0; }\n")
	asset := []byte("<svg></svg>")

	componentHash := writeBlob(t, ctx, blobs, component)
	pageSum := sha256.Sum256([]byte("sample:index:/"))
	pageHash := "sha256:" + hex.EncodeToString(pageSum[:])
	tokensHash := writeBlob(t, ctx, blobs, tokens)
	defaultsHash := writeBlob(t, ctx, blobs, defaults)
	assetHash := writeBlob(t, ctx, blobs, asset)

	if err := q.UpsertComponent(ctx, dbpkg.ComponentRow{WebsiteID: websiteID, Name: "header", Scope: "global", ContentHash: componentHash}); err != nil {
		t.Fatalf("UpsertComponent() error = %v", err)
	}
	layoutJSON, _ := json.Marshal([]map[string]string{{"include": "header"}})
	headJSON := `{"canonicalURL":"https://example.com/","meta":{"robots":"index,follow"},"openGraph":{"title":"Sample Home","url":"https://example.com/"},"twitter":{"card":"summary_large_image","title":"Sample Home"},"jsonLD":[{"id":"website","payload":{"@context":"https://schema.org","@type":"WebSite","name":"Sample"}}]}`
	if err := q.UpsertPage(ctx, dbpkg.PageRow{WebsiteID: websiteID, Name: "index", Route: "/", Title: "Home", Description: "Home", LayoutJSON: string(layoutJSON), HeadJSON: headJSON, ContentHash: pageHash}); err != nil {
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

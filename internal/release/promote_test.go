package release

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestPromoteSuccess(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	sourceDir := fix.seedSourceRelease(t, sourceReleaseID, "")

	result, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if result.SourceReleaseID != sourceReleaseID {
		t.Fatalf("expected source release id %q, got %#v", sourceReleaseID, result)
	}
	if result.ReleaseID == "" {
		t.Fatalf("expected non-empty target release id")
	}
	if result.FileCount != 3 {
		t.Fatalf("expected 3 promoted content files, got %d", result.FileCount)
	}

	targetDir := filepath.Join(fix.websitesRoot, "futurelab", "envs", "prod", "releases", result.ReleaseID)
	for _, rel := range []string{
		"index.html",
		"styles/default.css",
		"assets/logo.svg",
		".manifest.json",
		".build-log.txt",
		".output-hashes.json",
	} {
		if _, err := os.Stat(filepath.Join(targetDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected target release file %s to exist: %v", rel, err)
		}
	}

	sourceHashes, err := computePromotionHashes(sourceDir)
	if err != nil {
		t.Fatalf("computePromotionHashes(source) error = %v", err)
	}
	targetHashes, err := computePromotionHashes(targetDir)
	if err != nil {
		t.Fatalf("computePromotionHashes(target) error = %v", err)
	}
	if mismatch := comparePromotionHashes(sourceHashes, targetHashes); mismatch != "" {
		t.Fatalf("expected promoted content hashes to match source; mismatch=%s", mismatch)
	}

	targetEnvRow, err := fix.q.GetEnvironmentByName(ctx, fix.websiteID, "prod")
	if err != nil {
		t.Fatalf("GetEnvironmentByName(prod) error = %v", err)
	}
	if targetEnvRow.ActiveReleaseID == nil || *targetEnvRow.ActiveReleaseID != result.ReleaseID {
		t.Fatalf("expected prod active release %q, got %#v", result.ReleaseID, targetEnvRow.ActiveReleaseID)
	}
	targetReleaseRow, err := fix.q.GetReleaseByID(ctx, result.ReleaseID)
	if err != nil {
		t.Fatalf("GetReleaseByID(target) error = %v", err)
	}
	manifest := map[string]any{}
	if err := json.Unmarshal([]byte(targetReleaseRow.ManifestJSON), &manifest); err != nil {
		t.Fatalf("parse target manifest json: %v", err)
	}
	if manifest["environment"] != "prod" || manifest["sourceEnv"] != "staging" || manifest["sourceReleaseId"] != sourceReleaseID {
		t.Fatalf("unexpected promoted manifest metadata: %#v", manifest)
	}

	sourceManifestInfo, err := os.Stat(filepath.Join(sourceDir, ".manifest.json"))
	if err != nil {
		t.Fatalf("stat source manifest metadata: %v", err)
	}
	targetManifestInfo, err := os.Stat(filepath.Join(targetDir, ".manifest.json"))
	if err != nil {
		t.Fatalf("stat target manifest metadata: %v", err)
	}
	if os.SameFile(sourceManifestInfo, targetManifestInfo) {
		t.Fatalf("expected target manifest metadata file to not be hard-linked to source metadata")
	}
}

func TestPromoteCopyFallbackWhenHardLinkFails(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	fix.seedSourceRelease(t, "01ARZ3NDEKTSV4RRFFQ69G5FAA", "")

	prevLinkFile := linkFile
	linkFile = func(oldname, newname string) error {
		return errors.New("simulated hard-link failure")
	}
	defer func() {
		linkFile = prevLinkFile
	}()

	result, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if result.Strategy != "copy" {
		t.Fatalf("expected copy strategy, got %#v", result)
	}
}

func TestPromoteRejectsSourceWithoutActiveRelease(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	_, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if !errors.Is(err, ErrPromotionSourceNoActive) {
		t.Fatalf("expected ErrPromotionSourceNoActive, got %v", err)
	}
}

func TestPromoteCopyFallbackPreservesSymlinks(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	sourceDir := fix.seedSourceRelease(t, sourceReleaseID, "")
	if err := os.Symlink("logo.svg", filepath.Join(sourceDir, "assets", "logo-link.svg")); err != nil {
		t.Fatalf("create source symlink error = %v", err)
	}
	fullHashes, err := computeOutputHashes(sourceDir)
	if err != nil {
		t.Fatalf("computeOutputHashes(source with symlink) error = %v", err)
	}
	fullHashesJSON, err := json.MarshalIndent(fullHashes, "", "  ")
	if err != nil {
		t.Fatalf("marshal source output hashes with symlink: %v", err)
	}
	if _, err := fix.db.Exec(`UPDATE releases SET output_hashes = ? WHERE id = ?`, string(fullHashesJSON), sourceReleaseID); err != nil {
		t.Fatalf("update source output hashes with symlink: %v", err)
	}

	prevLinkFile := linkFile
	linkFile = func(oldname, newname string) error {
		return errors.New("simulated hard-link failure")
	}
	defer func() {
		linkFile = prevLinkFile
	}()

	result, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	targetSymlink := filepath.Join(fix.websitesRoot, "futurelab", "envs", "prod", "releases", result.ReleaseID, "assets", "logo-link.svg")
	info, err := os.Lstat(targetSymlink)
	if err != nil {
		t.Fatalf("Lstat(target symlink) error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to remain a symlink after copy fallback", targetSymlink)
	}
}

func TestPromoteMixedLinkAndCopyStrategy(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	fix.seedSourceRelease(t, sourceReleaseID, "")

	prevLinkFile := linkFile
	linkFile = func(oldname, newname string) error {
		if strings.HasSuffix(oldname, "index.html") {
			return errors.New("simulated selective hard-link failure")
		}
		return prevLinkFile(oldname, newname)
	}
	defer func() {
		linkFile = prevLinkFile
	}()

	result, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if result.Strategy != "hardlink+copy" {
		t.Fatalf("expected mixed strategy hardlink+copy, got %#v", result)
	}
}

func TestPromoteHashMismatchKeepsTargetUnchanged(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	badHashes := `{"index.html":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	fix.seedSourceRelease(t, sourceReleaseID, badHashes)

	oldTargetReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAB"
	oldTargetDir := filepath.Join(fix.websitesRoot, "futurelab", "envs", "prod", "releases", oldTargetReleaseID)
	if err := os.MkdirAll(oldTargetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(old target release dir) error = %v", err)
	}
	if err := fix.q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            oldTargetReleaseID,
		EnvironmentID: fix.targetEnvID,
		ManifestJSON:  "{}",
		OutputHashes:  "{}",
		BuildLog:      "",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(old target) error = %v", err)
	}
	if err := fix.q.UpdateEnvironmentActiveRelease(ctx, fix.targetEnvID, &oldTargetReleaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease(old target) error = %v", err)
	}
	targetEnvDir := filepath.Join(fix.websitesRoot, "futurelab", "envs", "prod")
	if err := SwitchCurrentSymlink(targetEnvDir, oldTargetReleaseID); err != nil {
		t.Fatalf("SwitchCurrentSymlink(old target) error = %v", err)
	}

	_, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	var mismatchErr *HashMismatchError
	if !errors.As(err, &mismatchErr) {
		t.Fatalf("expected HashMismatchError, got %v", err)
	}

	targetEnvRow, err := fix.q.GetEnvironmentByName(ctx, fix.websiteID, "prod")
	if err != nil {
		t.Fatalf("GetEnvironmentByName(prod) error = %v", err)
	}
	if targetEnvRow.ActiveReleaseID == nil || *targetEnvRow.ActiveReleaseID != oldTargetReleaseID {
		t.Fatalf("expected prod active release unchanged at %q, got %#v", oldTargetReleaseID, targetEnvRow.ActiveReleaseID)
	}
	currentTarget, err := os.Readlink(filepath.Join(targetEnvDir, "current"))
	if err != nil {
		t.Fatalf("Readlink(prod/current) error = %v", err)
	}
	if currentTarget != filepath.ToSlash(filepath.Join("releases", oldTargetReleaseID)) {
		t.Fatalf("expected prod/current symlink unchanged, got %q", currentTarget)
	}
}

func TestPromoteRejectsSameEnvironment(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	_, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "staging")
	if !errors.Is(err, ErrPromotionSourceTargetMatch) {
		t.Fatalf("expected ErrPromotionSourceTargetMatch, got %v", err)
	}
}

func TestPromoteReturnsErrorWhenDBNilOrInputsInvalid(t *testing.T) {
	ctx := context.Background()
	_, err := Promote(ctx, nil, "/tmp/x", "futurelab", "staging", "prod")
	if err == nil || !strings.Contains(err.Error(), "database is required") {
		t.Fatalf("expected database required error, got %v", err)
	}
	db := openReleaseTestDB(t, filepath.Join(t.TempDir(), "db.sqlite"))
	defer db.Close()
	_, err = Promote(ctx, db, "/tmp/x", " ", "staging", "prod")
	if err == nil || !strings.Contains(err.Error(), "are required") {
		t.Fatalf("expected required args error, got %v", err)
	}
}

func TestPromoteNotFoundAndSourceValidationBranches(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	_, err := Promote(ctx, fix.db, fix.websitesRoot, "missing", "staging", "prod")
	if err == nil {
		t.Fatalf("expected website not found error")
	}
	_, err = Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "missing", "prod")
	if err == nil {
		t.Fatalf("expected source env not found error")
	}
	_, err = Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "missing")
	if err == nil {
		t.Fatalf("expected target env not found error")
	}

	fix.seedSourceRelease(t, "01ARZ3NDEKTSV4RRFFQ69G5FAA", "")
	foreignReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAZ"
	if err := fix.q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            foreignReleaseID,
		EnvironmentID: fix.targetEnvID,
		ManifestJSON:  "{}",
		OutputHashes:  "{}",
		BuildLog:      "",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(foreign env) error = %v", err)
	}
	if err := fix.q.UpdateEnvironmentActiveRelease(ctx, fix.sourceEnvID, &foreignReleaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease(source->foreign) error = %v", err)
	}
	_, err = Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err == nil || !strings.Contains(err.Error(), "does not belong to environment") {
		t.Fatalf("expected source validation error, got %v", err)
	}
}

func TestPromoteSourceReleaseRowMissingBranch(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	fix.seedSourceRelease(t, "01ARZ3NDEKTSV4RRFFQ69G5FAA", "")
	if _, err := fix.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatalf("disable foreign keys error = %v", err)
	}
	if _, err := fix.db.Exec(`UPDATE environments SET active_release_id = ? WHERE id = ?`, "01ARZ3NDEKTSV4RRFFQ69G5FAZ", fix.sourceEnvID); err != nil {
		t.Fatalf("force invalid active release id error = %v", err)
	}
	if _, err := fix.db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatalf("re-enable foreign keys error = %v", err)
	}

	_, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err == nil || !strings.Contains(err.Error(), "source release") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected source release missing error, got %v", err)
	}
}

func TestPromoteSourceReleaseDirectoryValidation(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	sourceDir := fix.seedSourceRelease(t, sourceReleaseID, "")
	if err := os.RemoveAll(sourceDir); err != nil {
		t.Fatalf("RemoveAll(source dir) error = %v", err)
	}
	if _, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod"); err == nil {
		t.Fatalf("expected missing source directory error")
	}

	sourceReleaseID2 := "01ARZ3NDEKTSV4RRFFQ69G5FAB"
	sourceDir = fix.seedSourceRelease(t, sourceReleaseID2, "")
	if err := os.RemoveAll(sourceDir); err != nil {
		t.Fatalf("RemoveAll(source dir 2) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sourceDir), 0o755); err != nil {
		t.Fatalf("MkdirAll(parent) error = %v", err)
	}
	if err := os.WriteFile(sourceDir, []byte("not dir"), 0o644); err != nil {
		t.Fatalf("WriteFile(source path) error = %v", err)
	}
	if _, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod"); err == nil {
		t.Fatalf("expected source path not directory error")
	}
}

func TestPromoteRejectsMalformedStoredOutputHashes(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	fix.seedSourceRelease(t, sourceReleaseID, "{malformed")
	_, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err == nil || !strings.Contains(err.Error(), "load source release hashes") {
		t.Fatalf("expected malformed hash payload error, got %v", err)
	}
}

func TestPromoteRecomputesSourceHashesWhenStoredEmpty(t *testing.T) {
	ctx := context.Background()
	fix := setupPromotionFixture(t)
	defer fix.db.Close()

	sourceReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	fix.seedSourceRelease(t, sourceReleaseID, "{}")
	result, err := Promote(ctx, fix.db, fix.websitesRoot, "futurelab", "staging", "prod")
	if err != nil {
		t.Fatalf("Promote() with empty stored hashes error = %v", err)
	}
	if result.ReleaseID == "" || result.Hash == "" {
		t.Fatalf("expected promotion result with computed hash, got %#v", result)
	}
}

func TestPromoteErrorHelpersAndInternalUtils(t *testing.T) {
	if got := (&HashMismatchError{}).Error(); !strings.Contains(got, "promotion hash verification failed") {
		t.Fatalf("unexpected HashMismatchError() default string: %q", got)
	}
	if got := (&HashMismatchError{Reason: "x"}).Error(); !strings.Contains(got, "x") {
		t.Fatalf("unexpected HashMismatchError() reason string: %q", got)
	}

	if _, err := loadSourcePromotionHashes("{malformed"); err == nil {
		t.Fatalf("expected malformed json error")
	}
	parsed, err := loadSourcePromotionHashes(`{"index.html":"sha256:a",".manifest.json":"sha256:b"," ":"sha256:c"}`)
	if err != nil {
		t.Fatalf("loadSourcePromotionHashes(valid) error = %v", err)
	}
	if len(parsed) != 1 || parsed["index.html"] == "" {
		t.Fatalf("unexpected filtered parsed hashes: %#v", parsed)
	}

	tmp := t.TempDir()
	if _, err := computePromotionHashes(filepath.Join(tmp, "missing")); err == nil {
		t.Fatalf("expected computePromotionHashes missing root error")
	}

	if mismatch := comparePromotionHashes(map[string]string{"a": "x"}, map[string]string{}); mismatch == "" {
		t.Fatalf("expected missing-file mismatch")
	}
	if mismatch := comparePromotionHashes(map[string]string{"a": "x"}, map[string]string{"a": "y"}); mismatch == "" {
		t.Fatalf("expected hash mismatch")
	}
	if mismatch := comparePromotionHashes(map[string]string{"a": "x"}, map[string]string{"a": "x", "b": "y"}); mismatch == "" {
		t.Fatalf("expected unexpected-file mismatch")
	}

	if _, err := promoteManifestJSON("{malformed", "staging", "prod", "R1"); err == nil {
		t.Fatalf("expected promoteManifestJSON parse error")
	}
	manifestJSON, err := promoteManifestJSON(`{"website":"futurelab"}`, "staging", "prod", "R1")
	if err != nil {
		t.Fatalf("promoteManifestJSON valid error = %v", err)
	}
	if !strings.Contains(manifestJSON, `"sourceReleaseId": "R1"`) || !strings.Contains(manifestJSON, `"sourceEnv": "staging"`) {
		t.Fatalf("unexpected promoted manifest json: %s", manifestJSON)
	}

	if log := promoteBuildLog("R1", "staging", "prod"); !strings.Contains(log, "R1") || !strings.Contains(log, "staging") || !strings.Contains(log, "prod") {
		t.Fatalf("unexpected promote build log: %s", log)
	}
}

func TestCopyReleaseContentSkipsMetadataAndCountsFiles(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "source")
	targetRoot := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(source/styles) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write source index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "styles", "default.css"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write source styles/default.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, ".manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write source .manifest.json: %v", err)
	}

	linked, copied, files, err := copyReleaseContent(sourceRoot, targetRoot)
	if err != nil {
		t.Fatalf("copyReleaseContent() error = %v", err)
	}
	if files != 2 {
		t.Fatalf("expected 2 content files copied, got linked=%d copied=%d files=%d", linked, copied, files)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, ".manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("expected metadata file to be skipped, got err=%v", err)
	}
}

func TestCopyReleaseContentWalkError(t *testing.T) {
	_, _, _, err := copyReleaseContent(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "target"))
	if err == nil {
		t.Fatalf("expected copyReleaseContent to fail for missing source root")
	}
}

func TestCopyFileSuccessAndErrors(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.txt")
	target := filepath.Join(tmp, "target", "copied.txt")
	if err := os.WriteFile(source, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := copyFile(source, target); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected copied content: %q", string(content))
	}

	if err := copyFile(filepath.Join(tmp, "missing.txt"), target); err == nil {
		t.Fatalf("expected error when source is missing")
	}

	if err := copyFile(source, filepath.Join(tmp, "missing-parent", "copied.txt")); err == nil {
		t.Fatalf("expected error when target parent directory is missing")
	}
}

type promotionFixture struct {
	db           *sql.DB
	q            *dbpkg.Queries
	websitesRoot string
	websiteID    int64
	sourceEnvID  int64
	targetEnvID  int64
}

func setupPromotionFixture(t *testing.T) promotionFixture {
	t.Helper()
	dataDir := t.TempDir()
	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	q := dbpkg.NewQueries(db)

	websiteID, err := q.InsertWebsite(context.Background(), dbpkg.WebsiteRow{Name: "futurelab", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	sourceEnvID, err := q.InsertEnvironment(context.Background(), dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	targetEnvID, err := q.InsertEnvironment(context.Background(), dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "prod"})
	if err != nil {
		t.Fatalf("InsertEnvironment(prod) error = %v", err)
	}

	return promotionFixture{
		db:           db,
		q:            q,
		websitesRoot: filepath.Join(dataDir, "websites"),
		websiteID:    websiteID,
		sourceEnvID:  sourceEnvID,
		targetEnvID:  targetEnvID,
	}
}

func (f promotionFixture) seedSourceRelease(t *testing.T, releaseID string, outputHashesOverride string) string {
	t.Helper()
	ctx := context.Background()
	sourceDir := filepath.Join(f.websitesRoot, "futurelab", "envs", "staging", "releases", releaseID)
	if err := os.MkdirAll(filepath.Join(sourceDir, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(styles) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "assets"), 0o755); err != nil {
		t.Fatalf("MkdirAll(assets) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("<!doctype html><h1>hello</h1>\n"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "styles", "default.css"), []byte("body{margin:0}\n"), 0o644); err != nil {
		t.Fatalf("write styles/default.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "assets", "logo.svg"), []byte("<svg></svg>\n"), 0o644); err != nil {
		t.Fatalf("write assets/logo.svg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".manifest.json"), []byte(`{"website":"futurelab","environment":"staging"}`), 0o644); err != nil {
		t.Fatalf("write .manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".build-log.txt"), []byte("build ok\n"), 0o644); err != nil {
		t.Fatalf("write .build-log.txt: %v", err)
	}
	fullHashes, err := computeOutputHashes(sourceDir)
	if err != nil {
		t.Fatalf("computeOutputHashes(source) error = %v", err)
	}
	fullHashesJSON, err := json.MarshalIndent(fullHashes, "", "  ")
	if err != nil {
		t.Fatalf("marshal source output hashes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".output-hashes.json"), fullHashesJSON, 0o644); err != nil {
		t.Fatalf("write .output-hashes.json: %v", err)
	}

	outputHashesJSON := outputHashesOverride
	if strings.TrimSpace(outputHashesJSON) == "" {
		outputHashesJSON = string(fullHashesJSON)
	}
	if err := f.q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            releaseID,
		EnvironmentID: f.sourceEnvID,
		ManifestJSON:  `{"website":"futurelab","environment":"staging"}`,
		OutputHashes:  outputHashesJSON,
		BuildLog:      "source release",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(source) error = %v", err)
	}
	if err := f.q.UpdateEnvironmentActiveRelease(ctx, f.sourceEnvID, &releaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease(source) error = %v", err)
	}
	sourceEnvDir := filepath.Join(f.websitesRoot, "futurelab", "envs", "staging")
	if err := SwitchCurrentSymlink(sourceEnvDir, releaseID); err != nil {
		t.Fatalf("SwitchCurrentSymlink(source) error = %v", err)
	}

	return sourceDir
}

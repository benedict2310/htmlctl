package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestRollbackSuccessAndRepeatedUndo(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	envID := seedRollbackEnvironment(t, ctx, q, websitesRoot, "sample", "staging", []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA",
		"01ARZ3NDEKTSV4RRFFQ69G5FAB",
		"01ARZ3NDEKTSV4RRFFQ69G5FAC",
	}, "01ARZ3NDEKTSV4RRFFQ69G5FAC")
	_ = envID

	first, err := Rollback(ctx, db, websitesRoot, "sample", "staging")
	if err != nil {
		t.Fatalf("Rollback(first) error = %v", err)
	}
	if first.FromReleaseID != "01ARZ3NDEKTSV4RRFFQ69G5FAC" || first.ToReleaseID != "01ARZ3NDEKTSV4RRFFQ69G5FAB" {
		t.Fatalf("unexpected first rollback result: %#v", first)
	}
	assertActiveRelease(t, ctx, q, websitesRoot, "sample", "staging", "01ARZ3NDEKTSV4RRFFQ69G5FAB")

	second, err := Rollback(ctx, db, websitesRoot, "sample", "staging")
	if err != nil {
		t.Fatalf("Rollback(second) error = %v", err)
	}
	if second.FromReleaseID != "01ARZ3NDEKTSV4RRFFQ69G5FAB" || second.ToReleaseID != "01ARZ3NDEKTSV4RRFFQ69G5FAA" {
		t.Fatalf("unexpected second rollback result: %#v", second)
	}
	assertActiveRelease(t, ctx, q, websitesRoot, "sample", "staging", "01ARZ3NDEKTSV4RRFFQ69G5FAA")
}

func TestRollbackReturnsNoPreviousReleaseError(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	seedRollbackEnvironment(t, ctx, q, websitesRoot, "sample", "staging", []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA",
	}, "01ARZ3NDEKTSV4RRFFQ69G5FAA")

	_, err := Rollback(ctx, db, websitesRoot, "sample", "staging")
	if !errors.Is(err, ErrNoPreviousRelease) {
		t.Fatalf("expected ErrNoPreviousRelease, got %v", err)
	}
}

func TestRollbackSkipsFailedReleaseTargets(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	type relRow struct {
		id     string
		status string
	}
	rows := []relRow{
		{id: "01ARZ3NDEKTSV4RRFFQ69G5FAA", status: "active"},
		{id: "01ARZ3NDEKTSV4RRFFQ69G5FAB", status: "failed"},
		{id: "01ARZ3NDEKTSV4RRFFQ69G5FAC", status: "active"},
	}
	for _, row := range rows {
		if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
			ID:            row.id,
			EnvironmentID: envID,
			ManifestJSON:  "{}",
			OutputHashes:  "{}",
			BuildLog:      "",
			Status:        row.status,
		}); err != nil {
			t.Fatalf("InsertRelease(%q) error = %v", row.id, err)
		}
		releaseDir := filepath.Join(websitesRoot, "sample", "envs", "staging", "releases", row.id)
		if err := os.MkdirAll(releaseDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", releaseDir, err)
		}
	}
	activeID := "01ARZ3NDEKTSV4RRFFQ69G5FAC"
	if err := q.UpdateEnvironmentActiveRelease(ctx, envID, &activeID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease() error = %v", err)
	}
	envDir := filepath.Join(websitesRoot, "sample", "envs", "staging")
	if err := SwitchCurrentSymlink(envDir, activeID); err != nil {
		t.Fatalf("SwitchCurrentSymlink(%q) error = %v", activeID, err)
	}

	result, err := Rollback(ctx, db, websitesRoot, "sample", "staging")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if result.ToReleaseID != "01ARZ3NDEKTSV4RRFFQ69G5FAA" {
		t.Fatalf("expected rollback to skip failed release and target oldest good release, got %#v", result)
	}
}

func TestRollbackMissingReleaseDirectoryDoesNotMutateActiveState(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")

	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	seedRollbackEnvironment(t, ctx, q, websitesRoot, "sample", "staging", []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA",
		"01ARZ3NDEKTSV4RRFFQ69G5FAB",
	}, "01ARZ3NDEKTSV4RRFFQ69G5FAB")

	missingDir := filepath.Join(websitesRoot, "sample", "envs", "staging", "releases", "01ARZ3NDEKTSV4RRFFQ69G5FAA")
	if err := os.RemoveAll(missingDir); err != nil {
		t.Fatalf("RemoveAll(%s) error = %v", missingDir, err)
	}

	_, err := Rollback(ctx, db, websitesRoot, "sample", "staging")
	var missingErr *MissingReleaseDirError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected MissingReleaseDirError, got %v", err)
	}
	assertActiveRelease(t, ctx, q, websitesRoot, "sample", "staging", "01ARZ3NDEKTSV4RRFFQ69G5FAB")
}

func TestRollbackReturnsErrorWhenDBNilOrInputsInvalid(t *testing.T) {
	ctx := context.Background()
	_, err := Rollback(ctx, nil, "/tmp/x", "sample", "staging")
	if err == nil || !strings.Contains(err.Error(), "database is required") {
		t.Fatalf("expected database required error, got %v", err)
	}
	db := openReleaseTestDB(t, filepath.Join(t.TempDir(), "db.sqlite"))
	defer db.Close()
	_, err = Rollback(ctx, db, "/tmp/x", " ", "staging")
	if err == nil || !strings.Contains(err.Error(), "website and environment are required") {
		t.Fatalf("expected website/environment required error, got %v", err)
	}
}

func TestRollbackNotFoundBranches(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")
	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	if _, err := Rollback(ctx, db, websitesRoot, "missing", "staging"); err == nil {
		t.Fatalf("expected website not found error")
	}

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := Rollback(ctx, db, websitesRoot, "sample", "staging"); err == nil {
		t.Fatalf("expected environment not found error")
	}
	if _, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"}); err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	if _, err := Rollback(ctx, db, websitesRoot, "sample", "staging"); !errors.Is(err, ErrNoPreviousRelease) {
		t.Fatalf("expected ErrNoPreviousRelease for no active release, got %v", err)
	}
}

func TestRollbackActiveReleaseNotInEnvironmentHistory(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")
	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	sourceEnvID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment(staging) error = %v", err)
	}
	otherEnvID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "prod"})
	if err != nil {
		t.Fatalf("InsertEnvironment(prod) error = %v", err)
	}
	otherReleaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            otherReleaseID,
		EnvironmentID: otherEnvID,
		ManifestJSON:  "{}",
		OutputHashes:  "{}",
		BuildLog:      "",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(prod) error = %v", err)
	}
	if err := q.UpdateEnvironmentActiveRelease(ctx, sourceEnvID, &otherReleaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease(staging) error = %v", err)
	}

	_, err = Rollback(ctx, db, websitesRoot, "sample", "staging")
	if err == nil || !strings.Contains(err.Error(), "was not found in release history") {
		t.Fatalf("expected active release history mismatch error, got %v", err)
	}
}

func TestRollbackTargetPathNotDirectory(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")
	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	oldID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	activeID := "01ARZ3NDEKTSV4RRFFQ69G5FAB"
	for _, id := range []string{oldID, activeID} {
		if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
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
	envDir := filepath.Join(websitesRoot, "sample", "envs", "staging")
	if err := os.MkdirAll(filepath.Join(envDir, "releases"), 0o755); err != nil {
		t.Fatalf("MkdirAll(releases) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "releases", oldID), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile(target path) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(envDir, "releases", activeID), 0o755); err != nil {
		t.Fatalf("MkdirAll(active dir) error = %v", err)
	}
	if err := q.UpdateEnvironmentActiveRelease(ctx, envID, &activeID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease() error = %v", err)
	}
	if err := SwitchCurrentSymlink(envDir, activeID); err != nil {
		t.Fatalf("SwitchCurrentSymlink() error = %v", err)
	}

	_, err = Rollback(ctx, db, websitesRoot, "sample", "staging")
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

func TestRollbackReadCurrentSymlinkError(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	websitesRoot := filepath.Join(dataDir, "websites")
	db := openReleaseTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()
	q := dbpkg.NewQueries(db)

	seedRollbackEnvironment(t, ctx, q, websitesRoot, "sample", "staging", []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA",
		"01ARZ3NDEKTSV4RRFFQ69G5FAB",
	}, "01ARZ3NDEKTSV4RRFFQ69G5FAB")

	currentPath := filepath.Join(websitesRoot, "sample", "envs", "staging", "current")
	if err := os.Remove(currentPath); err != nil {
		t.Fatalf("remove current symlink: %v", err)
	}
	if err := os.WriteFile(currentPath, []byte("not-symlink"), 0o644); err != nil {
		t.Fatalf("write regular current file: %v", err)
	}

	_, err := Rollback(ctx, db, websitesRoot, "sample", "staging")
	if err == nil || !strings.Contains(err.Error(), "read current symlink") {
		t.Fatalf("expected read current symlink error, got %v", err)
	}
}

func TestMissingReleaseDirErrorString(t *testing.T) {
	err := (&MissingReleaseDirError{ReleaseID: "R1", Path: "/tmp/x"}).Error()
	if !strings.Contains(err, "R1") || !strings.Contains(err, "/tmp/x") {
		t.Fatalf("unexpected error string: %s", err)
	}
}

func seedRollbackEnvironment(t *testing.T, ctx context.Context, q *dbpkg.Queries, websitesRoot, website, env string, releaseIDs []string, activeReleaseID string) int64 {
	t.Helper()

	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: website, DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: env})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	for _, id := range releaseIDs {
		if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
			ID:            id,
			EnvironmentID: envID,
			ManifestJSON:  "{}",
			OutputHashes:  "{}",
			BuildLog:      "",
			Status:        "active",
		}); err != nil {
			t.Fatalf("InsertRelease(%q) error = %v", id, err)
		}
		releaseDir := filepath.Join(websitesRoot, website, "envs", env, "releases", id)
		if err := os.MkdirAll(releaseDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", releaseDir, err)
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

func assertActiveRelease(t *testing.T, ctx context.Context, q *dbpkg.Queries, websitesRoot, website, env, expected string) {
	t.Helper()
	websiteRow, err := q.GetWebsiteByName(ctx, website)
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, env)
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if envRow.ActiveReleaseID == nil || *envRow.ActiveReleaseID != expected {
		t.Fatalf("expected active release %q, got %#v", expected, envRow.ActiveReleaseID)
	}
	target, err := os.Readlink(filepath.Join(websitesRoot, website, "envs", env, "current"))
	if err != nil {
		t.Fatalf("Readlink(current) error = %v", err)
	}
	if target != filepath.ToSlash(filepath.Join("releases", expected)) {
		t.Fatalf("expected current symlink target %q, got %q", filepath.ToSlash(filepath.Join("releases", expected)), target)
	}
}

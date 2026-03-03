package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/internal/release"
)

func TestRetentionEndpointRunsDryRun(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	envID := seedServerRetentionEnvironment(t, srv, []serverRetentionSeedRelease{
		{ID: "03", Status: "active"},
		{ID: "02", Status: "active"},
		{ID: "01", Status: "active"},
	}, "03")
	if _, err := dbpkg.NewQueries(srv.db).InsertReleasePreview(context.Background(), dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "01",
		Hostname:      "preview--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     time.Now().UTC().Add(24 * time.Hour).Format(releaseRetentionTimestampLayoutForTest),
	}); err != nil {
		t.Fatalf("InsertReleasePreview() error = %v", err)
	}

	body := bytes.NewBufferString(`{"keep":1,"dryRun":true}`)
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/retention/run", "application/json", body)
	if err != nil {
		t.Fatalf("POST /retention/run error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}

	var out retentionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode retention response: %v", err)
	}
	if !out.DryRun || out.Keep != 1 {
		t.Fatalf("unexpected retention response: %#v", out)
	}
	if got, want := strings.Join(out.PrunableReleaseIDs, ","), ""; got != want {
		t.Fatalf("unexpected prunable release ids: got %q want %q", got, want)
	}
	if got, want := strings.Join(out.PreviewPinnedReleaseIDs, ","), "01"; got != want {
		t.Fatalf("unexpected preview pinned ids: got %q want %q", got, want)
	}
}

func TestRetentionEndpointRejectsInvalidKeep(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/retention/run", "application/json", bytes.NewBufferString(`{"keep":-1}`))
	if err != nil {
		t.Fatalf("POST /retention/run error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestRetentionEndpointInternalErrorIsSanitized(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/retention/run", "application/json", bytes.NewBufferString(`{"keep":1}`))
	if err != nil {
		t.Fatalf("POST /retention/run error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(b))
	}
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if strings.Contains(string(rawBody), "database is closed") {
		t.Fatalf("response body leaked internal error: %s", string(rawBody))
	}
}

const releaseRetentionTimestampLayoutForTest = "2006-01-02T15:04:05.000000000Z"

type serverRetentionSeedRelease struct {
	ID     string
	Status string
}

func seedServerRetentionEnvironment(t *testing.T, srv *Server, releases []serverRetentionSeedRelease, activeReleaseID string) int64 {
	t.Helper()

	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()
	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	baseTime := time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	for i, rel := range releases {
		if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
			ID:            rel.ID,
			EnvironmentID: envID,
			ManifestJSON:  "{}",
			OutputHashes:  "{}",
			BuildLog:      "",
			Status:        rel.Status,
		}); err != nil {
			t.Fatalf("InsertRelease(%s) error = %v", rel.ID, err)
		}
		releaseDir := filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "staging", "releases", rel.ID)
		if err := os.MkdirAll(releaseDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", releaseDir, err)
		}
		createdAt := baseTime.Add(-time.Duration(i) * time.Minute).Format(releaseRetentionTimestampLayoutForTest)
		if _, err := srv.db.ExecContext(ctx, `UPDATE releases SET created_at = ? WHERE id = ?`, createdAt, rel.ID); err != nil {
			t.Fatalf("UPDATE releases.created_at(%s) error = %v", rel.ID, err)
		}
	}
	if err := q.UpdateEnvironmentActiveRelease(ctx, envID, &activeReleaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease() error = %v", err)
	}
	envDir := filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "staging")
	if err := release.SwitchCurrentSymlink(envDir, activeReleaseID); err != nil {
		t.Fatalf("SwitchCurrentSymlink() error = %v", err)
	}
	return envID
}

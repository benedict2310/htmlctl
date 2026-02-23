package server

import (
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
)

func TestRollbackEndpointSuccess(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	first := createReleaseWithActor(t, baseURL, "alice")
	second := createReleaseWithActor(t, baseURL, "bob")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/websites/futurelab/environments/staging/rollback", nil)
	if err != nil {
		t.Fatalf("new rollback request: %v", err)
	}
	req.Header.Set("X-Actor", "carol")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /rollback error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}
	var out rollbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode rollback response: %v", err)
	}
	if out.FromReleaseID != second || out.ToReleaseID != first {
		t.Fatalf("unexpected rollback response: %#v", out)
	}

	currentTarget, err := os.Readlink(filepath.Join(srv.dataPaths.WebsitesRoot, "futurelab", "envs", "staging", "current"))
	if err != nil {
		t.Fatalf("read current symlink: %v", err)
	}
	if currentTarget != filepath.ToSlash(filepath.Join("releases", first)) {
		t.Fatalf("expected current -> %q, got %q", filepath.ToSlash(filepath.Join("releases", first)), currentTarget)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if envRow.ActiveReleaseID == nil || *envRow.ActiveReleaseID != first {
		t.Fatalf("expected active_release_id %q, got %#v", first, envRow.ActiveReleaseID)
	}

	waitForRollbackAuditEntry(t, baseURL)
}

func TestRollbackEndpointNoPreviousRelease(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	_ = createReleaseWithActor(t, baseURL, "alice")

	resp, err := http.Post(baseURL+"/api/v1/websites/futurelab/environments/staging/rollback", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /rollback error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read rollback response body: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestRollbackEndpointMissingTargetReleaseDirectory(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	first := createReleaseWithActor(t, baseURL, "alice")
	second := createReleaseWithActor(t, baseURL, "bob")

	missingDir := filepath.Join(srv.dataPaths.WebsitesRoot, "futurelab", "envs", "staging", "releases", first)
	if err := os.RemoveAll(missingDir); err != nil {
		t.Fatalf("RemoveAll(%s) error = %v", missingDir, err)
	}

	resp, err := http.Post(baseURL+"/api/v1/websites/futurelab/environments/staging/rollback", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /rollback error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read rollback response body: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(body))
	}

	if strings.Contains(string(body), missingDir) || strings.Contains(string(body), first) {
		t.Fatalf("rollback conflict response leaked internal path or release id: %s", string(body))
	}

	var out struct {
		Error   string   `json:"error"`
		Details []string `json:"details,omitempty"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode rollback conflict response: %v body=%s", err, string(body))
	}
	if out.Error != "rollback target release directory is missing" {
		t.Fatalf("expected sanitized rollback message, got %q", out.Error)
	}
	if len(out.Details) != 0 {
		t.Fatalf("expected no details for rollback missing-dir conflict, got %#v", out.Details)
	}

	currentTarget, err := os.Readlink(filepath.Join(srv.dataPaths.WebsitesRoot, "futurelab", "envs", "staging", "current"))
	if err != nil {
		t.Fatalf("read current symlink: %v", err)
	}
	if currentTarget != filepath.ToSlash(filepath.Join("releases", second)) {
		t.Fatalf("expected current symlink unchanged at %q, got %q", filepath.ToSlash(filepath.Join("releases", second)), currentTarget)
	}
}

func TestRollbackEndpointMethodNotAllowed(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/api/v1/websites/futurelab/environments/staging/rollback")
	if err != nil {
		t.Fatalf("GET /rollback error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 405, got %d body=%s", resp.StatusCode, string(b))
	}
}

func waitForRollbackAuditEntry(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/v1/websites/futurelab/environments/staging/logs?operation=rollback")
		if err != nil {
			t.Fatalf("GET /logs?operation=rollback error = %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("expected 200 for rollback logs, got %d body=%s", resp.StatusCode, string(b))
		}
		var out logsResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			t.Fatalf("decode rollback logs response: %v", err)
		}
		resp.Body.Close()
		if out.Total >= 1 && len(out.Entries) >= 1 {
			if out.Entries[0].Operation != "rollback" {
				t.Fatalf("expected rollback operation, got %#v", out.Entries[0])
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for rollback audit entry")
}

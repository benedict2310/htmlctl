package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestReleaseHandlersServiceUnavailable(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites/sample/environments/staging/releases", nil)
	rec := httptest.NewRecorder()
	s.handleRelease(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected GET /releases to return 503, got %d body=%s", resp.StatusCode, string(body))
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/websites/sample/environments/staging/releases", nil)
	rec = httptest.NewRecorder()
	s.handleRelease(rec, req)
	resp = rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected POST /releases to return 503, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestReleaseHistoryEnvironmentNotFoundBranch(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	q := dbpkg.NewQueries(srv.db)

	if _, err := q.InsertWebsite(context.Background(), dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"}); err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/releases")
	if err != nil {
		t.Fatalf("GET /releases env not found error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 404, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestReleaseCreateWebsiteNotFoundBranch(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Post(baseURL+"/api/v1/websites/missing/environments/staging/releases", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /releases missing website error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 404, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestReleaseHistoryUnknownActorFallback(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	q := dbpkg.NewQueries(srv.db)

	websiteID, err := q.InsertWebsite(context.Background(), dbpkg.WebsiteRow{Name: "sample", DefaultStyleBundle: "default", BaseTemplate: "default"})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	envID, err := q.InsertEnvironment(context.Background(), dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"})
	if err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	releaseID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"
	if err := q.InsertRelease(context.Background(), dbpkg.ReleaseRow{
		ID:            releaseID,
		EnvironmentID: envID,
		ManifestJSON:  "{}",
		OutputHashes:  "{}",
		BuildLog:      "",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease() error = %v", err)
	}
	if err := q.UpdateEnvironmentActiveRelease(context.Background(), envID, &releaseID); err != nil {
		t.Fatalf("UpdateEnvironmentActiveRelease() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/releases")
	if err != nil {
		t.Fatalf("GET /releases error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}
	var out releasesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode releases response: %v", err)
	}
	if len(out.Releases) != 1 || out.Releases[0].Actor != "unknown" {
		t.Fatalf("expected actor fallback unknown, got %#v", out.Releases)
	}
}

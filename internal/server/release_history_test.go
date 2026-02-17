package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestReleaseHistoryPaginationAndActor(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	first := createReleaseWithActor(t, baseURL, "alice")
	second := createReleaseWithActor(t, baseURL, "bob")

	resp, err := http.Get(baseURL + "/api/v1/websites/futurelab/environments/staging/releases?limit=1")
	if err != nil {
		t.Fatalf("GET release history error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}
	var pageOne releasesResponse
	if err := json.NewDecoder(resp.Body).Decode(&pageOne); err != nil {
		t.Fatalf("decode page one response: %v", err)
	}
	if pageOne.Limit != 1 || pageOne.Offset != 0 {
		t.Fatalf("unexpected pagination metadata: %#v", pageOne)
	}
	if len(pageOne.Releases) != 1 {
		t.Fatalf("expected one row in page one, got %#v", pageOne.Releases)
	}
	if pageOne.Releases[0].ReleaseID != second || pageOne.Releases[0].Actor != "bob" || pageOne.Releases[0].Status != "active" {
		t.Fatalf("unexpected first page row: %#v", pageOne.Releases[0])
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/futurelab/environments/staging/releases?limit=1&offset=1")
	if err != nil {
		t.Fatalf("GET release history page two error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}
	var pageTwo releasesResponse
	if err := json.NewDecoder(resp.Body).Decode(&pageTwo); err != nil {
		t.Fatalf("decode page two response: %v", err)
	}
	if pageTwo.Limit != 1 || pageTwo.Offset != 1 {
		t.Fatalf("unexpected second-page pagination metadata: %#v", pageTwo)
	}
	if len(pageTwo.Releases) != 1 {
		t.Fatalf("expected one row in page two, got %#v", pageTwo.Releases)
	}
	if pageTwo.Releases[0].ReleaseID != first || pageTwo.Releases[0].Actor != "alice" || pageTwo.Releases[0].Status != "previous" {
		t.Fatalf("unexpected second page row: %#v", pageTwo.Releases[0])
	}
}

func TestReleaseHistoryRejectsInvalidPagination(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/api/v1/websites/futurelab/environments/staging/releases?limit=-1")
	if err != nil {
		t.Fatalf("GET release history invalid pagination error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}

func createReleaseWithActor(t *testing.T, baseURL, actor string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/websites/futurelab/environments/staging/releases", nil)
	if err != nil {
		t.Fatalf("new release request: %v", err)
	}
	req.Header.Set("X-Actor", actor)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /releases error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(b))
	}
	var out releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode release response: %v", err)
	}
	return out.ReleaseID
}

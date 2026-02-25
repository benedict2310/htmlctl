package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestReleaseEndpointBuildsAndActivatesRelease(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/websites/sample/environments/staging/releases", nil)
	if err != nil {
		t.Fatalf("new release request: %v", err)
	}
	req.Header.Set("X-Actor", "bene")
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
	if out.ReleaseID == "" || out.Status != "active" {
		t.Fatalf("unexpected response: %#v", out)
	}

	releaseDir := filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "staging", "releases", out.ReleaseID)
	for _, rel := range []string{"index.html", "styles/tokens.css", "styles/default.css", "assets/logo.svg", ".manifest.json", ".build-log.txt", ".output-hashes.json"} {
		if _, err := os.Stat(filepath.Join(releaseDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected release output file %s: %v", rel, err)
		}
	}
	currentTarget, err := os.Readlink(filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "staging", "current"))
	if err != nil {
		t.Fatalf("read current symlink: %v", err)
	}
	if currentTarget != filepath.ToSlash(filepath.Join("releases", out.ReleaseID)) {
		t.Fatalf("unexpected current symlink target %q", currentTarget)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if envRow.ActiveReleaseID == nil || *envRow.ActiveReleaseID != out.ReleaseID {
		t.Fatalf("expected active_release_id %q, got %#v", out.ReleaseID, envRow.ActiveReleaseID)
	}
	releaseRow, err := q.GetReleaseByID(context.Background(), out.ReleaseID)
	if err != nil {
		t.Fatalf("GetReleaseByID() error = %v", err)
	}
	if releaseRow.Status != "active" {
		t.Fatalf("expected active release status, got %q", releaseRow.Status)
	}
}

func TestReleaseEndpointFailureRecordsFailedStatus(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	page := []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  description: Home page\n  layout:\n    - include: missing\n")
	tokensCSS := []byte(":root { --brand: #00f; }")
	defaultCSS := []byte("body { margin: 0; }")
	body := tarBody(t, map[string][]byte{
		"pages/index.page.yaml": page,
		"styles/tokens.css":     tokensCSS,
		"styles/default.css":    defaultCSS,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "sample",
		"resources": []map[string]any{
			{"kind": "Page", "name": "index", "file": "pages/index.page.yaml", "hash": "sha256:" + sha256Hex(page)},
			{"kind": "StyleBundle", "name": "default", "files": []map[string]any{{"file": "styles/tokens.css", "hash": "sha256:" + sha256Hex(tokensCSS)}, {"file": "styles/default.css", "hash": "sha256:" + sha256Hex(defaultCSS)}}},
		},
	})
	resp := postBundle(t, baseURL+"/api/v1/websites/sample/environments/staging/apply", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected apply to succeed, got %d", resp.StatusCode)
	}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/releases", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /releases error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(b))
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	releases, err := q.ListReleasesByEnvironment(context.Background(), envRow.ID)
	if err != nil {
		t.Fatalf("ListReleasesByEnvironment() error = %v", err)
	}
	if len(releases) == 0 || releases[0].Status != "failed" {
		t.Fatalf("expected latest release status failed, got %#v", releases)
	}

	listResp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/releases")
	if err != nil {
		t.Fatalf("GET /releases error = %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		t.Fatalf("expected 200, got %d body=%s", listResp.StatusCode, string(body))
	}
	var out releasesResponse
	if err := json.NewDecoder(listResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode releases response: %v", err)
	}
	if len(out.Releases) == 0 || out.Releases[0].Status != "failed" {
		t.Fatalf("expected failed status in API response, got %#v", out.Releases)
	}
}

func TestReleaseEndpointMethodNotAllowed(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/releases", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /releases error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 405, got %d body=%s", resp.StatusCode, string(b))
	}
}

func applySampleSite(t *testing.T, baseURL string) {
	t.Helper()
	component := []byte("<section id=\"header\">Header</section>")
	page := []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  description: Home page\n  layout:\n    - include: header\n")
	tokensCSS := []byte(":root { --brand: #00f; }")
	defaultCSS := []byte("body { margin: 0; }")
	asset := []byte("<svg></svg>")

	body := tarBody(t, map[string][]byte{
		"components/header.html": component,
		"pages/index.page.yaml":  page,
		"styles/tokens.css":      tokensCSS,
		"styles/default.css":     defaultCSS,
		"assets/logo.svg":        asset,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "sample",
		"resources": []map[string]any{
			{"kind": "Component", "name": "header", "file": "components/header.html", "hash": "sha256:" + sha256Hex(component)},
			{"kind": "Page", "name": "index", "file": "pages/index.page.yaml", "hash": "sha256:" + sha256Hex(page)},
			{"kind": "StyleBundle", "name": "default", "files": []map[string]any{{"file": "styles/tokens.css", "hash": "sha256:" + sha256Hex(tokensCSS)}, {"file": "styles/default.css", "hash": "sha256:" + sha256Hex(defaultCSS)}}},
			{"kind": "Asset", "name": "assets/logo.svg", "file": "assets/logo.svg", "hash": "sha256:" + sha256Hex(asset), "contentType": "image/svg+xml"},
		},
	})
	resp := postBundle(t, baseURL+"/api/v1/websites/sample/environments/staging/apply", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected apply success, got %d", resp.StatusCode)
	}
}

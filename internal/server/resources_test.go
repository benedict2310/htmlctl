package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestWebsitesEnvironmentsStatusAndReleasesEndpoints(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	releaseResp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/releases", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /releases error = %v", err)
	}
	defer releaseResp.Body.Close()
	if releaseResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(releaseResp.Body)
		t.Fatalf("expected 201, got %d body=%s", releaseResp.StatusCode, string(body))
	}
	var created releaseResponse
	if err := json.NewDecoder(releaseResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode release response: %v", err)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(t.Context(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if err := q.UpsertAsset(t.Context(), dbpkg.AssetRow{
		WebsiteID:   websiteRow.ID,
		Filename:    "app.js",
		ContentType: "text/javascript",
		SizeBytes:   18,
		ContentHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}); err != nil {
		t.Fatalf("UpsertAsset() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites")
	if err != nil {
		t.Fatalf("GET /websites error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected websites status 200, got %d", resp.StatusCode)
	}
	var websites websitesResponse
	if err := json.NewDecoder(resp.Body).Decode(&websites); err != nil {
		t.Fatalf("decode websites response: %v", err)
	}
	if len(websites.Websites) != 1 || websites.Websites[0].Name != "sample" {
		t.Fatalf("unexpected websites payload: %#v", websites)
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/sample/environments")
	if err != nil {
		t.Fatalf("GET /environments error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected environments status 200, got %d", resp.StatusCode)
	}
	var envs environmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envs); err != nil {
		t.Fatalf("decode environments response: %v", err)
	}
	if len(envs.Environments) != 1 || envs.Environments[0].Name != "staging" {
		t.Fatalf("unexpected environments payload: %#v", envs)
	}
	if envs.Environments[0].ActiveReleaseID == nil || *envs.Environments[0].ActiveReleaseID != created.ReleaseID {
		t.Fatalf("unexpected active release: %#v", envs.Environments[0].ActiveReleaseID)
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/sample/environments/staging/status")
	if err != nil {
		t.Fatalf("GET /status error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status endpoint 200, got %d", resp.StatusCode)
	}
	var status statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status.ActiveReleaseID == nil || *status.ActiveReleaseID != created.ReleaseID {
		t.Fatalf("unexpected status active release: %#v", status.ActiveReleaseID)
	}
	if status.ResourceCounts.Pages != 1 || status.ResourceCounts.Components != 1 || status.ResourceCounts.Styles != 1 {
		t.Fatalf("unexpected resource counts: %#v", status.ResourceCounts)
	}
	if status.ResourceCounts.Scripts != 1 {
		t.Fatalf("expected script count 1, got %#v", status.ResourceCounts)
	}
	if status.ResourceCounts.Assets != 1 {
		t.Fatalf("expected non-script asset count 1, got %#v", status.ResourceCounts)
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/sample/environments/staging/releases")
	if err != nil {
		t.Fatalf("GET /releases error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected releases list status 200, got %d", resp.StatusCode)
	}
	var releases releasesResponse
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		t.Fatalf("decode releases response: %v", err)
	}
	if len(releases.Releases) != 1 {
		t.Fatalf("expected one release, got %#v", releases.Releases)
	}
	if releases.Releases[0].ReleaseID != created.ReleaseID || !releases.Releases[0].Active {
		t.Fatalf("unexpected releases payload: %#v", releases.Releases)
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/sample/environments/staging/manifest")
	if err != nil {
		t.Fatalf("GET /manifest error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected manifest status 200, got %d", resp.StatusCode)
	}
	var manifest desiredStateManifestResponse
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest response: %v", err)
	}
	if manifest.Website != "sample" || manifest.Environment != "staging" {
		t.Fatalf("unexpected manifest metadata: %#v", manifest)
	}
	if len(manifest.Files) != 6 {
		t.Fatalf("expected 6 manifest files, got %d (%#v)", len(manifest.Files), manifest.Files)
	}
	paths := map[string]bool{}
	for _, file := range manifest.Files {
		paths[file.Path] = true
	}
	for _, want := range []string{
		"components/header.html",
		"pages/index.page.yaml",
		"styles/tokens.css",
		"styles/default.css",
		"assets/logo.svg",
		"app.js",
	} {
		if !paths[want] {
			t.Fatalf("missing expected manifest path %q in %#v", want, manifest.Files)
		}
	}
}

func TestManifestIncludesWebsiteAndBrandingFiles(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(t.Context(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if err := q.UpdateWebsiteSpec(t.Context(), dbpkg.WebsiteRow{
		ID:                 websiteRow.ID,
		DefaultStyleBundle: websiteRow.DefaultStyleBundle,
		BaseTemplate:       websiteRow.BaseTemplate,
		HeadJSON:           `{"icons":{"svg":"branding/favicon.svg"}}`,
		ContentHash:        "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}); err != nil {
		t.Fatalf("UpdateWebsiteSpec() error = %v", err)
	}
	if err := q.UpsertWebsiteIcon(t.Context(), dbpkg.WebsiteIconRow{
		WebsiteID:   websiteRow.ID,
		Slot:        "svg",
		SourcePath:  "branding/favicon.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   12,
		ContentHash: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}); err != nil {
		t.Fatalf("UpsertWebsiteIcon() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/manifest")
	if err != nil {
		t.Fatalf("GET /manifest error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected manifest status 200, got %d", resp.StatusCode)
	}
	var manifest desiredStateManifestResponse
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest response: %v", err)
	}
	paths := map[string]bool{}
	for _, file := range manifest.Files {
		paths[file.Path] = true
	}
	for _, want := range []string{"website.yaml", "branding/favicon.svg"} {
		if !paths[want] {
			t.Fatalf("missing expected manifest path %q in %#v", want, manifest.Files)
		}
	}
}

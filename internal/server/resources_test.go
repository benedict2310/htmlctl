package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
		Filename:    "scripts/site.js",
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
	if status.DefaultStyleBundle != "default" || status.BaseTemplate != "default" {
		t.Fatalf("unexpected status style/template summary: %#v", status)
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
		"scripts/site.js",
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

func TestResourcesEndpointIncludesSiteInventory(t *testing.T) {
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
		SEOJSON:            `{"publicBaseURL":"https://example.com/","displayName":"Sample Studio","robots":{"enabled":true},"structuredData":{"enabled":true}}`,
		ContentHash:        "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}); err != nil {
		t.Fatalf("UpdateWebsiteSpec() error = %v", err)
	}
	if err := q.UpsertPage(t.Context(), dbpkg.PageRow{
		WebsiteID:   websiteRow.ID,
		Name:        "index",
		Route:       "/",
		Title:       "Home",
		Description: "Home page",
		LayoutJSON:  `[{"include":"header"},{"include":"hero"}]`,
		HeadJSON:    `{"canonicalURL":"https://example.com/","openGraph":{"title":"Home OG"}}`,
		ContentHash: "sha256:pagepagepagepagepagepagepagepagepagepagepagepagepagepagepagepage",
	}); err != nil {
		t.Fatalf("UpsertPage() error = %v", err)
	}
	if err := q.UpsertComponent(t.Context(), dbpkg.ComponentRow{
		WebsiteID:   websiteRow.ID,
		Name:        "header",
		Scope:       "global",
		ContentHash: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		CSSHash:     "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		JSHash:      "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
	}); err != nil {
		t.Fatalf("UpsertComponent() error = %v", err)
	}
	if err := q.UpsertComponent(t.Context(), dbpkg.ComponentRow{
		WebsiteID:   websiteRow.ID,
		Name:        "hero",
		Scope:       "global",
		ContentHash: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}); err != nil {
		t.Fatalf("UpsertComponent() error = %v", err)
	}
	if err := q.UpsertWebsiteIcon(t.Context(), dbpkg.WebsiteIconRow{
		WebsiteID:   websiteRow.ID,
		Slot:        "svg",
		SourcePath:  "branding/favicon.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   12,
		ContentHash: "sha256:1212121212121212121212121212121212121212121212121212121212121212",
	}); err != nil {
		t.Fatalf("UpsertWebsiteIcon() error = %v", err)
	}
	if err := q.UpsertAsset(t.Context(), dbpkg.AssetRow{
		WebsiteID:   websiteRow.ID,
		Filename:    "scripts/site.js",
		ContentType: "text/javascript; charset=utf-8",
		SizeBytes:   23,
		ContentHash: "sha256:3434343434343434343434343434343434343434343434343434343434343434",
	}); err != nil {
		t.Fatalf("UpsertAsset(script) error = %v", err)
	}
	if err := q.UpsertAsset(t.Context(), dbpkg.AssetRow{
		WebsiteID:   websiteRow.ID,
		Filename:    "assets/widget.mjs",
		ContentType: "text/javascript; charset=utf-8",
		SizeBytes:   31,
		ContentHash: "sha256:5656565656565656565656565656565656565656565656565656565656565656",
	}); err != nil {
		t.Fatalf("UpsertAsset(asset mjs) error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/resources")
	if err != nil {
		t.Fatalf("GET /resources error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected resources status 200, got %d body=%s", resp.StatusCode, string(body))
	}
	var resources resourcesResponse
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		t.Fatalf("decode resources response: %v", err)
	}
	if resources.Website != "sample" || resources.Environment != "staging" {
		t.Fatalf("unexpected resources metadata: %#v", resources)
	}
	if resources.Site.SEO == nil || resources.Site.SEO.PublicBaseURL != "https://example.com/" {
		t.Fatalf("expected website seo metadata, got %#v", resources.Site.SEO)
	}
	if len(resources.Pages) != 1 || len(resources.Pages[0].Layout) != 2 {
		t.Fatalf("unexpected pages payload: %#v", resources.Pages)
	}
	if resources.Pages[0].Head == nil || resources.Pages[0].Head.CanonicalURL != "https://example.com/" {
		t.Fatalf("expected page head metadata, got %#v", resources.Pages[0].Head)
	}
	if len(resources.Components) != 2 || !resources.Components[0].HasCSS || !resources.Components[0].HasJS {
		t.Fatalf("unexpected components payload: %#v", resources.Components)
	}
	if len(resources.Styles) != 1 || len(resources.Styles[0].Files) != 2 {
		t.Fatalf("unexpected styles payload: %#v", resources.Styles)
	}
	if len(resources.Assets) != 2 || resources.Assets[0].Path != "assets/logo.svg" || resources.Assets[1].Path != "assets/widget.mjs" {
		t.Fatalf("unexpected assets payload: %#v", resources.Assets)
	}
	if len(resources.Branding) != 1 || resources.Branding[0].SourcePath != "branding/favicon.svg" {
		t.Fatalf("unexpected branding payload: %#v", resources.Branding)
	}
	if resources.ResourceCounts.Pages != 1 || resources.ResourceCounts.Components != 2 || resources.ResourceCounts.Styles != 1 || resources.ResourceCounts.Assets != 2 || resources.ResourceCounts.Scripts != 1 {
		t.Fatalf("unexpected resource counts: %#v", resources.ResourceCounts)
	}
}

func TestResourcesEndpointMissingWebsiteReturnsWebsiteNotFound(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/api/v1/websites/missing/environments/staging/resources")
	if err != nil {
		t.Fatalf("GET /resources missing website error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) == "" || !bytes.Contains(body, []byte(`website \"missing\" not found`)) {
		t.Fatalf("unexpected missing website body: %s", string(body))
	}
}

func TestSourceEndpointExportsCanonicalSiteArchive(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/source")
	if err != nil {
		t.Fatalf("GET /source error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected source status 200, got %d body=%s", resp.StatusCode, string(body))
	}
	if got := resp.Header.Get("Content-Type"); got != "application/gzip" {
		t.Fatalf("unexpected content type %q", got)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	paths := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		paths[hdr.Name] = true
	}
	for _, want := range []string{
		"website.yaml",
		"pages/index.page.yaml",
		"components/header.html",
		"styles/tokens.css",
		"styles/default.css",
		"assets/logo.svg",
	} {
		if !paths[want] {
			t.Fatalf("missing exported path %q in %#v", want, paths)
		}
	}
}

func TestSourceEndpointPreservesOriginalWebsiteAndPageYAML(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(t.Context(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}

	websiteBytes := []byte("apiVersion: htmlctl.dev/v1\nkind: Website\nmetadata:\n  name: sample\nspec:\n  baseTemplate: default\n  defaultStyleBundle: default  # keep comments\n")
	pageBytes := []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  title: Home\n  route: /\n  description: Home page\n  layout:\n    - include: header\n")
	websiteHash := "sha256:" + sha256Hex(websiteBytes)
	pageHash := "sha256:" + sha256Hex(pageBytes)
	if _, err := srv.blobStore.Put(t.Context(), sha256Hex(websiteBytes), websiteBytes); err != nil {
		t.Fatalf("Put(website) error = %v", err)
	}
	if _, err := srv.blobStore.Put(t.Context(), sha256Hex(pageBytes), pageBytes); err != nil {
		t.Fatalf("Put(page) error = %v", err)
	}
	if err := q.UpdateWebsiteSpec(t.Context(), dbpkg.WebsiteRow{
		ID:                 websiteRow.ID,
		DefaultStyleBundle: websiteRow.DefaultStyleBundle,
		BaseTemplate:       websiteRow.BaseTemplate,
		ContentHash:        websiteHash,
	}); err != nil {
		t.Fatalf("UpdateWebsiteSpec() error = %v", err)
	}
	if err := q.UpsertPage(t.Context(), dbpkg.PageRow{
		WebsiteID:   websiteRow.ID,
		Name:        "index",
		Route:       "/",
		Title:       "Home",
		Description: "Home page",
		LayoutJSON:  `[{"include":"header"}]`,
		ContentHash: pageHash,
	}); err != nil {
		t.Fatalf("UpsertPage() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/source")
	if err != nil {
		t.Fatalf("GET /source error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected source status 200, got %d body=%s", resp.StatusCode, string(body))
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar content %s: %v", hdr.Name, err)
		}
		files[hdr.Name] = content
	}
	if !bytes.Equal(files["website.yaml"], websiteBytes) {
		t.Fatalf("website.yaml was regenerated instead of preserved: %q", string(files["website.yaml"]))
	}
	if !bytes.Equal(files["pages/index.page.yaml"], pageBytes) {
		t.Fatalf("index.page.yaml was regenerated instead of preserved: %q", string(files["pages/index.page.yaml"]))
	}
}

func TestSourceEndpointRejectsWebsitesWithoutPages(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(t.Context(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if _, err := q.DeletePageByName(t.Context(), websiteRow.ID, "index"); err != nil {
		t.Fatalf("DeletePageByName() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/source")
	if err != nil {
		t.Fatalf("GET /source error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected source conflict 409, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestSourceEndpointRejectsMultipleStyleBundles(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(t.Context(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if err := q.UpsertStyleBundle(t.Context(), dbpkg.StyleBundleRow{
		WebsiteID: websiteRow.ID,
		Name:      "alt",
		FilesJSON: `[{"file":"styles/tokens.css","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},{"file":"styles/default.css","hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]`,
	}); err != nil {
		t.Fatalf("UpsertStyleBundle() error = %v", err)
	}

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/source")
	if err != nil {
		t.Fatalf("GET /source error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected source conflict 409, got %d body=%s", resp.StatusCode, string(body))
	}
}

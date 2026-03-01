package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/blob"
	"github.com/benedict2310/htmlctl/internal/bundle"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestApplyRejectsInvalidComponentAndPageNames(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		resourceKind string
		resourceName string
		filePath     string
		content      []byte
	}{
		{
			name:         "invalid component name",
			resourceKind: "Component",
			resourceName: "../evil",
			filePath:     "components/header.html",
			content:      []byte("<section id=\"header\">Header</section>"),
		},
		{
			name:         "invalid page name",
			resourceKind: "Page",
			resourceName: "home\nadmin",
			filePath:     "pages/index.page.yaml",
			content:      []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n"),
		},
		{
			name:         "invalid stylebundle name",
			resourceKind: "StyleBundle",
			resourceName: "../../evil",
			filePath:     "styles/default.css",
			content:      []byte("body { margin: 0; }"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
			defer db.Close()

			blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
			applier, err := NewApplier(db, blobStore)
			if err != nil {
				t.Fatalf("NewApplier() error = %v", err)
			}

			manifest := bundle.Manifest{
				Mode:    bundle.ApplyModePartial,
				Website: "sample",
				Resources: []bundle.Resource{
					{
						Kind: tc.resourceKind,
						Name: tc.resourceName,
						File: tc.filePath,
						Hash: "sha256:" + sha256Hex(tc.content),
					},
				},
			}
			b := bundle.Bundle{
				Manifest: manifest,
				Files: map[string][]byte{
					tc.filePath: tc.content,
				},
			}

			_, err = applier.Apply(ctx, "sample", "staging", b, false)
			if err == nil {
				t.Fatalf("Apply() expected error")
			}
			var badRequestErr *BadRequestError
			if !errors.As(err, &badRequestErr) {
				t.Fatalf("Apply() expected BadRequestError, got %T (%v)", err, err)
			}

			q := dbpkg.NewQueries(db)
			_, err = q.GetWebsiteByName(ctx, "sample")
			if !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("expected no committed website row after failed apply, got err=%v", err)
			}
		})
	}
}

func TestApplyPersistsPageHeadMetadata(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	pageYAML := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout: []
  head:
    canonicalURL: https://example.com/
    meta:
      robots: index,follow
    openGraph:
      title: Sample
    twitter:
      card: summary
    jsonLD:
      - id: website
        payload:
          "@context": https://schema.org
          "@type": WebSite
          name: Sample
`)

	manifest := bundle.Manifest{
		Mode:    bundle.ApplyModePartial,
		Website: "sample",
		Resources: []bundle.Resource{
			{
				Kind: "Page",
				Name: "index",
				File: "pages/index.page.yaml",
				Hash: "sha256:" + sha256Hex(pageYAML),
			},
		},
	}
	b := bundle.Bundle{
		Manifest: manifest,
		Files: map[string][]byte{
			"pages/index.page.yaml": pageYAML,
		},
	}

	if _, err := applier.Apply(ctx, "sample", "staging", b, false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	q := dbpkg.NewQueries(db)
	website, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	pages, err := q.ListPagesByWebsite(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListPagesByWebsite() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page row, got %d", len(pages))
	}
	if strings.TrimSpace(pages[0].HeadJSON) == "" || pages[0].HeadJSON == "{}" {
		t.Fatalf("expected persisted head_json, got %q", pages[0].HeadJSON)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(pages[0].HeadJSON), &got); err != nil {
		t.Fatalf("unmarshal head_json: %v", err)
	}
	if got["canonicalURL"] != "https://example.com/" {
		t.Fatalf("unexpected canonicalURL in head_json: %#v", got["canonicalURL"])
	}
}

func TestApplyDetectsHeadOnlyChanges(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	fixedHash := "sha256:" + strings.Repeat("a", 64)
	initial := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout: []
  head:
    canonicalURL: https://foo.example/
`)
	updated := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout: []
  head:
    canonicalURL: https://bar.example/
`)

	apply := func(content []byte) (ApplyResult, error) {
		return applier.Apply(ctx, "sample", "staging", bundle.Bundle{
			Manifest: bundle.Manifest{
				Mode:    bundle.ApplyModePartial,
				Website: "sample",
				Resources: []bundle.Resource{
					{
						Kind: "Page",
						Name: "index",
						File: "pages/index.page.yaml",
						Hash: fixedHash,
					},
				},
			},
			Files: map[string][]byte{
				"pages/index.page.yaml": content,
			},
		}, false)
	}

	first, err := apply(initial)
	if err != nil {
		t.Fatalf("first Apply() error = %v", err)
	}
	if first.Changes.Created != 1 || first.Changes.Updated != 0 {
		t.Fatalf("unexpected first apply changes: %#v", first.Changes)
	}

	second, err := apply(updated)
	if err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if second.Changes.Updated != 1 {
		t.Fatalf("expected head-only change to count as updated=1, got %#v", second.Changes)
	}

	q := dbpkg.NewQueries(db)
	website, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	pages, err := q.ListPagesByWebsite(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListPagesByWebsite() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page row, got %d", len(pages))
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(pages[0].HeadJSON), &got); err != nil {
		t.Fatalf("unmarshal updated head_json: %v", err)
	}
	if got["canonicalURL"] != "https://bar.example/" {
		t.Fatalf("expected updated canonicalURL, got %#v", got["canonicalURL"])
	}
}

func TestApplyPersistsWebsiteHeadAndIcons(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	websiteYAML := []byte(`apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
  seo:
    publicBaseURL: https://example.com/
    robots:
      enabled: true
      groups:
        - userAgents:
            - "*"
          disallow:
            - /drafts/
    sitemap:
      enabled: true
`)
	iconBytes := []byte("<svg></svg>\n")
	manifest := bundle.Manifest{
		Mode:    bundle.ApplyModePartial,
		Website: "sample",
		Resources: []bundle.Resource{
			{Kind: "Website", Name: "sample", File: "website.yaml", Hash: "sha256:" + sha256Hex(websiteYAML)},
			{Kind: "WebsiteIcon", Name: "website-icon-svg", File: "branding/favicon.svg", Hash: "sha256:" + sha256Hex(iconBytes), ContentType: "image/svg+xml"},
		},
	}
	b := bundle.Bundle{
		Manifest: manifest,
		Files: map[string][]byte{
			"website.yaml":         websiteYAML,
			"branding/favicon.svg": iconBytes,
		},
	}

	if _, err := applier.Apply(ctx, "sample", "staging", b, false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	q := dbpkg.NewQueries(db)
	website, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if website.ContentHash != "sha256:"+sha256Hex(websiteYAML) {
		t.Fatalf("unexpected website content hash: %q", website.ContentHash)
	}
	if !strings.Contains(website.HeadJSON, `"svg":"branding/favicon.svg"`) {
		t.Fatalf("unexpected website head_json: %q", website.HeadJSON)
	}
	if !strings.Contains(website.SEOJSON, `"publicBaseURL":"https://example.com/"`) {
		t.Fatalf("unexpected website seo_json: %q", website.SEOJSON)
	}
	if !strings.Contains(website.SEOJSON, `"disallow":["/drafts/"]`) {
		t.Fatalf("unexpected website seo_json: %q", website.SEOJSON)
	}
	if !strings.Contains(website.SEOJSON, `"sitemap":{"enabled":true}`) {
		t.Fatalf("unexpected website seo_json: %q", website.SEOJSON)
	}
	icons, err := q.ListWebsiteIconsByWebsite(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListWebsiteIconsByWebsite() error = %v", err)
	}
	if len(icons) != 1 || icons[0].Slot != "svg" || icons[0].SourcePath != "branding/favicon.svg" {
		t.Fatalf("unexpected website icons: %#v", icons)
	}
}

func TestApplyPreservesWebsiteAndIconsForOldManifest(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	q := dbpkg.NewQueries(db)
	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
		HeadJSON:           `{"icons":{"svg":"branding/favicon.svg"}}`,
		SEOJSON:            `{"publicBaseURL":"https://example.com","robots":{"enabled":true},"sitemap":{"enabled":true}}`,
		ContentHash:        "sha256:website",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"}); err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}
	if err := q.UpsertWebsiteIcon(ctx, dbpkg.WebsiteIconRow{
		WebsiteID:   websiteID,
		Slot:        "svg",
		SourcePath:  "branding/favicon.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   11,
		ContentHash: "sha256:icon",
	}); err != nil {
		t.Fatalf("UpsertWebsiteIcon() error = %v", err)
	}

	pageYAML := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home
  layout: []
`)
	manifest := bundle.Manifest{
		Mode:    bundle.ApplyModePartial,
		Website: "sample",
		Resources: []bundle.Resource{
			{Kind: "Page", Name: "index", File: "pages/index.page.yaml", Hash: "sha256:" + sha256Hex(pageYAML)},
		},
	}
	b := bundle.Bundle{
		Manifest: manifest,
		Files: map[string][]byte{
			"pages/index.page.yaml": pageYAML,
		},
	}
	if _, err := applier.Apply(ctx, "sample", "staging", b, false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	website, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if website.HeadJSON != `{"icons":{"svg":"branding/favicon.svg"}}` || website.SEOJSON != `{"publicBaseURL":"https://example.com","robots":{"enabled":true},"sitemap":{"enabled":true}}` || website.ContentHash != "sha256:website" {
		t.Fatalf("unexpected website row after old manifest apply: %#v", website)
	}
	icons, err := q.ListWebsiteIconsByWebsite(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListWebsiteIconsByWebsite() error = %v", err)
	}
	if len(icons) != 1 || icons[0].Slot != "svg" {
		t.Fatalf("expected preserved website icon state, got %#v", icons)
	}
}

func TestApplyWebsiteWithoutSEOBlockClearsStoredSEO(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	q := dbpkg.NewQueries(db)
	websiteID, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
		SEOJSON:            `{"publicBaseURL":"https://example.com","robots":{"enabled":true},"sitemap":{"enabled":true}}`,
		ContentHash:        "sha256:before",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	if _, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{WebsiteID: websiteID, Name: "staging"}); err != nil {
		t.Fatalf("InsertEnvironment() error = %v", err)
	}

	websiteYAML := []byte(`apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
`)
	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Mode:    bundle.ApplyModePartial,
			Website: "sample",
			Resources: []bundle.Resource{
				{Kind: "Website", Name: "sample", File: "website.yaml", Hash: "sha256:" + sha256Hex(websiteYAML)},
			},
		},
		Files: map[string][]byte{
			"website.yaml": websiteYAML,
		},
	}

	if _, err := applier.Apply(ctx, "sample", "staging", b, false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	row, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if row.SEOJSON != "{}" {
		t.Fatalf("expected cleared website seo_json, got %q", row.SEOJSON)
	}
}

func TestFullApplyRemovesStaleWebsiteIconsWhenPresent(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	websiteYAML := []byte(`apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
`)
	svg := []byte("<svg></svg>\n")
	ico := []byte("ico\n")
	initial := bundle.Bundle{
		Manifest: bundle.Manifest{
			Mode:    bundle.ApplyModePartial,
			Website: "sample",
			Resources: []bundle.Resource{
				{Kind: "Website", Name: "sample", File: "website.yaml", Hash: "sha256:" + sha256Hex(websiteYAML)},
				{Kind: "WebsiteIcon", Name: "website-icon-svg", File: "branding/favicon.svg", Hash: "sha256:" + sha256Hex(svg), ContentType: "image/svg+xml"},
				{Kind: "WebsiteIcon", Name: "website-icon-ico", File: "branding/favicon.ico", Hash: "sha256:" + sha256Hex(ico), ContentType: "image/x-icon"},
			},
		},
		Files: map[string][]byte{
			"website.yaml":         websiteYAML,
			"branding/favicon.svg": svg,
			"branding/favicon.ico": ico,
		},
	}
	if _, err := applier.Apply(ctx, "sample", "staging", initial, false); err != nil {
		t.Fatalf("initial Apply() error = %v", err)
	}

	full := bundle.Bundle{
		Manifest: bundle.Manifest{
			Mode:    bundle.ApplyModeFull,
			Website: "sample",
			Resources: []bundle.Resource{
				{Kind: "Website", Name: "sample", File: "website.yaml", Hash: "sha256:" + sha256Hex(websiteYAML)},
				{Kind: "WebsiteIcon", Name: "website-icon-svg", File: "branding/favicon.svg", Hash: "sha256:" + sha256Hex(svg), ContentType: "image/svg+xml"},
			},
		},
		Files: map[string][]byte{
			"website.yaml":         websiteYAML,
			"branding/favicon.svg": svg,
		},
	}
	if _, err := applier.Apply(ctx, "sample", "staging", full, false); err != nil {
		t.Fatalf("full Apply() error = %v", err)
	}

	q := dbpkg.NewQueries(db)
	website, err := q.GetWebsiteByName(ctx, "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	icons, err := q.ListWebsiteIconsByWebsite(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListWebsiteIconsByWebsite() error = %v", err)
	}
	if len(icons) != 1 || icons[0].Slot != "svg" {
		t.Fatalf("expected stale icon removal, got %#v", icons)
	}
}

func openStateTestDB(t *testing.T, path string) *sql.DB {
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

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

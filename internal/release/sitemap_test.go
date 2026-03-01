package release

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestGenerateSitemap(t *testing.T) {
	pages := map[string]model.Page{
		"index": {
			Metadata: model.Metadata{Name: "index"},
			Spec: model.PageSpec{
				Route: "/",
				Head: &model.PageHead{
					CanonicalURL: "https://example.com/?a=1&b=2",
					Meta:         map[string]string{"robots": "index, follow"},
				},
			},
		},
		"pricing": {
			Metadata: model.Metadata{Name: "pricing"},
			Spec: model.PageSpec{
				Route: "/pricing",
				Head: &model.PageHead{
					CanonicalURL: "/plans",
				},
			},
		},
		"drafts": {
			Metadata: model.Metadata{Name: "drafts"},
			Spec: model.PageSpec{
				Route: "/drafts",
				Head: &model.PageHead{
					Meta: map[string]string{"ROBOTS": "noindex"},
				},
			},
		},
		"staging": {
			Metadata: model.Metadata{Name: "staging"},
			Spec: model.PageSpec{
				Route: "/staging",
				Head: &model.PageHead{
					CanonicalURL: "https://staging.example.com/staging",
				},
			},
		},
	}

	log := newBuildLog()
	content, err := GenerateSitemap(&model.WebsiteSEO{
		PublicBaseURL: "https://example.com",
		Sitemap:       &model.WebsiteSitemap{Enabled: true},
	}, pages, log)
	if err != nil {
		t.Fatalf("GenerateSitemap() error = %v", err)
	}

	got := string(content)
	for _, want := range []string{
		xml.Header,
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
		`<loc>https://example.com/?a=1&amp;b=2</loc>`,
		`<loc>https://example.com/plans</loc>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected sitemap to contain %q, got %s", want, got)
		}
	}
	if strings.Contains(got, "/drafts") {
		t.Fatalf("expected noindex page to be excluded, got %s", got)
	}
	if strings.Contains(got, "staging.example.com") {
		t.Fatalf("expected cross-host canonical page to be excluded, got %s", got)
	}
	if strings.Index(got, "https://example.com/?a=1&amp;b=2") > strings.Index(got, "https://example.com/plans") {
		t.Fatalf("expected sitemap URLs to be sorted lexicographically, got %s", got)
	}
	if !strings.Contains(log.String(), `warning: page=staging skipped from sitemap because canonicalURL "https://staging.example.com/staging" does not match publicBaseURL "https://example.com/"`) {
		t.Fatalf("expected cross-host canonical warning, got %s", log.String())
	}

	type urlset struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	var parsed urlset
	if err := xml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("xml.Unmarshal() error = %v", err)
	}
	if len(parsed.URLs) != 2 {
		t.Fatalf("expected 2 sitemap URLs, got %#v", parsed.URLs)
	}
}

func TestGenerateSitemapRejectsCanonicalOutsidePublicBasePath(t *testing.T) {
	pages := map[string]model.Page{
		"docs-home": {
			Metadata: model.Metadata{Name: "docs-home"},
			Spec: model.PageSpec{
				Route: "/",
				Head: &model.PageHead{
					CanonicalURL: "https://example.com/docs/../other",
				},
			},
		},
		"docs-page": {
			Metadata: model.Metadata{Name: "docs-page"},
			Spec: model.PageSpec{
				Route: "/guide",
				Head: &model.PageHead{
					CanonicalURL: "/docs/guide",
				},
			},
		},
	}

	log := newBuildLog()
	content, err := GenerateSitemap(&model.WebsiteSEO{
		PublicBaseURL: "https://example.com/docs",
		Sitemap:       &model.WebsiteSitemap{Enabled: true},
	}, pages, log)
	if err != nil {
		t.Fatalf("GenerateSitemap() error = %v", err)
	}

	got := string(content)
	if strings.Contains(got, "https://example.com/other") {
		t.Fatalf("expected canonical outside publicBaseURL path to be excluded, got %s", got)
	}
	if !strings.Contains(got, `<loc>https://example.com/docs/guide</loc>`) {
		t.Fatalf("expected in-scope canonical to be included, got %s", got)
	}
	if !strings.Contains(log.String(), `warning: page=docs-home skipped from sitemap because canonicalURL "https://example.com/docs/../other" does not match publicBaseURL "https://example.com/docs"`) {
		t.Fatalf("expected out-of-scope canonical warning, got %s", log.String())
	}
}

func TestGenerateSitemapAcceptsDefaultPortEquivalentCanonical(t *testing.T) {
	pages := map[string]model.Page{
		"page": {
			Metadata: model.Metadata{Name: "page"},
			Spec: model.PageSpec{
				Route: "/page",
				Head: &model.PageHead{
					CanonicalURL: "https://example.com:443/page",
				},
			},
		},
	}

	content, err := GenerateSitemap(&model.WebsiteSEO{
		PublicBaseURL: "https://example.com",
		Sitemap:       &model.WebsiteSitemap{Enabled: true},
	}, pages, newBuildLog())
	if err != nil {
		t.Fatalf("GenerateSitemap() error = %v", err)
	}
	if !strings.Contains(string(content), `<loc>https://example.com:443/page</loc>`) {
		t.Fatalf("expected default-port canonical to be included, got %s", string(content))
	}
}

func TestGenerateSitemapDisabled(t *testing.T) {
	content, err := GenerateSitemap(&model.WebsiteSEO{
		PublicBaseURL: "https://example.com",
		Sitemap:       &model.WebsiteSitemap{Enabled: false},
	}, map[string]model.Page{}, newBuildLog())
	if err != nil {
		t.Fatalf("GenerateSitemap() error = %v", err)
	}
	if content != nil {
		t.Fatalf("expected nil sitemap content when disabled, got %q", string(content))
	}
}

func TestGenerateSitemapRequiresPublicBaseURL(t *testing.T) {
	_, err := GenerateSitemap(&model.WebsiteSEO{
		Sitemap: &model.WebsiteSitemap{Enabled: true},
	}, map[string]model.Page{}, newBuildLog())
	if err == nil {
		t.Fatalf("expected publicBaseURL validation error")
	}
	if !strings.Contains(err.Error(), "must be absolute") && !strings.Contains(err.Error(), "publicBaseURL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShouldExcludeFromSitemap(t *testing.T) {
	tests := []struct {
		robotsMeta string
		want       bool
	}{
		{robotsMeta: "", want: false},
		{robotsMeta: "index,follow", want: false},
		{robotsMeta: "none", want: true},
		{robotsMeta: "follow, NOINDEX", want: true},
		{robotsMeta: "x-noindexable", want: false},
	}

	for _, tc := range tests {
		if got := shouldExcludeFromSitemap(tc.robotsMeta); got != tc.want {
			t.Fatalf("shouldExcludeFromSitemap(%q) = %v, want %v", tc.robotsMeta, got, tc.want)
		}
	}
}

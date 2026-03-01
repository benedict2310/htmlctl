package loader

import (
	"fmt"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestNormalizeRoute(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{name: "root", in: "/", out: "/"},
		{name: "trim trailing slash", in: "/product/", out: "/product"},
		{name: "add leading slash", in: "pricing", out: "/pricing"},
		{name: "trim spaces", in: "  /faq/  ", out: "/faq"},
		{name: "empty", in: "", out: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeRoute(tc.in); got != tc.out {
				t.Fatalf("NormalizeRoute(%q) = %q, want %q", tc.in, got, tc.out)
			}
		})
	}
}

func TestValidateSiteRejectsDuplicateRoutes(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Components: map[string]model.Component{
			"header": {Name: "header", Scope: model.ComponentScopeGlobal, HTML: "<section></section>"},
		},
		Pages: map[string]model.Page{
			"home": {
				Metadata: model.Metadata{Name: "home"},
				Spec:     model.PageSpec{Route: "/", Layout: []model.PageLayoutItem{{Include: "header"}}},
			},
			"home-alt": {
				Metadata: model.Metadata{Name: "home-alt"},
				Spec:     model.PageSpec{Route: " / ", Layout: []model.PageLayoutItem{{Include: "header"}}},
			},
		},
	}

	err := ValidateSite(site)
	if err == nil {
		t.Fatalf("expected duplicate route validation error")
	}
	if !strings.Contains(err.Error(), "duplicate route") {
		t.Fatalf("expected duplicate route error, got %v", err)
	}
}

func TestValidateSiteRejectsMissingInclude(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec:     model.PageSpec{Route: "/", Layout: []model.PageLayoutItem{{Include: "missing"}}},
			},
		},
		Components: map[string]model.Component{},
	}

	err := ValidateSite(site)
	if err == nil {
		t.Fatalf("expected missing include validation error")
	}
	if !strings.Contains(err.Error(), "missing component") {
		t.Fatalf("expected missing component error, got %v", err)
	}
}

func TestValidateSiteNormalizesRoute(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Components: map[string]model.Component{
			"header": {Name: "header", Scope: model.ComponentScopeGlobal, HTML: "<section></section>"},
		},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec:     model.PageSpec{Route: " /about/ ", Layout: []model.PageLayoutItem{{Include: "header"}}},
			},
		},
	}

	if err := ValidateSite(site); err != nil {
		t.Fatalf("ValidateSite() error = %v", err)
	}

	if got := site.Pages["index"].Spec.Route; got != "/about" {
		t.Fatalf("route was not normalized, got %q", got)
	}
}

func TestValidateSiteRejectsUnsupportedHeadURLSchemes(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Components: map[string]model.Component{
			"header": {Name: "header", Scope: model.ComponentScopeGlobal, HTML: "<section></section>"},
		},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route:  "/",
					Layout: []model.PageLayoutItem{{Include: "header"}},
					Head: &model.PageHead{
						CanonicalURL: "javascript:alert(1)",
					},
				},
			},
		},
	}

	err := ValidateSite(site)
	if err == nil {
		t.Fatalf("expected head URL validation error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

func TestValidateSiteAllowsRelativeHeadURLs(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Components: map[string]model.Component{
			"header": {Name: "header", Scope: model.ComponentScopeGlobal, HTML: "<section></section>"},
		},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route:  "/",
					Layout: []model.PageLayoutItem{{Include: "header"}},
					Head: &model.PageHead{
						CanonicalURL: "/ora",
						OpenGraph: &model.OpenGraph{
							Image: "/assets/ora/og-image.jpg",
						},
						Twitter: &model.TwitterCard{
							Image: "/assets/ora/og-image.jpg",
						},
					},
				},
			},
		},
	}

	if err := ValidateSite(site); err != nil {
		t.Fatalf("ValidateSite() error = %v", err)
	}
}

func TestValidateSiteEnforcesHeadMetadataLimits(t *testing.T) {
	tooManyMeta := map[string]string{}
	for i := 0; i < maxHeadMetaEntries+1; i++ {
		tooManyMeta[fmt.Sprintf("key-%d", i)] = "v"
	}
	tooManyJSONLD := make([]model.JSONLDBlock, maxHeadJSONLDBlocks+1)
	for i := range tooManyJSONLD {
		tooManyJSONLD[i] = model.JSONLDBlock{
			ID:      "id",
			Payload: map[string]any{"@context": "https://schema.org"},
		}
	}

	tests := []struct {
		name      string
		head      *model.PageHead
		wantError string
	}{
		{
			name: "canonical too long",
			head: &model.PageHead{
				CanonicalURL: "https://example.com/" + strings.Repeat("a", maxHeadURLLength),
			},
			wantError: "canonicalURL longer",
		},
		{
			name: "too many meta entries",
			head: &model.PageHead{
				Meta: tooManyMeta,
			},
			wantError: "too many head.meta entries",
		},
		{
			name: "meta key too long",
			head: &model.PageHead{
				Meta: map[string]string{
					strings.Repeat("k", maxHeadMetaKeyLength+1): "value",
				},
			},
			wantError: "head.meta key longer",
		},
		{
			name: "meta value too long",
			head: &model.PageHead{
				Meta: map[string]string{
					"keywords": strings.Repeat("v", maxHeadMetaValueLength+1),
				},
			},
			wantError: "head.meta value longer",
		},
		{
			name: "open graph description too long",
			head: &model.PageHead{
				OpenGraph: &model.OpenGraph{
					Description: strings.Repeat("d", maxHeadTextFieldLength+1),
				},
			},
			wantError: "openGraph.description longer",
		},
		{
			name: "twitter image too long",
			head: &model.PageHead{
				Twitter: &model.TwitterCard{
					Image: "https://cdn.example.com/" + strings.Repeat("i", maxHeadURLLength),
				},
			},
			wantError: "twitter.image longer",
		},
		{
			name: "too many jsonld blocks",
			head: &model.PageHead{
				JSONLD: tooManyJSONLD,
			},
			wantError: "too many jsonLD blocks",
		},
		{
			name: "jsonld id too long",
			head: &model.PageHead{
				JSONLD: []model.JSONLDBlock{
					{
						ID:      strings.Repeat("x", maxHeadJSONLDIDLength+1),
						Payload: map[string]any{"@context": "https://schema.org"},
					},
				},
			},
			wantError: "jsonLD[0].id longer",
		},
		{
			name: "jsonld payload too large",
			head: &model.PageHead{
				JSONLD: []model.JSONLDBlock{
					{
						ID: "large",
						Payload: map[string]any{
							"data": strings.Repeat("x", maxHeadJSONLDPayloadBytes),
						},
					},
				},
			},
			wantError: "payload larger than",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			site := &model.Site{
				Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
				Components: map[string]model.Component{
					"header": {Name: "header", Scope: model.ComponentScopeGlobal, HTML: "<section></section>"},
				},
				Pages: map[string]model.Page{
					"index": {
						Metadata: model.Metadata{Name: "index"},
						Spec: model.PageSpec{
							Route:  "/",
							Layout: []model.PageLayoutItem{{Include: "header"}},
							Head:   tc.head,
						},
					},
				},
			}

			err := ValidateSite(site)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}

func TestValidateSiteNormalizesWebsiteSEO(t *testing.T) {
	site := &model.Site{
		Website: model.Website{
			Metadata: model.Metadata{Name: "sample"},
			Spec: model.WebsiteSpec{
				SEO: &model.WebsiteSEO{
					PublicBaseURL: " https://example.com/docs/ ",
					Robots: &model.WebsiteRobots{
						Enabled: true,
						Groups: []model.RobotsGroup{
							{
								UserAgents: []string{" * "},
								Allow:      []string{" / "},
								Disallow:   []string{" /drafts/ "},
							},
						},
					},
				},
			},
		},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec:     model.PageSpec{Route: "/", Layout: []model.PageLayoutItem{}},
			},
		},
		Components: map[string]model.Component{},
	}

	if err := ValidateSite(site); err != nil {
		t.Fatalf("ValidateSite() error = %v", err)
	}

	if got := site.Website.Spec.SEO.PublicBaseURL; got != "https://example.com/docs" {
		t.Fatalf("unexpected normalized publicBaseURL: %q", got)
	}
	group := site.Website.Spec.SEO.Robots.Groups[0]
	if len(group.UserAgents) != 1 || group.UserAgents[0] != "*" {
		t.Fatalf("unexpected normalized userAgents: %#v", group.UserAgents)
	}
	if len(group.Allow) != 1 || group.Allow[0] != "/" {
		t.Fatalf("unexpected normalized allow rules: %#v", group.Allow)
	}
	if len(group.Disallow) != 1 || group.Disallow[0] != "/drafts/" {
		t.Fatalf("unexpected normalized disallow rules: %#v", group.Disallow)
	}
}

func TestValidateSiteRejectsInvalidWebsiteSEO(t *testing.T) {
	tests := []struct {
		name      string
		seo       *model.WebsiteSEO
		wantError string
	}{
		{
			name: "relative public base url",
			seo: &model.WebsiteSEO{
				PublicBaseURL: "/docs",
			},
			wantError: "must be absolute",
		},
		{
			name: "public base url with query",
			seo: &model.WebsiteSEO{
				PublicBaseURL: "https://example.com/?q=1",
			},
			wantError: "must not include a query string",
		},
		{
			name: "robots group without user agent",
			seo: &model.WebsiteSEO{
				Robots: &model.WebsiteRobots{
					Enabled: true,
					Groups:  []model.RobotsGroup{{Allow: []string{"/"}}},
				},
			},
			wantError: "must include at least one userAgent",
		},
		{
			name: "robots rule missing leading slash",
			seo: &model.WebsiteSEO{
				Robots: &model.WebsiteRobots{
					Enabled: true,
					Groups: []model.RobotsGroup{{
						UserAgents: []string{"*"},
						Disallow:   []string{"drafts"},
					}},
				},
			},
			wantError: "must start with /",
		},
		{
			name: "robots user agent with newline",
			seo: &model.WebsiteSEO{
				Robots: &model.WebsiteRobots{
					Groups: []model.RobotsGroup{{
						UserAgents: []string{"Google\nbot"},
					}},
				},
			},
			wantError: "must not contain newlines",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			site := &model.Site{
				Website: model.Website{
					Metadata: model.Metadata{Name: "sample"},
					Spec:     model.WebsiteSpec{SEO: tc.seo},
				},
				Pages: map[string]model.Page{
					"index": {
						Metadata: model.Metadata{Name: "index"},
						Spec:     model.PageSpec{Route: "/", Layout: []model.PageLayoutItem{}},
					},
				},
				Components: map[string]model.Component{},
			}

			err := ValidateSite(site)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}

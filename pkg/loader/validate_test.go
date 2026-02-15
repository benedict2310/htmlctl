package loader

import (
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
		Website: model.Website{Metadata: model.Metadata{Name: "futurelab"}},
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
		Website: model.Website{Metadata: model.Metadata{Name: "futurelab"}},
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
		Website: model.Website{Metadata: model.Metadata{Name: "futurelab"}},
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

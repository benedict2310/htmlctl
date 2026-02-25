package validator

import (
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestValidateAllComponentsCollectsAllErrors(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{Route: "/", Layout: []model.PageLayoutItem{
					{Include: "header"},
					{Include: "hero"},
					{Include: "pricing"},
					{Include: "promo"},
					{Include: "footer"},
				}},
			},
		},
		Components: map[string]model.Component{
			"header":  {Name: "header", HTML: `<header id="header"></header>`},
			"hero":    {Name: "hero", HTML: `<section id="hero"></section>`},
			"pricing": {Name: "pricing", HTML: `<section><h2>Pricing</h2></section>`},
			"promo":   {Name: "promo", HTML: `<section id="promo"><script>bad()</script></section>`},
			"footer":  {Name: "footer", HTML: `<footer id="footer"></footer>`},
		},
	}

	errs := ValidateAllComponents(site)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %#v", len(errs), errs)
	}

	got := FormatErrors(errs)
	if !strings.Contains(got, "pricing") || !strings.Contains(got, "promo") {
		t.Fatalf("expected both invalid components in errors, got:\n%s", got)
	}
}

func TestValidateAllComponentsOnlyRequiresAnchorIDForUsedComponents(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec:     model.PageSpec{Route: "/", Layout: []model.PageLayoutItem{{Include: "header"}}},
			},
		},
		Components: map[string]model.Component{
			"header": {Name: "header", HTML: `<header id="header"></header>`},
			"draft":  {Name: "draft", HTML: `<section><h2>Draft</h2></section>`},
		},
	}

	errs := ValidateAllComponents(site)
	if len(errs) != 0 {
		t.Fatalf("expected no errors; unused components should not require anchor id, got %#v", errs)
	}
}

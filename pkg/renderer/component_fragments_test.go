package renderer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestRenderInjectsComponentSidecarsInLayoutOrderAndDedupes(t *testing.T) {
	site := &model.Site{
		RootDir: t.TempDir(),
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route:       "/",
					Title:       "Home",
					Description: "Home",
					Layout: []model.PageLayoutItem{
						{Include: "hero"},
						{Include: "cta"},
						{Include: "hero"},
					},
				},
			},
		},
		Components: map[string]model.Component{
			"hero": {Name: "hero", HTML: "<section id=\"hero\"></section>\n", CSS: "#hero { color: red; }\n", JS: "console.log('hero')\n"},
			"cta":  {Name: "cta", HTML: "<section id=\"cta\"></section>\n", CSS: "#cta { color: blue; }\n"},
		},
		Styles: model.StyleBundle{
			Name:       "default",
			TokensCSS:  ":root {}\n",
			DefaultCSS: "body {}\n",
		},
		ScriptPath: "scripts/site.js",
	}
	if err := os.MkdirAll(filepath.Join(site.RootDir, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(site.RootDir, "scripts", "site.js"), []byte("console.log('global')\n"), 0o644); err != nil {
		t.Fatalf("write site.js: %v", err)
	}

	outDir := t.TempDir()
	if err := Render(site, outDir); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	html := readFile(t, filepath.Join(outDir, "index.html"))
	heroCSS := regexp.MustCompile(`/components/hero-[a-f0-9]{12}\.css`)
	ctaCSS := regexp.MustCompile(`/components/cta-[a-f0-9]{12}\.css`)
	heroJS := regexp.MustCompile(`/components/hero-[a-f0-9]{12}\.js`)
	if !heroCSS.MatchString(html) || !ctaCSS.MatchString(html) || !heroJS.MatchString(html) {
		t.Fatalf("expected component sidecar paths in html:\n%s", html)
	}
	if strings.Count(html, "/components/hero-") != 2 {
		t.Fatalf("expected hero css + js references once each, got html:\n%s", html)
	}
	if strings.Count(html, "/components/cta-") != 1 {
		t.Fatalf("expected cta css reference once, got html:\n%s", html)
	}

	defaultIdx := strings.Index(html, "/styles/default-")
	heroCSSIdx := strings.Index(html, "/components/hero-")
	ctaCSSIdx := strings.LastIndex(html, "/components/cta-")
	globalJSIdx := strings.Index(html, "/scripts/site-")
	heroJSIdx := strings.LastIndex(html, ".js")
	if !(defaultIdx < heroCSSIdx && heroCSSIdx < ctaCSSIdx && globalJSIdx < heroJSIdx) {
		t.Fatalf("unexpected component sidecar ordering:\n%s", html)
	}
	if !strings.Contains(html, "defer></script>") {
		t.Fatalf("expected component js defer attribute, got html:\n%s", html)
	}
}

func TestRenderWithoutComponentSidecarsOmitsComponentAssets(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "sample"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route:       "/",
					Title:       "Home",
					Description: "Home",
					Layout:      []model.PageLayoutItem{{Include: "hero"}},
				},
			},
		},
		Components: map[string]model.Component{
			"hero": {Name: "hero", HTML: "<section id=\"hero\"></section>\n"},
		},
		Styles: model.StyleBundle{
			Name:       "default",
			TokensCSS:  ":root {}\n",
			DefaultCSS: "body {}\n",
		},
	}

	outDir := t.TempDir()
	if err := Render(site, outDir); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := readFile(t, filepath.Join(outDir, "index.html"))
	if strings.Contains(html, "/components/") {
		t.Fatalf("expected no component asset references, got html:\n%s", html)
	}
}

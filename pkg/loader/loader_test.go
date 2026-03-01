package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSiteValidSite(t *testing.T) {
	site, err := LoadSite(filepath.Join("..", "..", "testdata", "valid-site"))
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}

	if site.Website.Metadata.Name != "sample" {
		t.Fatalf("unexpected website name: %q", site.Website.Metadata.Name)
	}

	page, ok := site.Pages["index"]
	if !ok {
		t.Fatalf("expected page 'index' to be present")
	}
	if page.Spec.Route != "/" {
		t.Fatalf("unexpected normalized route: %q", page.Spec.Route)
	}
	if len(page.Spec.Layout) != 1 || page.Spec.Layout[0].Include != "header" {
		t.Fatalf("unexpected page layout: %#v", page.Spec.Layout)
	}
	ora, ok := site.Pages["ora"]
	if !ok {
		t.Fatalf("expected page 'ora' to be present")
	}
	if ora.Spec.Head == nil || ora.Spec.Head.CanonicalURL != "https://example.com/ora" {
		t.Fatalf("expected ora head metadata to be parsed, got %#v", ora.Spec.Head)
	}

	component, ok := site.Components["header"]
	if !ok {
		t.Fatalf("expected component 'header' to be present")
	}
	if component.Name != "header" {
		t.Fatalf("unexpected component name: %q", component.Name)
	}
	if component.Scope == "" {
		t.Fatalf("expected component scope default to be set")
	}
	if !strings.Contains(component.HTML, "<h1>Hello</h1>") {
		t.Fatalf("component HTML not loaded")
	}

	if site.Styles.Name != "default" {
		t.Fatalf("unexpected style bundle name: %q", site.Styles.Name)
	}
	if !strings.Contains(site.Styles.TokensCSS, "--bg") {
		t.Fatalf("tokens.css not loaded")
	}
	if !strings.Contains(site.Styles.DefaultCSS, "background") {
		t.Fatalf("default.css not loaded")
	}

	if site.ScriptPath != "scripts/site.js" {
		t.Fatalf("unexpected script path: %q", site.ScriptPath)
	}
	if len(site.Assets) != 1 || site.Assets[0].Name != "logo.svg" {
		t.Fatalf("unexpected assets: %#v", site.Assets)
	}
}

func TestLoadSiteMissingWebsiteYAML(t *testing.T) {
	_, err := LoadSite(filepath.Join("..", "..", "testdata", "missing-website-yaml"))
	if err == nil {
		t.Fatalf("expected error for missing website.yaml")
	}
	if !strings.Contains(err.Error(), "website.yaml") {
		t.Fatalf("expected error to mention website.yaml, got %v", err)
	}
}

func TestLoadSiteMissingReferencedComponent(t *testing.T) {
	_, err := LoadSite(filepath.Join("..", "..", "testdata", "missing-component"))
	if err == nil {
		t.Fatalf("expected error for missing referenced component")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("expected error to mention missing component, got %v", err)
	}
}

func TestLoadSiteMalformedYAMLIncludesFilename(t *testing.T) {
	_, err := LoadSite(filepath.Join("..", "..", "testdata", "malformed-yaml"))
	if err == nil {
		t.Fatalf("expected error for malformed yaml")
	}
	if !strings.Contains(err.Error(), "website.yaml") {
		t.Fatalf("expected parse error to include filename, got %v", err)
	}
}

func TestLoadSiteOptionalScriptMissing(t *testing.T) {
	root := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "testdata", "valid-site"), root)

	if err := os.Remove(filepath.Join(root, "scripts", "site.js")); err != nil {
		t.Fatalf("remove script file: %v", err)
	}

	site, err := LoadSite(root)
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}
	if site.ScriptPath != "" {
		t.Fatalf("expected no script path when scripts/site.js is absent, got %q", site.ScriptPath)
	}
}

func TestLoadSiteWebsiteBrandingIcons(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"pages", "styles", "branding"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	files := map[string]string{
		"website.yaml": `apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
      ico: branding/favicon.ico
      appleTouch: branding/apple-touch-icon.png
  seo:
    publicBaseURL: https://example.com/
    robots:
      enabled: true
      groups:
        - userAgents:
            - "*"
          allow:
            - /
          disallow:
            - /drafts/
`,
		"pages/index.page.yaml": `apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home
  layout: []
`,
		"styles/tokens.css":             ":root {}\n",
		"styles/default.css":            "body {}\n",
		"branding/favicon.svg":          "<svg></svg>\n",
		"branding/favicon.ico":          "ico\n",
		"branding/apple-touch-icon.png": "png\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	site, err := LoadSite(root)
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}
	if len(site.Branding) != 3 {
		t.Fatalf("expected 3 branding assets, got %#v", site.Branding)
	}
	if site.Branding["svg"].SourcePath != "branding/favicon.svg" {
		t.Fatalf("unexpected svg branding path: %#v", site.Branding["svg"])
	}
	if site.Branding["ico"].SourcePath != "branding/favicon.ico" {
		t.Fatalf("unexpected ico branding path: %#v", site.Branding["ico"])
	}
	if site.Branding["apple_touch"].SourcePath != "branding/apple-touch-icon.png" {
		t.Fatalf("unexpected apple touch branding path: %#v", site.Branding["apple_touch"])
	}
	if site.Website.Spec.SEO == nil || site.Website.Spec.SEO.Robots == nil {
		t.Fatalf("expected website seo to be loaded")
	}
	if got := site.Website.Spec.SEO.PublicBaseURL; got != "https://example.com/" {
		t.Fatalf("unexpected publicBaseURL: %q", got)
	}
	if !site.Website.Spec.SEO.Robots.Enabled {
		t.Fatalf("expected robots to be enabled")
	}
}

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
}

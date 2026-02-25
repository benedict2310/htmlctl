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

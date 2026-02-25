package bundle

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildTarFromDirProducesValidBundle(t *testing.T) {
	siteDir := writeSiteFixture(t)

	archive, manifest, err := BuildTarFromDir(siteDir, "sample")
	if err != nil {
		t.Fatalf("BuildTarFromDir() error = %v", err)
	}
	if manifest.Mode != ApplyModeFull {
		t.Fatalf("expected full mode, got %q", manifest.Mode)
	}
	if manifest.Website != "sample" {
		t.Fatalf("expected sample website, got %q", manifest.Website)
	}
	if len(manifest.Resources) != 6 {
		t.Fatalf("expected 6 resources, got %d", len(manifest.Resources))
	}

	b, err := ReadTar(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("ReadTar() error = %v", err)
	}
	if got := len(b.Files); got != 7 {
		t.Fatalf("expected 7 files in bundle, got %d", got)
	}
	for _, name := range []string{
		"components/header.html",
		"pages/index.page.yaml",
		"styles/default.css",
		"styles/tokens.css",
		"scripts/site.js",
		"assets/logo.svg",
		"assets/icons/check.svg",
	} {
		if _, ok := b.Files[name]; !ok {
			t.Fatalf("expected bundle file %q", name)
		}
	}
}

func TestBuildTarFromDirMissingWebsiteYAMLFails(t *testing.T) {
	siteDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(siteDir, "pages"), 0o755); err != nil {
		t.Fatalf("mkdir pages: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "styles"), 0o755); err != nil {
		t.Fatalf("mkdir styles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "pages", "index.page.yaml"), []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  description: Home\n  layout: []\n"), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "styles", "tokens.css"), []byte(":root{}"), 0o644); err != nil {
		t.Fatalf("write tokens.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "styles", "default.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write default.css: %v", err)
	}

	_, _, err := BuildTarFromDir(siteDir, "sample")
	if err == nil {
		t.Fatalf("expected missing website.yaml error")
	}
}

func TestBuildTarFromDirWebsiteNameMismatchFails(t *testing.T) {
	siteDir := writeSiteFixture(t)
	_, _, err := BuildTarFromDir(siteDir, "another-site")
	if err == nil {
		t.Fatalf("expected website mismatch error")
	}
}

func TestContentTypeForPathFallbacks(t *testing.T) {
	if got := contentTypeForPath("styles/default.css"); got != "text/css; charset=utf-8" {
		t.Fatalf("unexpected css content type %q", got)
	}
	if got := contentTypeForPath("scripts/site.js"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("unexpected js content type %q", got)
	}
	if got := contentTypeForPath("assets/logo.svg"); got != "image/svg+xml" {
		t.Fatalf("unexpected svg content type %q", got)
	}
	if got := contentTypeForPath("assets/archive.unknown"); got != "application/octet-stream" {
		t.Fatalf("unexpected default content type %q", got)
	}
}

func writeSiteFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{
		"components",
		"pages",
		"styles",
		"scripts",
		"assets/icons",
	} {
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
`,
		"components/header.html": "<section id=\"header\">Header</section>\n",
		"pages/index.page.yaml": `apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout:
    - include: header
`,
		"styles/tokens.css":      ":root { --brand: #00f; }\n",
		"styles/default.css":     "body { margin: 0; }\n",
		"scripts/site.js":        "console.log('ok');\n",
		"assets/logo.svg":        "<svg></svg>\n",
		"assets/icons/check.svg": "<svg></svg>\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

package bundle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
	if len(manifest.Resources) != 7 {
		t.Fatalf("expected 7 resources, got %d", len(manifest.Resources))
	}

	b, err := ReadTar(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("ReadTar() error = %v", err)
	}
	if got := len(b.Files); got != 8 {
		t.Fatalf("expected 8 files in bundle, got %d", got)
	}
	for _, name := range []string{
		"website.yaml",
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

func TestBuildTarFromDirWithSourceMetadata(t *testing.T) {
	siteDir := writeSiteFixture(t)

	_, manifest, err := BuildTarFromDirWithOptions(siteDir, "sample", BuildOptions{
		Source: &Source{
			Type:   "git",
			Repo:   "git@github.com:org/repo.git",
			Ref:    "01abcdef",
			Subdir: "site",
		},
	})
	if err != nil {
		t.Fatalf("BuildTarFromDirWithOptions() error = %v", err)
	}
	if manifest.Source == nil || manifest.Source.Type != "git" || manifest.Source.Ref != "01abcdef" {
		t.Fatalf("unexpected source metadata %#v", manifest.Source)
	}
}

func TestBuildTarFromDirIncludesComponentSidecars(t *testing.T) {
	siteDir := writeSiteFixture(t)
	if err := os.WriteFile(filepath.Join(siteDir, "components", "header.css"), []byte("#header { color: red; }\n"), 0o644); err != nil {
		t.Fatalf("write header.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "components", "header.js"), []byte("console.log('header');\n"), 0o644); err != nil {
		t.Fatalf("write header.js: %v", err)
	}

	archive, manifest, err := BuildTarFromDir(siteDir, "sample")
	if err != nil {
		t.Fatalf("BuildTarFromDir() error = %v", err)
	}
	b, err := ReadTar(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("ReadTar() error = %v", err)
	}
	for _, name := range []string{"components/header.html", "components/header.css", "components/header.js"} {
		if _, ok := b.Files[name]; !ok {
			t.Fatalf("expected bundle file %q", name)
		}
	}
	for _, resource := range manifest.Resources {
		if resource.Kind != "Component" || resource.Name != "header" {
			continue
		}
		if len(resource.FileEntries()) != 3 {
			t.Fatalf("expected component sidecars in manifest, got %#v", resource.FileEntries())
		}
		return
	}
	t.Fatalf("expected header component resource in manifest")
}

func TestBuildTarFromPathBuildsPartialComponentResourceFromSidecarFile(t *testing.T) {
	siteDir := writeSiteFixture(t)
	if err := os.WriteFile(filepath.Join(siteDir, "components", "header.css"), []byte("#header { color: red; }\n"), 0o644); err != nil {
		t.Fatalf("write header.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "components", "header.js"), []byte("console.log('header');\n"), 0o644); err != nil {
		t.Fatalf("write header.js: %v", err)
	}

	archive, manifest, err := BuildTarFromPath(filepath.Join(siteDir, "components", "header.css"), "sample")
	if err != nil {
		t.Fatalf("BuildTarFromPath() error = %v", err)
	}
	if manifest.Mode != ApplyModePartial {
		t.Fatalf("expected partial mode, got %q", manifest.Mode)
	}
	if len(manifest.Resources) != 1 {
		t.Fatalf("expected one partial resource, got %d", len(manifest.Resources))
	}
	resource := manifest.Resources[0]
	if resource.Kind != "Component" || resource.Name != "header" {
		t.Fatalf("unexpected partial resource %#v", resource)
	}
	if len(resource.FileEntries()) != 3 {
		t.Fatalf("expected owning component files, got %#v", resource.FileEntries())
	}

	b, err := ReadTar(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("ReadTar() error = %v", err)
	}
	if got := len(b.Files); got != 3 {
		t.Fatalf("expected 3 component files in partial bundle, got %d", got)
	}
}

func TestBuildTarFromPathRejectsNestedComponentPaths(t *testing.T) {
	siteDir := writeSiteFixture(t)
	nestedDir := filepath.Join(siteDir, "components", "tmp")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested component dir: %v", err)
	}
	nestedPath := filepath.Join(nestedDir, "header.css")
	if err := os.WriteFile(nestedPath, []byte("#header { color: red; }\n"), 0o644); err != nil {
		t.Fatalf("write nested component file: %v", err)
	}

	if _, _, err := BuildTarFromPath(nestedPath, "sample"); err == nil {
		t.Fatalf("BuildTarFromPath() expected error")
	} else if !strings.Contains(err.Error(), "unsupported component path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTarFromPathPageIncludesReferencedComponents(t *testing.T) {
	siteDir := writeSiteFixture(t)
	if err := os.WriteFile(filepath.Join(siteDir, "components", "header.css"), []byte("#header { color: red; }\n"), 0o644); err != nil {
		t.Fatalf("write header.css: %v", err)
	}

	archive, manifest, err := BuildTarFromPath(filepath.Join(siteDir, "pages", "index.page.yaml"), "sample")
	if err != nil {
		t.Fatalf("BuildTarFromPath() error = %v", err)
	}
	if manifest.Mode != ApplyModePartial {
		t.Fatalf("expected partial mode, got %q", manifest.Mode)
	}
	if len(manifest.Resources) != 2 {
		t.Fatalf("expected page + component resources, got %#v", manifest.Resources)
	}
	if manifest.Resources[0].Kind != "Page" || manifest.Resources[1].Kind != "Component" {
		t.Fatalf("unexpected resource order %#v", manifest.Resources)
	}

	b, err := ReadTar(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("ReadTar() error = %v", err)
	}
	for _, name := range []string{"pages/index.page.yaml", "components/header.html", "components/header.css"} {
		if _, ok := b.Files[name]; !ok {
			t.Fatalf("expected bundled dependency file %q", name)
		}
	}
}

func TestBuildTarFromDirIncludesWebsiteIcons(t *testing.T) {
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
		"styles/tokens.css":    ":root{}\n",
		"styles/default.css":   "body{}\n",
		"branding/favicon.svg": "<svg></svg>\n",
		"branding/favicon.ico": "ico\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	_, manifest, err := BuildTarFromDir(root, "sample")
	if err != nil {
		t.Fatalf("BuildTarFromDir() error = %v", err)
	}
	seen := map[string]bool{}
	for _, res := range manifest.Resources {
		seen[strings.ToLower(res.Kind)+":"+res.Name+":"+res.File] = true
	}
	for _, want := range []string{
		"website:sample:website.yaml",
		"websiteicon:website-icon-svg:branding/favicon.svg",
		"websiteicon:website-icon-ico:branding/favicon.ico",
	} {
		if !seen[want] {
			t.Fatalf("missing expected resource %q in %#v", want, manifest.Resources)
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

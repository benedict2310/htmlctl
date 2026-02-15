package renderer

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestRenderProducesExpectedStructureAndPaths(t *testing.T) {
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "valid-site"), root)

	productPage := `apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: product
spec:
  route: /product
  title: Product
  description: Product page
  layout:
    - include: header
`
	if err := os.WriteFile(filepath.Join(root, "pages", "product.page.yaml"), []byte(productPage), 0o644); err != nil {
		t.Fatalf("write product page: %v", err)
	}

	site, err := loader.LoadSite(root)
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}

	outDir := t.TempDir()
	stale := filepath.Join(outDir, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if err := Render(site, outDir); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed before render")
	}

	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Fatalf("missing root index.html: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "product", "index.html")); err != nil {
		t.Fatalf("missing /product/index.html: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "assets", "logo.svg")); err != nil {
		t.Fatalf("missing original-path asset file: %v", err)
	}

	rootHTML := readFile(t, filepath.Join(outDir, "index.html"))
	if !strings.Contains(rootHTML, "<!DOCTYPE html>") || !strings.Contains(rootHTML, "<main>") {
		t.Fatalf("unexpected root html structure")
	}
	if !strings.Contains(rootHTML, "<title>Futurelab</title>") {
		t.Fatalf("expected title tag in output html")
	}
	if !strings.Contains(rootHTML, `meta name="description" content="Landing page"`) {
		t.Fatalf("expected description meta tag in output html")
	}

	styleRe := regexp.MustCompile(`/styles/tokens-[a-f0-9]{12}\.css`)
	if !styleRe.MatchString(rootHTML) {
		t.Fatalf("expected tokens css hashed link in html")
	}
	defaultRe := regexp.MustCompile(`/styles/default-[a-f0-9]{12}\.css`)
	if !defaultRe.MatchString(rootHTML) {
		t.Fatalf("expected default css hashed link in html")
	}
	scriptRe := regexp.MustCompile(`/scripts/site-[a-f0-9]{12}\.js`)
	if !scriptRe.MatchString(rootHTML) {
		t.Fatalf("expected script hashed src in html")
	}

	tokensIdx := strings.Index(rootHTML, "/styles/tokens-")
	defaultIdx := strings.Index(rootHTML, "/styles/default-")
	if tokensIdx < 0 || defaultIdx < 0 || tokensIdx > defaultIdx {
		t.Fatalf("expected tokens.css link before default.css")
	}

	if strings.Contains(rootHTML, "\r") {
		t.Fatalf("expected LF line endings only in rendered html")
	}
}

func TestRenderOmitsScriptWhenNotPresent(t *testing.T) {
	site, err := loader.LoadSite(filepath.Join("..", "..", "testdata", "site-no-scripts"))
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}

	outDir := t.TempDir()
	if err := Render(site, outDir); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	html := readFile(t, filepath.Join(outDir, "index.html"))
	if strings.Contains(html, "<script ") {
		t.Fatalf("expected no script tag when site.js is absent")
	}
}

func TestRenderDeterministicAcrossRuns(t *testing.T) {
	site, err := loader.LoadSite(filepath.Join("..", "..", "testdata", "valid-site"))
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}

	outA := t.TempDir()
	outB := t.TempDir()

	if err := Render(site, outA); err != nil {
		t.Fatalf("Render(outA) error = %v", err)
	}
	if err := Render(site, outB); err != nil {
		t.Fatalf("Render(outB) error = %v", err)
	}

	filesA := collectFiles(t, outA)
	filesB := collectFiles(t, outB)

	if len(filesA) != len(filesB) {
		t.Fatalf("different file counts: %d vs %d", len(filesA), len(filesB))
	}
	for i := range filesA {
		if filesA[i] != filesB[i] {
			t.Fatalf("different file paths: %q vs %q", filesA[i], filesB[i])
		}
		aBytes := mustReadBytes(t, filepath.Join(outA, filesA[i]))
		bBytes := mustReadBytes(t, filepath.Join(outB, filesB[i]))
		if !bytes.Equal(aBytes, bBytes) {
			t.Fatalf("file %q differs between renders", filesA[i])
		}
	}
}

func TestStitchComponentsPreservesLayoutOrder(t *testing.T) {
	content, err := stitchComponents(
		[]model.PageLayoutItem{
			{Include: "header"},
			{Include: "hero"},
			{Include: "footer"},
		},
		map[string]model.Component{
			"header": {Name: "header", HTML: "<section id=\"header\"></section>\n"},
			"hero":   {Name: "hero", HTML: "<section id=\"hero\"></section>\n"},
			"footer": {Name: "footer", HTML: "<footer id=\"footer\"></footer>\n"},
		},
	)
	if err != nil {
		t.Fatalf("stitchComponents() error = %v", err)
	}

	expected := "<section id=\"header\"></section>\n<section id=\"hero\"></section>\n<footer id=\"footer\"></footer>\n"
	if content != expected {
		t.Fatalf("unexpected component stitching order:\n%s", content)
	}
}

func TestRouteToOutputPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "/", want: "index.html"},
		{in: "/product", want: "product/index.html"},
		{in: "pricing/", want: "pricing/index.html"},
	}

	for _, tc := range tests {
		got, err := routeToOutputPath(tc.in)
		if err != nil {
			t.Fatalf("routeToOutputPath(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("routeToOutputPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderErrorBranches(t *testing.T) {
	if err := Render(nil, t.TempDir()); err == nil {
		t.Fatalf("expected error for nil site")
	}
	if err := Render(&model.Site{}, ""); err == nil {
		t.Fatalf("expected error for empty output dir")
	}
}

func TestRenderFailsWhenComponentMissing(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "x"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route:  "/",
					Layout: []model.PageLayoutItem{{Include: "missing"}},
				},
			},
		},
		Components: map[string]model.Component{},
		Styles: model.StyleBundle{
			Name:       "default",
			TokensCSS:  ":root{}",
			DefaultCSS: "body{}",
		},
	}
	err := Render(site, t.TempDir())
	if err == nil {
		t.Fatalf("expected missing component error")
	}
}

func TestRenderFailsWhenRouteEmpty(t *testing.T) {
	site := &model.Site{
		Website: model.Website{Metadata: model.Metadata{Name: "x"}},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route:  "",
					Layout: []model.PageLayoutItem{{Include: "header"}},
				},
			},
		},
		Components: map[string]model.Component{
			"header": {Name: "header", HTML: "<section></section>"},
		},
		Styles: model.StyleBundle{
			Name:       "default",
			TokensCSS:  ":root{}",
			DefaultCSS: "body{}",
		},
	}
	err := Render(site, t.TempDir())
	if err == nil {
		t.Fatalf("expected empty route error")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b := mustReadBytes(t, path)
	return string(b)
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return b
}

func collectFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk files: %v", err)
	}
	sort.Strings(files)
	return files
}

func copyDir(t *testing.T, src, dst string) {
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}
}

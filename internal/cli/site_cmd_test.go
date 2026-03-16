package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteExplainStructuredOutput(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"site", "explain", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"supportedApplyPaths": [`) || !strings.Contains(out.String(), `components/\u003cname\u003e.css`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestSiteInitCreatesRenderableMinimalSite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "futurelab")

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"site", "init", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("site init error = %v", err)
	}

	for _, rel := range []string{
		"website.yaml",
		"pages/index.page.yaml",
		"components/hero.html",
		"styles/tokens.css",
		"styles/default.css",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}

	renderOut := filepath.Join(root, "dist")
	cmd = NewRootCmd("test")
	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"render", "-f", root, "-o", renderOut})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render error = %v; stderr=%s", err, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(renderOut, "index.html")); err != nil {
		t.Fatalf("expected rendered index.html: %v", err)
	}
}

func TestSiteInitRejectsNonEmptyTargetWithoutForce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"site", "init", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected non-empty target error")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSiteExportWritesRenderableSite(t *testing.T) {
	exportRoot := filepath.Join(t.TempDir(), "exported")
	archive := testSiteArchive(t, map[string]string{
		"website.yaml":          "apiVersion: htmlctl.dev/v1\nkind: Website\nmetadata:\n  name: sample\nspec:\n  defaultStyleBundle: default\n  baseTemplate: default\n",
		"pages/index.page.yaml": "apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  description: Home page\n  layout:\n    - include: hero\n",
		"components/hero.html":  "<section id=\"hero\"><h1>Home</h1></section>\n",
		"styles/tokens.css":     ":root { --brand: #111; }\n",
		"styles/default.css":    "body { margin: 0; }\n",
	})
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments/staging/source" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archive)),
				Header:     make(http.Header),
			}, nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"site", "export", "-o", exportRoot}, tr)
	if err != nil {
		t.Fatalf("site export error = %v", err)
	}
	if !strings.Contains(out, "regenerated canonically") {
		t.Fatalf("unexpected export output: %s", out)
	}

	renderOut := filepath.Join(t.TempDir(), "dist")
	cmd := NewRootCmd("test")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"render", "-f", exportRoot, "-o", renderOut})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render exported site error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "website.yaml")); err != nil {
		t.Fatalf("expected exported website.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, exportMarkerFile)); err != nil {
		t.Fatalf("expected export marker: %v", err)
	}
}

func TestSiteExportRejectsNonEmptyDirectoryWithoutForce(t *testing.T) {
	exportRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(exportRoot, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(testSiteArchive(t, map[string]string{}))),
				Header:     make(http.Header),
			}, nil
		},
	}

	_, _, err := runCommandWithTransport(t, []string{"site", "export", "-o", exportRoot}, tr)
	if err == nil {
		t.Fatalf("expected non-empty directory error")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSiteExportForceReplacesExistingDirectoryContents(t *testing.T) {
	exportRoot := t.TempDir()
	stalePath := filepath.Join(exportRoot, "components", "stale.html")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("create stale parent: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(exportRoot, exportMarkerFile), []byte("website=sample\nenvironment=staging\n"), 0o644); err != nil {
		t.Fatalf("write export marker: %v", err)
	}

	archive := testSiteArchive(t, map[string]string{
		"website.yaml":          "apiVersion: htmlctl.dev/v1\nkind: Website\nmetadata:\n  name: sample\nspec:\n  defaultStyleBundle: default\n  baseTemplate: default\n",
		"pages/index.page.yaml": "apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  layout:\n    - include: hero\n",
		"components/hero.html":  "<section id=\"hero\"><h1>Home</h1></section>\n",
		"styles/tokens.css":     ":root { --brand: #111; }\n",
		"styles/default.css":    "body { margin: 0; }\n",
	})
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archive)),
				Header:     make(http.Header),
			}, nil
		},
	}

	if _, _, err := runCommandWithTransport(t, []string{"site", "export", "-o", exportRoot, "--force"}, tr); err != nil {
		t.Fatalf("site export --force error = %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale file removal, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "components", "hero.html")); err != nil {
		t.Fatalf("expected fresh export file: %v", err)
	}
}

func TestSiteExportForceRejectsNonExportDirectory(t *testing.T) {
	exportRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(exportRoot, "notes.txt"), []byte("not an export"), 0o644); err != nil {
		t.Fatalf("write non-export file: %v", err)
	}
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			t.Fatalf("unexpected transport call for unsafe force target")
			return nil, nil
		},
	}

	_, _, err := runCommandWithTransport(t, []string{"site", "export", "-o", exportRoot, "--force"}, tr)
	if err == nil {
		t.Fatalf("expected unsafe force target error")
	}
	if !strings.Contains(err.Error(), "not a previous htmlctl export") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSiteExportArchiveInsideOutputDirectory(t *testing.T) {
	exportRoot := filepath.Join(t.TempDir(), "exported")
	archivePath := filepath.Join(exportRoot, "source.tar.gz")
	archive := testSiteArchive(t, map[string]string{
		"website.yaml":          "apiVersion: htmlctl.dev/v1\nkind: Website\nmetadata:\n  name: sample\nspec:\n  defaultStyleBundle: default\n  baseTemplate: default\n",
		"pages/index.page.yaml": "apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  layout:\n    - include: hero\n",
		"components/hero.html":  "<section id=\"hero\"><h1>Home</h1></section>\n",
		"styles/tokens.css":     ":root { --brand: #111; }\n",
		"styles/default.css":    "body { margin: 0; }\n",
	})
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archive)),
				Header:     make(http.Header),
			}, nil
		},
	}

	if _, _, err := runCommandWithTransport(t, []string{"site", "export", "-o", exportRoot, "--archive", archivePath}, tr); err != nil {
		t.Fatalf("site export with nested archive error = %v", err)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "website.yaml")); err != nil {
		t.Fatalf("expected exported site file: %v", err)
	}
}

func TestSiteExportArchiveOnlyWritesExactArchiveBytes(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "source.tar.gz")
	archive := testSiteArchive(t, map[string]string{
		"website.yaml": "apiVersion: htmlctl.dev/v1\nkind: Website\nmetadata:\n  name: sample\nspec:\n  defaultStyleBundle: default\n  baseTemplate: default\n",
	})
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archive)),
				Header:     make(http.Header),
			}, nil
		},
	}

	if _, _, err := runCommandWithTransport(t, []string{"site", "export", "--archive", archivePath}, tr); err != nil {
		t.Fatalf("site export archive-only error = %v", err)
	}
	got, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, archive) {
		t.Fatalf("archive file bytes were not preserved")
	}
}

func TestSiteExportRejectsUnexpectedPositionalArgs(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			t.Fatalf("unexpected transport call for invalid args")
			return nil, nil
		},
	}

	_, _, err := runCommandWithTransport(t, []string{"site", "export", "./site"}, tr)
	if err == nil {
		t.Fatalf("expected positional arg validation error")
	}
	if !strings.Contains(err.Error(), `unknown command "./site"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSafeArchiveRelPathRejectsWindowsAbsolutePath(t *testing.T) {
	if _, err := safeArchiveRelPath("C:/Users/Public/pwned.txt"); err == nil {
		t.Fatalf("expected windows absolute path rejection")
	}
}

func testSiteArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for path, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: path, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatalf("write header %s: %v", path, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write content %s: %v", path, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

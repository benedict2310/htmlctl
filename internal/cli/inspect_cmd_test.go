package cli

import (
	"net/http"
	"strings"
	"testing"
)

const inspectResourcesJSON = `{
  "website":"sample",
  "environment":"staging",
  "site":{"name":"sample","defaultStyleBundle":"default","baseTemplate":"default","seo":{"publicBaseURL":"https://example.com/","robots":{"enabled":true}},"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"},
  "pages":[{"name":"index","route":"/","title":"Home","description":"Home page","layout":[{"include":"header"},{"include":"hero"}],"head":{"canonicalURL":"https://example.com/","openGraph":{"title":"Home OG"},"twitter":{"card":"summary_large_image"}},"contentHash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}],
  "components":[
    {"name":"header","scope":"global","hasCss":true,"hasJs":true,"contentHash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","cssHash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","jsHash":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"},
    {"name":"hero","scope":"global","hasCss":false,"hasJs":false,"contentHash":"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}
  ],
  "styles":[{"name":"default","files":[{"path":"styles/tokens.css","hash":"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},{"path":"styles/default.css","hash":"sha256:1212121212121212121212121212121212121212121212121212121212121212"}],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}],
  "assets":[{"path":"assets/logo.svg","contentType":"image/svg+xml","sizeBytes":11,"contentHash":"sha256:3434343434343434343434343434343434343434343434343434343434343434","createdAt":"2026-01-01T00:00:00Z"}],
  "branding":[{"slot":"svg","sourcePath":"branding/favicon.svg","contentType":"image/svg+xml","sizeBytes":12,"contentHash":"sha256:5656565656565656565656565656565656565656565656565656565656565656","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}],
  "resourceCounts":{"pages":1,"components":2,"styles":1,"assets":1,"scripts":0}
}`

func TestInspectWebsiteTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments/staging/resources" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(http.StatusOK, inspectResourcesJSON), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"inspect", "website"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "default_style_bundle") || !strings.Contains(out, "public_base_url") || !strings.Contains(out, "scripts") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestInspectPageJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, inspectResourcesJSON), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"inspect", "page", "index", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"referencedComponents": [`) || !strings.Contains(out, `"header"`) || !strings.Contains(out, `"hero"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestInspectComponentTableOutputIncludesReferencingPages(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, inspectResourcesJSON), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"inspect", "component", "header"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "referencing_pages") || !strings.Contains(out, "index") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestInspectPageMissingProvidesGuidance(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, inspectResourcesJSON), nil
		},
	}

	_, _, err := runCommandWithTransport(t, []string{"inspect", "page", "missing"}, tr)
	if err == nil {
		t.Fatalf("expected missing page error")
	}
	if !strings.Contains(err.Error(), "htmlctl get pages --context staging") {
		t.Fatalf("unexpected error: %v", err)
	}
}

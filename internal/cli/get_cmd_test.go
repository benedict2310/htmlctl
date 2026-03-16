package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestGetWebsitesTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected call index %d", call)
			}
			if req.Method != "GET" || req.Path != "/api/v1/websites" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"websites":[{"name":"sample","defaultStyleBundle":"default","baseTemplate":"default","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "websites"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "sample") {
		t.Fatalf("expected website in output, got: %s", out)
	}
	if !tr.closed {
		t.Fatalf("expected transport to be closed")
	}
}

func TestGetEnvironmentsJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environments":[{"name":"staging","activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "environments", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"activeReleaseId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"`) {
		t.Fatalf("expected active release in output, got: %s", out)
	}
}

func TestGetEnvironmentsMissingContextWebsiteProvidesGuidance(t *testing.T) {
	configPath := writeRemoteCommandConfig(t, `apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: ""
    environment: staging
`)

	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransportAndConfigPath(t, []string{"get", "environments"}, tr, configPath)
	if err == nil {
		t.Fatalf("expected missing website error")
	}
	if !strings.Contains(err.Error(), "no website selected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "htmlctl context set <name> --website <website>") {
		t.Fatalf("expected context set guidance, got %v", err)
	}
}

func TestGetDomainsTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodGet || req.Path != "/api/v1/domains" {
				t.Fatalf("unexpected request: %#v", req)
			}
			if req.Query != "environment=staging&website=sample" && req.Query != "website=sample&environment=staging" {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return jsonHTTPResponse(200, `{"domains":[{"id":1,"domain":"example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "domains"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "example.com") || !strings.Contains(out, "staging") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestGetDomainsYAMLOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"domains":[{"id":1,"domain":"example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "domains", "--output", "yaml"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "domains:") || !strings.Contains(out, "domain: example.com") {
		t.Fatalf("unexpected YAML output: %s", out)
	}
}

func TestGetDomainsJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"domains":[{"id":1,"domain":"example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "domains", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"domain": "example.com"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestGetBackendsJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodGet || req.Path != "/api/v1/websites/sample/environments/staging/backends" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","backends":[{"pathPrefix":"/api/*","upstream":"https://api.example.com","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "backends", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"pathPrefix": "/api/*"`) || !strings.Contains(out, `"upstream": "https://api.example.com"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestGetBackendsTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","backends":[{"pathPrefix":"/api/*","upstream":"https://api.example.com","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "backends"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "/api/*") || !strings.Contains(out, "https://api.example.com") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestGetBackendsYAMLOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","backends":[{"pathPrefix":"/api/*","upstream":"https://api.example.com","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "backends", "--output", "yaml"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "backends:") || !strings.Contains(out, "pathPrefix: /api/*") {
		t.Fatalf("unexpected YAML output: %s", out)
	}
}

func TestGetPagesTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments/staging/resources" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, inspectResourcesJSON), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "pages"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "index") || !strings.Contains(out, "/") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestGetComponentsJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, inspectResourcesJSON), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "components", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"hasCss": true`) || !strings.Contains(out, `"name": "header"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestGetWebsiteTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, inspectResourcesJSON), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "website"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "default_style_bundle") || !strings.Contains(out, "robots_enabled") || !strings.Contains(out, "scripts") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestGetUnsupportedResourceTypeProvidesSupportedList(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"get", "widgets"}, tr)
	if err == nil {
		t.Fatalf("expected unsupported resource type error")
	}
	if !strings.Contains(err.Error(), "supported: websites, website, environments, releases, pages, components, styles, assets, branding, domains, backends") {
		t.Fatalf("unexpected error: %v", err)
	}
}

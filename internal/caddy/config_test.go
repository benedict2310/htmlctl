package caddy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConfigSorted(t *testing.T) {
	cfg, err := GenerateConfig([]Site{
		{Domain: "staging.example.com", Root: "/srv/sample/staging/current"},
		{Domain: "example.com", Root: "/srv/sample/prod/current"},
	})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	first := strings.Index(cfg, "example.com {")
	second := strings.Index(cfg, "staging.example.com {")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected deterministic sorted output, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "\troot * /srv/sample/prod/current") {
		t.Fatalf("expected prod root path in config, got:\n%s", cfg)
	}
}

func TestGenerateConfigEmpty(t *testing.T) {
	cfg, err := GenerateConfig(nil)
	if err != nil {
		t.Fatalf("GenerateConfig(nil) error = %v", err)
	}
	if strings.TrimSpace(cfg) != "# managed by htmlservd" {
		t.Fatalf("unexpected empty config output: %q", cfg)
	}
}

func TestGenerateConfigRejectsInvalidSite(t *testing.T) {
	if _, err := GenerateConfig([]Site{{Domain: "", Root: "/x"}}); err == nil {
		t.Fatalf("expected missing-domain error")
	}
	if _, err := GenerateConfig([]Site{{Domain: "example.com", Root: ""}}); err == nil {
		t.Fatalf("expected missing-root error")
	}
	if _, err := GenerateConfig([]Site{{Domain: "example.com", Root: "/srv/sample/\ncurrent"}}); err == nil {
		t.Fatalf("expected forbidden-root-character error")
	}
	if _, err := GenerateConfig([]Site{{Domain: "example.com", Root: "/srv/sample/{current}"}}); err == nil {
		t.Fatalf("expected forbidden-root-character error")
	}
	if _, err := GenerateConfig([]Site{{
		Domain:   "example.com",
		Root:     "/srv/sample/current",
		Backends: []Backend{{PathPrefix: "/api/", Upstream: "https://api.example.com"}},
	}}); err == nil {
		t.Fatalf("expected invalid backend path prefix error")
	}
	if _, err := GenerateConfig([]Site{{
		Domain:   "example.com",
		Root:     "/srv/sample/current",
		Backends: []Backend{{PathPrefix: "/api/*", Upstream: "ftp://api.example.com"}},
	}}); err == nil {
		t.Fatalf("expected invalid backend upstream error")
	}
	if _, err := GenerateConfig([]Site{{
		Domain:   "example.com",
		Root:     "/srv/sample/current",
		Backends: []Backend{{PathPrefix: "/api/*", Upstream: "https://api.example.com/base"}},
	}}); err == nil {
		t.Fatalf("expected backend upstream path rejection")
	}
}

func TestGenerateConfigWithAutoHTTPSDisabled(t *testing.T) {
	cfg, err := GenerateConfigWithOptions([]Site{
		{Domain: "example.com", Root: "/srv/sample/prod/current"},
	}, ConfigOptions{DisableAutoHTTPS: true})
	if err != nil {
		t.Fatalf("GenerateConfigWithOptions() error = %v", err)
	}
	if !strings.Contains(cfg, "auto_https off") {
		t.Fatalf("expected auto_https off in config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "http://example.com {") {
		t.Fatalf("expected explicit http site address in config, got:\n%s", cfg)
	}
}

func TestGenerateConfigWithTelemetryProxy(t *testing.T) {
	cfg, err := GenerateConfigWithOptions([]Site{
		{Domain: "example.com", Root: "/srv/sample/prod/current"},
	}, ConfigOptions{TelemetryPort: 9400})
	if err != nil {
		t.Fatalf("GenerateConfigWithOptions() error = %v", err)
	}
	if !strings.Contains(cfg, "handle /collect/v1/events*") {
		t.Fatalf("expected telemetry handle stanza in config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "reverse_proxy 127.0.0.1:9400") {
		t.Fatalf("expected telemetry reverse proxy in config, got:\n%s", cfg)
	}
}

func TestGenerateConfigWithSiteHeaders(t *testing.T) {
	cfg, err := GenerateConfig([]Site{
		{
			Domain: "preview.example.com",
			Root:   "/srv/sample/prod/releases/R1",
			Headers: map[string]string{
				"X-Robots-Tag": "noindex, nofollow, noarchive",
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	if !strings.Contains(cfg, `header X-Robots-Tag "noindex, nofollow, noarchive"`) {
		t.Fatalf("expected robots header directive, got:\n%s", cfg)
	}
}

func TestGenerateConfigWithResponseOnlySite(t *testing.T) {
	cfg, err := GenerateConfigWithOptions([]Site{
		{Domain: "example.com", Root: "/srv/sample/current"},
		{Domain: "*.preview.example.com", RespondStatus: 404},
	}, ConfigOptions{DisableAutoHTTPS: true})
	if err != nil {
		t.Fatalf("GenerateConfigWithOptions() error = %v", err)
	}
	if !strings.Contains(cfg, "http://*.preview.example.com {\n\trespond 404\n}") {
		t.Fatalf("expected response-only preview fallback, got:\n%s", cfg)
	}
	if strings.Contains(cfg, "http://*.preview.example.com {\n\troot *") {
		t.Fatalf("did not expect root directive in response-only site, got:\n%s", cfg)
	}
}

func TestGenerateConfigWithBackends(t *testing.T) {
	cfg, err := GenerateConfig([]Site{
		{
			Domain: "example.com",
			Root:   "/srv/sample/prod/current",
			Backends: []Backend{
				{PathPrefix: "/auth/*", Upstream: "https://auth.example.com"},
				{PathPrefix: "/api/*", Upstream: "https://api.example.com"},
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}

	apiIndex := strings.Index(cfg, "\thandle /api/* {\n\t\treverse_proxy https://api.example.com\n\t}")
	authIndex := strings.Index(cfg, "\thandle /auth/* {\n\t\treverse_proxy https://auth.example.com\n\t}")
	fileServerIndex := strings.Index(cfg, "\thandle {\n\t\troot * /srv/sample/prod/current\n\t\tfile_server\n\t}")
	if apiIndex == -1 || authIndex == -1 || fileServerIndex == -1 {
		t.Fatalf("expected backend directives and file_server, got:\n%s", cfg)
	}
	if !(apiIndex < authIndex && authIndex < fileServerIndex) {
		t.Fatalf("expected sorted backend directives before file_server, got:\n%s", cfg)
	}
}

func TestGenerateConfigMixedBackendSites(t *testing.T) {
	cfg, err := GenerateConfig([]Site{
		{
			Domain: "example.com",
			Root:   "/srv/sample/prod/current",
			Backends: []Backend{
				{PathPrefix: "/api/*", Upstream: "https://api.example.com"},
			},
		},
		{
			Domain: "static.example.com",
			Root:   "/srv/sample/static/current",
		},
	})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	if strings.Count(cfg, "\thandle /api/* {\n\t\treverse_proxy https://api.example.com\n\t}") != 1 {
		t.Fatalf("expected one backend directive, got:\n%s", cfg)
	}
	expectedStaticBlock := "static.example.com {\n\troot * /srv/sample/static/current\n\thandle {\n\t\troot * /srv/sample/static/current\n\t\tfile_server\n\t}\n}"
	if !strings.Contains(cfg, expectedStaticBlock) {
		t.Fatalf("expected static-only site block, got:\n%s", cfg)
	}
}

func TestGenerateConfigOrdersMoreSpecificBackendsFirst(t *testing.T) {
	cfg, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		Backends: []Backend{
			{PathPrefix: "/api/*", Upstream: "https://api.example.com"},
			{PathPrefix: "/api/internal/*", Upstream: "https://internal.example.com"},
		},
	}})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}

	internalIndex := strings.Index(cfg, "\thandle /api/internal/* {\n\t\treverse_proxy https://internal.example.com\n\t}")
	apiIndex := strings.Index(cfg, "\thandle /api/* {\n\t\treverse_proxy https://api.example.com\n\t}")
	if internalIndex == -1 || apiIndex == -1 {
		t.Fatalf("expected both backend handle blocks, got:\n%s", cfg)
	}
	if internalIndex > apiIndex {
		t.Fatalf("expected more specific backend to be emitted first, got:\n%s", cfg)
	}
}

func TestGenerateConfigWithProtectedBackend(t *testing.T) {
	cfg, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		Backends: []Backend{
			{PathPrefix: "/api/*", Upstream: "https://api.example.com"},
		},
		AuthPolicies: []AuthPolicy{
			{PathPrefix: "/api/*", Username: "reviewer", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
		},
	}})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	if !strings.Contains(cfg, "\thandle /api/* {\n\t\tbasic_auth {\n\t\t\treviewer ") {
		t.Fatalf("expected protected backend basic auth stanza, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "\t\treverse_proxy https://api.example.com\n\t}") {
		t.Fatalf("expected protected backend reverse_proxy stanza, got:\n%s", cfg)
	}
}

func TestGenerateConfigWithProtectedStaticPath(t *testing.T) {
	cfg, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		AuthPolicies: []AuthPolicy{
			{PathPrefix: "/docs/*", Username: "reviewer", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
		},
	}})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	if !strings.Contains(cfg, "\thandle /docs/* {\n\t\tbasic_auth {\n\t\t\treviewer ") {
		t.Fatalf("expected protected static basic auth stanza, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "\t\troot * /srv/sample/prod/current\n\t\tfile_server\n\t}") {
		t.Fatalf("expected protected static file_server stanza, got:\n%s", cfg)
	}
}

func TestGenerateConfigRejectsOverlappingAuthPolicies(t *testing.T) {
	_, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		AuthPolicies: []AuthPolicy{
			{PathPrefix: "/docs/*", Username: "reviewer", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
			{PathPrefix: "/docs/private/*", Username: "admin", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
		},
	}})
	if err == nil {
		t.Fatalf("expected overlapping auth policy error")
	}
}

func TestGenerateConfigRejectsAuthPolicyBackendOverlap(t *testing.T) {
	_, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		Backends: []Backend{
			{PathPrefix: "/api/internal/*", Upstream: "https://api.example.com"},
		},
		AuthPolicies: []AuthPolicy{
			{PathPrefix: "/api/*", Username: "reviewer", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
		},
	}})
	if err == nil {
		t.Fatalf("expected auth/backend overlap error")
	}
}

func TestGenerateConfigAllowsExactAuthPolicyBackendMatch(t *testing.T) {
	cfg, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		Backends: []Backend{
			{PathPrefix: "/api/*", Upstream: "https://api.example.com"},
		},
		AuthPolicies: []AuthPolicy{
			{PathPrefix: "/api/*", Username: "reviewer", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
		},
	}})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	if !strings.Contains(cfg, "\thandle /api/* {\n\t\tbasic_auth {") || !strings.Contains(cfg, "\t\treverse_proxy https://api.example.com\n\t}") {
		t.Fatalf("expected exact-match protected backend stanza, got:\n%s", cfg)
	}
}

func TestGenerateConfigRejectsTelemetryAuthPolicyOverlap(t *testing.T) {
	_, err := GenerateConfigWithOptions([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		AuthPolicies: []AuthPolicy{
			{PathPrefix: "/collect/*", Username: "reviewer", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5M4FQ8Q6H4L9Xn1w1s8T8a1mYg0sB7e"},
		},
	}}, ConfigOptions{TelemetryPort: 9400})
	if err == nil {
		t.Fatalf("expected telemetry overlap error")
	}
}

func TestGenerateConfigWithoutTelemetryProxyWhenDisabled(t *testing.T) {
	cfg, err := GenerateConfigWithOptions([]Site{
		{Domain: "example.com", Root: "/srv/sample/prod/current"},
	}, ConfigOptions{TelemetryPort: 0})
	if err != nil {
		t.Fatalf("GenerateConfigWithOptions() error = %v", err)
	}
	if strings.Contains(cfg, "handle /collect/v1/events*") {
		t.Fatalf("did not expect telemetry handle stanza, got:\n%s", cfg)
	}
}

func TestGenerateConfigRejectsInvalidTelemetryPort(t *testing.T) {
	if _, err := GenerateConfigWithOptions([]Site{
		{Domain: "example.com", Root: "/srv/sample/prod/current"},
	}, ConfigOptions{TelemetryPort: 70000}); err == nil {
		t.Fatalf("expected invalid telemetry port error")
	}
}

func TestGenerateConfigRejectsInvalidHeader(t *testing.T) {
	if _, err := GenerateConfig([]Site{{
		Domain: "example.com",
		Root:   "/srv/sample/prod/current",
		Headers: map[string]string{
			"Bad\nHeader": "value",
		},
	}}); err == nil {
		t.Fatalf("expected invalid header error")
	}
}

func TestGenerateConfigRejectsInvalidResponseStatus(t *testing.T) {
	if _, err := GenerateConfig([]Site{{
		Domain:        "*.preview.example.com",
		RespondStatus: 1000,
	}}); err == nil {
		t.Fatalf("expected invalid response status error")
	}
}

func TestWriteConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Caddyfile")
	if err := WriteConfig(path, "example.com {\n\tfile_server\n}\n"); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(b), "example.com") {
		t.Fatalf("unexpected config content: %s", string(b))
	}
}

func TestWriteConfigEmptyPath(t *testing.T) {
	if err := WriteConfig("", "x"); err == nil {
		t.Fatalf("expected empty path error")
	}
}

func TestWriteConfigCreateDirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	path := filepath.Join(blocker, "Caddyfile")
	if err := WriteConfig(path, "x"); err == nil {
		t.Fatalf("expected directory creation failure")
	}
}

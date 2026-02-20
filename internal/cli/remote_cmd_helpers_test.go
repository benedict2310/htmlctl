package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/transport"
)

type recordedRequest struct {
	Method  string
	Path    string
	Query   string
	Headers http.Header
	Body    []byte
}

type scriptedTransport struct {
	handle   func(call int, req recordedRequest) (*http.Response, error)
	requests []recordedRequest
	closed   bool
}

func (s *scriptedTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}
	call := len(s.requests)
	s.requests = append(s.requests, recordedRequest{
		Method:  req.Method,
		Path:    req.URL.Path,
		Query:   req.URL.RawQuery,
		Headers: req.Header.Clone(),
		Body:    body,
	})
	if s.handle == nil {
		return nil, errors.New("unexpected transport call")
	}
	return s.handle(call, s.requests[call])
}

func (s *scriptedTransport) Close() error {
	s.closed = true
	return nil
}

func runCommandWithTransport(t *testing.T, args []string, tr *scriptedTransport) (string, string, error) {
	t.Helper()

	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	prevFactory := buildTransportForContext
	buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
		return tr, nil
	}
	t.Cleanup(func() {
		buildTransportForContext = prevFactory
	})

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestRemoteCommandsSendContextTokenAsBearerHeader(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: futurelab
    environment: staging
    token: test-context-token
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv(config.EnvConfigPath, configPath)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected call %d", call)
			}
			if got := req.Headers.Get("Authorization"); got != "Bearer test-context-token" {
				t.Fatalf("expected bearer token header, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"websites":[]}`), nil
		},
	}

	prevFactory := buildTransportForContext
	buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
		return tr, nil
	}
	t.Cleanup(func() {
		buildTransportForContext = prevFactory
	})

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"get", "websites"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func writeApplySiteFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, rel := range []string{
		"components",
		"pages",
		"styles",
		"assets",
	} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "website.yaml"), []byte(`apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: futurelab
spec:
  defaultStyleBundle: default
  baseTemplate: default
`), 0o644); err != nil {
		t.Fatalf("write website.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "header.html"), []byte("<section id=\"header\">Header</section>\n"), 0o644); err != nil {
		t.Fatalf("write component: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pages", "index.page.yaml"), []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout:
    - include: header
`), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "styles", "tokens.css"), []byte(":root { --brand: #00f; }\n"), 0o644); err != nil {
		t.Fatalf("write tokens.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "styles", "default.css"), []byte("body { margin: 0; }\n"), 0o644); err != nil {
		t.Fatalf("write default.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "logo.svg"), []byte("<svg></svg>\n"), 0o644); err != nil {
		t.Fatalf("write logo.svg: %v", err)
	}
	return root
}

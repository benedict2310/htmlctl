package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestApplyEndpointSuccessAndAutoCreate(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	component := []byte("<section id=\"header\">Header</section>")
	page := []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n  description: Home page\n  layout:\n    - include: header\n")
	tokensCSS := []byte(":root { --brand: #00f; }")
	defaultCSS := []byte("body { margin: 0; }")
	asset := []byte("<svg></svg>")

	body := tarBody(t, map[string][]byte{
		"components/header.html": component,
		"pages/index.page.yaml":  page,
		"styles/tokens.css":      tokensCSS,
		"styles/default.css":     defaultCSS,
		"assets/logo.svg":        asset,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{
				"kind": "Component", "name": "header", "file": "components/header.html", "hash": "sha256:" + sha256Hex(component),
			},
			{
				"kind": "Page", "name": "index", "file": "pages/index.page.yaml", "hash": "sha256:" + sha256Hex(page),
			},
			{
				"kind": "StyleBundle", "name": "default", "files": []map[string]any{
					{"file": "styles/tokens.css", "hash": "sha256:" + sha256Hex(tokensCSS)},
					{"file": "styles/default.css", "hash": "sha256:" + sha256Hex(defaultCSS)},
				},
			},
			{
				"kind": "Asset", "name": "assets/logo.svg", "file": "assets/logo.svg", "hash": "sha256:" + sha256Hex(asset), "contentType": "image/svg+xml",
			},
		},
	})

	resp := postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}

	var out applyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Website != "futurelab" || out.Environment != "staging" || out.Mode != "partial" {
		t.Fatalf("unexpected response metadata: %#v", out)
	}
	if len(out.AcceptedResource) != 4 {
		t.Fatalf("expected 4 accepted resources, got %d", len(out.AcceptedResource))
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	if _, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging"); err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if rows, err := q.ListComponentsByWebsite(context.Background(), websiteRow.ID); err != nil || len(rows) != 1 {
		t.Fatalf("ListComponentsByWebsite() rows=%d err=%v", len(rows), err)
	}
	if rows, err := q.ListPagesByWebsite(context.Background(), websiteRow.ID); err != nil || len(rows) != 1 {
		t.Fatalf("ListPagesByWebsite() rows=%d err=%v", len(rows), err)
	}
	if rows, err := q.ListStyleBundlesByWebsite(context.Background(), websiteRow.ID); err != nil || len(rows) != 1 {
		t.Fatalf("ListStyleBundlesByWebsite() rows=%d err=%v", len(rows), err)
	}
	if rows, err := q.ListAssetsByWebsite(context.Background(), websiteRow.ID); err != nil || len(rows) != 1 {
		t.Fatalf("ListAssetsByWebsite() rows=%d err=%v", len(rows), err)
	}

	for _, hexHash := range []string{sha256Hex(component), sha256Hex(page), sha256Hex(tokensCSS), sha256Hex(defaultCSS), sha256Hex(asset)} {
		blobPath := filepath.Join(srv.dataPaths.BlobsSHA256, hexHash)
		if _, err := os.Stat(blobPath); err != nil {
			t.Fatalf("expected blob %s to exist: %v", blobPath, err)
		}
	}
}

func TestApplyEndpointDryRunDoesNotPersist(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	component := []byte("<section id=\"header\">Header</section>")
	body := tarBody(t, map[string][]byte{
		"components/header.html": component,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Component", "name": "header", "file": "components/header.html", "hash": "sha256:" + sha256Hex(component)},
		},
	})

	resp := postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply?dry_run=true", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}

	q := dbpkg.NewQueries(srv.db)
	_, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected website to not persist in dry_run, got err=%v", err)
	}
}

func TestApplyEndpointHashMismatch(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	component := []byte("<section id=\"header\">Header</section>")
	body := tarBody(t, map[string][]byte{
		"components/header.html": component,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Component", "name": "header", "file": "components/header.html", "hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		},
	})

	resp := postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestApplyEndpointFullModeDeletesMissingResources(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	header := []byte("<section id=\"header\">Header</section>")
	footer := []byte("<footer id=\"footer\">Footer</footer>")

	partial := tarBody(t, map[string][]byte{
		"components/header.html": header,
		"components/footer.html": footer,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Component", "name": "header", "file": "components/header.html", "hash": "sha256:" + sha256Hex(header)},
			{"kind": "Component", "name": "footer", "file": "components/footer.html", "hash": "sha256:" + sha256Hex(footer)},
		},
	})
	resp := postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", partial)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected first apply to succeed, got %d", resp.StatusCode)
	}

	full := tarBody(t, map[string][]byte{
		"components/header.html": header,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "full",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Component", "name": "header", "file": "components/header.html", "hash": "sha256:" + sha256Hex(header)},
		},
	})
	resp = postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", full)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected full apply to succeed, got %d body=%s", resp.StatusCode, string(b))
	}
	resp.Body.Close()

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	rows, err := q.ListComponentsByWebsite(context.Background(), websiteRow.ID)
	if err != nil {
		t.Fatalf("ListComponentsByWebsite() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "header" {
		t.Fatalf("expected only header after full apply, got %#v", rows)
	}
}

func TestApplyEndpointNormalizesHashFormat(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	component := []byte("<section id=\"header\">Header</section>")
	rawHex := sha256Hex(component)
	body := tarBody(t, map[string][]byte{
		"components/header.html": component,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Component", "name": "header", "file": "components/header.html", "hash": rawHex},
		},
	})

	resp := postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected apply to succeed, got %d", resp.StatusCode)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	rows, err := q.ListComponentsByWebsite(context.Background(), websiteRow.ID)
	if err != nil {
		t.Fatalf("ListComponentsByWebsite() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one component, got %d", len(rows))
	}
	if rows[0].ContentHash != "sha256:"+rawHex {
		t.Fatalf("expected canonical hash format, got %q", rows[0].ContentHash)
	}
}

func TestApplyEndpointPartialModeDeletesAsset(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	asset := []byte("<svg></svg>")

	createBundle := tarBody(t, map[string][]byte{
		"assets/logo.svg": asset,
	}, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Asset", "name": "assets/logo.svg", "file": "assets/logo.svg", "hash": "sha256:" + sha256Hex(asset), "contentType": "image/svg+xml"},
		},
	})
	resp := postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", createBundle)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected first apply to succeed, got %d", resp.StatusCode)
	}

	deleteBundle := tarBody(t, nil, map[string]any{
		"apiVersion": "htmlctl.dev/v1",
		"kind":       "Bundle",
		"mode":       "partial",
		"website":    "futurelab",
		"resources": []map[string]any{
			{"kind": "Asset", "name": "assets/logo.svg", "file": "assets/logo.svg", "deleted": true},
		},
	})
	resp = postBundle(t, baseURL+"/api/v1/websites/futurelab/environments/staging/apply", deleteBundle)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected delete apply to succeed, got %d body=%s", resp.StatusCode, string(b))
	}
	resp.Body.Close()

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	rows, err := q.ListAssetsByWebsite(context.Background(), websiteRow.ID)
	if err != nil {
		t.Fatalf("ListAssetsByWebsite() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no assets after delete, got %#v", rows)
	}
}

func startTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: t.TempDir(), LogLevel: "info", DBWAL: true}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return srv
}

func postBundle(t *testing.T, endpoint string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", endpoint, err)
	}
	return resp
}

func tarBody(t *testing.T, files map[string][]byte, manifest map[string]any) []byte {
	t.Helper()
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeTarFile(t, tw, "manifest.json", manifestBytes)
	for name, content := range files {
		writeTarFile(t, tw, name, content)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, content []byte) {
	t.Helper()
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header %s: %v", name, err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content %s: %v", name, err)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

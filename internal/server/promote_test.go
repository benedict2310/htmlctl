package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestPromoteEndpointSuccess(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	sourceReleaseID := createReleaseWithActor(t, baseURL, "alice")
	ensureEnvironment(t, srv.db, "sample", "prod")

	body := bytes.NewBufferString(`{"from":"staging","to":"prod"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/websites/sample/promote", body)
	if err != nil {
		t.Fatalf("new promote request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", "carol")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /promote error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}
	var out promoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode promote response: %v", err)
	}
	if out.SourceReleaseID != sourceReleaseID || out.ReleaseID == "" || !out.HashVerified {
		t.Fatalf("unexpected promote response: %#v", out)
	}

	stagingDir := filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "staging", "releases", sourceReleaseID)
	prodDir := filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "prod", "releases", out.ReleaseID)
	stagingHashes, err := computePromotedContentHashes(stagingDir)
	if err != nil {
		t.Fatalf("computePromotedContentHashes(staging) error = %v", err)
	}
	prodHashes, err := computePromotedContentHashes(prodDir)
	if err != nil {
		t.Fatalf("computePromotedContentHashes(prod) error = %v", err)
	}
	if mismatch := compareHashMaps(stagingHashes, prodHashes); mismatch != "" {
		t.Fatalf("expected promoted content hashes to match staging; mismatch=%s", mismatch)
	}

	currentTarget, err := os.Readlink(filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "prod", "current"))
	if err != nil {
		t.Fatalf("read prod current symlink: %v", err)
	}
	if currentTarget != filepath.ToSlash(filepath.Join("releases", out.ReleaseID)) {
		t.Fatalf("expected prod current target %q, got %q", filepath.ToSlash(filepath.Join("releases", out.ReleaseID)), currentTarget)
	}

	waitForPromoteAuditEntry(t, baseURL)
}

func TestPromoteEndpointSourceHasNoActiveRelease(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	ensureEnvironment(t, srv.db, "sample", "prod")

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"prod","to":"staging"}`))
	if err != nil {
		t.Fatalf("POST /promote error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestPromoteEndpointTargetEnvironmentNotFound(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	createReleaseWithActor(t, baseURL, "alice")

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"staging","to":"prod"}`))
	if err != nil {
		t.Fatalf("POST /promote error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 404, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestPromoteEndpointRejectsSameEnvironment(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"staging","to":"staging"}`))
	if err != nil {
		t.Fatalf("POST /promote error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestPromoteEndpointHashMismatchDoesNotSwitchTargetCurrent(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)
	sourceReleaseID := createReleaseWithActor(t, baseURL, "alice")
	ensureEnvironment(t, srv.db, "sample", "prod")

	if _, err := srv.db.Exec(`UPDATE releases SET output_hashes = ? WHERE id = ?`, `{"index.html":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, sourceReleaseID); err != nil {
		t.Fatalf("inject hash mismatch in source release row: %v", err)
	}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"staging","to":"prod"}`))
	if err != nil {
		t.Fatalf("POST /promote error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read hash mismatch response body: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
	}
	if strings.Contains(string(body), "hash mismatch") || strings.Contains(string(body), "aaaaaaaa") {
		t.Fatalf("response body leaked internal hash mismatch details: %s", string(body))
	}
	var out struct {
		Error   string   `json:"error"`
		Details []string `json:"details,omitempty"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode hash mismatch response: %v body=%s", err, string(body))
	}
	if out.Error != "promotion hash verification failed" {
		t.Fatalf("expected sanitized error message, got %q", out.Error)
	}
	if len(out.Details) != 0 {
		t.Fatalf("expected no details in hash mismatch response, got %#v", out.Details)
	}
	if _, err := os.Readlink(filepath.Join(srv.dataPaths.WebsitesRoot, "sample", "envs", "prod", "current")); !os.IsNotExist(err) {
		t.Fatalf("expected prod current symlink to remain absent, got err=%v", err)
	}
}

func TestPromoteEndpointMethodNotAllowedAndInvalidBody(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/promote")
	if err != nil {
		t.Fatalf("GET /promote error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 405, got %d body=%s", resp.StatusCode, string(b))
	}

	resp, err = http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{`))
	if err != nil {
		t.Fatalf("POST /promote invalid body error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read invalid-body response: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(body))
	}
	var out struct {
		Error   string   `json:"error"`
		Details []string `json:"details,omitempty"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode invalid-body response: %v body=%s", err, string(body))
	}
	if len(out.Details) == 0 {
		t.Fatalf("expected verbose 4xx details for invalid JSON body, got %#v", out)
	}
}

func TestPromoteEndpointMissingFields(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"staging"}`))
	if err != nil {
		t.Fatalf("POST /promote missing fields error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestPromoteEndpointRejectsInvalidNames(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	resp, err := http.Post(baseURL+"/api/v1/websites/future.lab/promote", "application/json", bytes.NewBufferString(`{"from":"staging","to":"prod"}`))
	if err != nil {
		t.Fatalf("POST /promote invalid website error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid website name, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"staging\nblue","to":"prod"}`))
	if err != nil {
		t.Fatalf("POST /promote invalid from env error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid source env name, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/websites/sample/promote", "application/json", bytes.NewBufferString(`{"from":"staging","to":"prod{evil}"}`))
	if err != nil {
		t.Fatalf("POST /promote invalid to env error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid target env name, got %d", resp.StatusCode)
	}
}

func ensureEnvironment(t *testing.T, db *sql.DB, website, env string) {
	t.Helper()
	q := dbpkg.NewQueries(db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), website)
	if err != nil {
		t.Fatalf("GetWebsiteByName(%q) error = %v", website, err)
	}
	if _, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, env); err == nil {
		return
	}
	if _, err := q.InsertEnvironment(context.Background(), dbpkg.EnvironmentRow{WebsiteID: websiteRow.ID, Name: env}); err != nil {
		t.Fatalf("InsertEnvironment(%q) error = %v", env, err)
	}
}

func waitForPromoteAuditEntry(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/prod/logs?operation=promote")
		if err != nil {
			t.Fatalf("GET /logs?operation=promote error = %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("expected 200 for promote logs, got %d body=%s", resp.StatusCode, string(b))
		}
		var out logsResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			t.Fatalf("decode promote logs response: %v", err)
		}
		resp.Body.Close()
		if out.Total >= 1 && len(out.Entries) >= 1 {
			if out.Entries[0].Operation != "promote" {
				t.Fatalf("expected promote operation, got %#v", out.Entries[0])
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for promote audit entry")
}

func computePromotedContentHashes(root string) (map[string]string, error) {
	paths := []string{}
	err := filepath.WalkDir(root, func(pathValue string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, pathValue)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch rel {
		case ".manifest.json", ".build-log.txt", ".output-hashes.json":
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := map[string]string{}
	for _, rel := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(content)
		out[rel] = "sha256:" + hex.EncodeToString(sum[:])
	}
	return out, nil
}

func compareHashMaps(source, target map[string]string) string {
	for pathValue, sourceHash := range source {
		targetHash, ok := target[pathValue]
		if !ok {
			return "target missing " + pathValue
		}
		if sourceHash != targetHash {
			return "hash mismatch at " + pathValue
		}
	}
	for pathValue := range target {
		if _, ok := source[pathValue]; !ok {
			return "target has unexpected " + pathValue
		}
	}
	return ""
}

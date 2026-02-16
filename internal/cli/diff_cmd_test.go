package cli

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/bundle"
)

func TestDiffCommandReturnsExitCodeOneWhenChangesDetected(t *testing.T) {
	siteDir := writeApplySiteFixture(t)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			if req.Method != "GET" || req.Path != "/api/v1/websites/futurelab/environments/staging/manifest" {
				t.Fatalf("unexpected diff request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"website":"futurelab","environment":"staging","files":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"diff", "-f", siteDir}, tr)
	if err == nil {
		t.Fatalf("expected diff to return non-zero exit error when changes exist")
	}
	if got := ExitCode(err); got != 1 {
		t.Fatalf("expected exit code 1, got %d (err=%v)", got, err)
	}
	if !strings.Contains(out, "added") {
		t.Fatalf("expected diff output with changes, got: %s", out)
	}
}

func TestDiffCommandNoChangesReturnsExitCodeZero(t *testing.T) {
	siteDir := writeApplySiteFixture(t)
	remotePayload := buildManifestPayloadForSite(t, siteDir)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			if req.Method != "GET" || req.Path != "/api/v1/websites/futurelab/environments/staging/manifest" {
				t.Fatalf("unexpected diff request: %#v", req)
			}
			return jsonHTTPResponse(200, remotePayload), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"diff", "-f", siteDir}, tr)
	if err != nil {
		t.Fatalf("expected no diff error, got %v", err)
	}
	if !strings.Contains(out, "No changes detected.") {
		t.Fatalf("unexpected diff output: %s", out)
	}
}

func TestDiffCommandJSONIncludesFullHashes(t *testing.T) {
	siteDir := writeApplySiteFixture(t)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			return jsonHTTPResponse(200, `{"website":"futurelab","environment":"staging","files":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"diff", "-f", siteDir, "--output", "json"}, tr)
	if err == nil {
		t.Fatalf("expected diff to return non-zero exit error when changes exist")
	}
	if got := ExitCode(err); got != 1 {
		t.Fatalf("expected exit code 1, got %d (err=%v)", got, err)
	}
	if !strings.Contains(out, "sha256:") {
		t.Fatalf("expected json diff output to include full hashes, got: %s", out)
	}
	if strings.Contains(out, "OLD_HASH") {
		t.Fatalf("expected structured output, got table output: %s", out)
	}
}

func TestDiffCommandMissingFromReturnsExitCodeTwo(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			t.Fatalf("unexpected transport call %d: %#v", call, req)
			return nil, nil
		},
	}

	_, _, err := runCommandWithTransport(t, []string{"diff"}, tr)
	if err == nil {
		t.Fatalf("expected missing --from error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected exit code 2, got %d (err=%v)", got, err)
	}
}

func buildManifestPayloadForSite(t *testing.T, siteDir string) string {
	t.Helper()

	_, manifest, err := bundle.BuildTarFromDir(siteDir, "futurelab")
	if err != nil {
		t.Fatalf("BuildTarFromDir() error = %v", err)
	}
	type file struct {
		Path string `json:"path"`
		Hash string `json:"hash"`
	}
	files := make([]file, 0, len(manifest.Resources))
	for _, resource := range manifest.Resources {
		for _, ref := range resource.FileEntries() {
			files = append(files, file{
				Path: ref.File,
				Hash: ref.Hash,
			})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	payload := map[string]any{
		"website":     "futurelab",
		"environment": "staging",
		"files":       files,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(b)
}

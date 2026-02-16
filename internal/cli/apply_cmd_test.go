package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/bundle"
)

func TestApplyCommandProgressAndRelease(t *testing.T) {
	siteDir := writeApplySiteFixture(t)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			switch call {
			case 0:
				if req.Method != "POST" || req.Path != "/api/v1/websites/futurelab/environments/staging/apply" {
					t.Fatalf("unexpected apply request: %#v", req)
				}
				b, err := bundle.ReadTar(bytes.NewReader(req.Body))
				if err != nil {
					t.Fatalf("bundle.ReadTar() error = %v", err)
				}
				if b.Manifest.Website != "futurelab" {
					t.Fatalf("unexpected manifest website %q", b.Manifest.Website)
				}
				return jsonHTTPResponse(200, `{"website":"futurelab","environment":"staging","mode":"full","dryRun":false,"acceptedResources":[{"kind":"Component","name":"header"}],"changes":{"created":1,"updated":0,"deleted":0}}`), nil
			case 1:
				if req.Method != "POST" || req.Path != "/api/v1/websites/futurelab/environments/staging/releases" {
					t.Fatalf("unexpected releases request: %#v", req)
				}
				return jsonHTTPResponse(201, `{"website":"futurelab","environment":"staging","releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV","status":"active"}`), nil
			default:
				t.Fatalf("unexpected transport call %d", call)
				return nil, nil
			}
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"apply", "-f", siteDir}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, marker := range []string{"Bundling...", "Uploading...", "Validating...", "Rendering...", "Activating...", "Done. Release 01ARZ3NDEKTSV4RRFFQ69G5FAV active."} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected %q in output, got: %s", marker, out)
		}
	}
	if len(tr.requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(tr.requests))
	}
}

func TestApplyCommandLocalValidationFailsBeforeUpload(t *testing.T) {
	dir := t.TempDir()

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			t.Fatalf("unexpected transport call %#v", req)
			return nil, nil
		},
	}

	_, _, err := runCommandWithTransport(t, []string{"apply", "-f", dir}, tr)
	if err == nil {
		t.Fatalf("expected local validation error")
	}
	if !strings.Contains(err.Error(), "local validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tr.requests) != 0 {
		t.Fatalf("expected no API requests, got %d", len(tr.requests))
	}
}

func TestApplyCommandJSONOutputSuppressesProgress(t *testing.T) {
	siteDir := writeApplySiteFixture(t)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call == 0 {
				return jsonHTTPResponse(200, `{"website":"futurelab","environment":"staging","mode":"full","dryRun":false,"acceptedResources":[{"kind":"Component","name":"header"}],"changes":{"created":1,"updated":0,"deleted":0}}`), nil
			}
			return jsonHTTPResponse(201, `{"website":"futurelab","environment":"staging","releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV","status":"active"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"apply", "-f", siteDir, "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(out, "Bundling...") {
		t.Fatalf("expected progress output to be suppressed for json mode, got: %s", out)
	}
	if !strings.Contains(out, `"releaseId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"`) {
		t.Fatalf("expected json release id output, got: %s", out)
	}
}

func TestApplyCommandDryRunUsesDiffWithoutUpload(t *testing.T) {
	siteDir := writeApplySiteFixture(t)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			if req.Method != "GET" || req.Path != "/api/v1/websites/futurelab/environments/staging/manifest" {
				t.Fatalf("unexpected dry-run request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"website":"futurelab","environment":"staging","files":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"apply", "-f", siteDir, "--dry-run"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(tr.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(tr.requests))
	}
	if !strings.Contains(out, "Dry run: no changes applied") {
		t.Fatalf("expected dry-run message, got: %s", out)
	}
	if strings.Contains(out, "Uploading...") || strings.Contains(out, "Rendering...") {
		t.Fatalf("expected no upload/release progress in dry-run output, got: %s", out)
	}
}

package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestRetentionRunCommand(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodPost || req.Path != "/api/v1/websites/sample/environments/staging/retention/run" {
				t.Fatalf("unexpected retention request: %#v", req)
			}
			body := string(req.Body)
			if !strings.Contains(body, `"keep":2`) || !strings.Contains(body, `"dryRun":true`) || !strings.Contains(body, `"blobGC":true`) {
				t.Fatalf("unexpected retention request body: %s", body)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","keep":2,"dryRun":true,"blobGC":true,"activeReleaseId":"R3","rollbackReleaseId":"R2","previewPinnedReleaseIds":["R1"],"retainedReleaseIds":["R3","R2","R1"],"prunableReleaseIds":["R0"],"prunedReleaseIds":[],"markedBlobCount":7,"blobDeleteCandidates":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"deletedBlobHashes":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"retention", "run", "website/sample", "--env", "staging", "--keep", "2", "--dry-run", "--blob-gc"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "retention run complete for sample/staging") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Dry run only; no data was deleted.") {
		t.Fatalf("expected dry-run summary in output: %s", out)
	}
	if !strings.Contains(out, "Prunable IDs: R0") {
		t.Fatalf("expected prunable ids in output: %s", out)
	}
}

func TestRetentionRunJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","keep":5,"dryRun":false,"blobGC":false,"retainedReleaseIds":["R3","R2"],"prunableReleaseIds":[],"prunedReleaseIds":[],"markedBlobCount":0,"blobDeleteCandidates":[],"deletedBlobHashes":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"retention", "run", "--keep", "5", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"keep": 5`) || !strings.Contains(out, `"blobGC": false`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

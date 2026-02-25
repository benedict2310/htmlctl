package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestLogsEndpointsReturnApplyAndReleaseAuditEntries(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	applySampleSite(t, baseURL)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/websites/sample/environments/staging/releases", nil)
	if err != nil {
		t.Fatalf("new release request: %v", err)
	}
	req.Header.Set("X-Actor", "bene")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /releases error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected release status 201, got %d", resp.StatusCode)
	}

	var envLogs logsResponse
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err = http.Get(baseURL + "/api/v1/websites/sample/environments/staging/logs")
		if err != nil {
			t.Fatalf("GET env logs error = %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("expected 200 for env logs, got %d body=%s", resp.StatusCode, string(b))
		}
		if err := json.NewDecoder(resp.Body).Decode(&envLogs); err != nil {
			resp.Body.Close()
			t.Fatalf("decode env logs: %v", err)
		}
		resp.Body.Close()
		if envLogs.Total >= 3 && len(envLogs.Entries) >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected at least 3 entries, got total=%d len=%d", envLogs.Total, len(envLogs.Entries))
		}
		time.Sleep(25 * time.Millisecond)
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/sample/environments/staging/logs?operation=apply")
	if err != nil {
		t.Fatalf("GET filtered logs error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for filtered logs, got %d", resp.StatusCode)
	}
	var filtered logsResponse
	if err := json.NewDecoder(resp.Body).Decode(&filtered); err != nil {
		t.Fatalf("decode filtered logs: %v", err)
	}
	if filtered.Total < 1 {
		t.Fatalf("expected at least one apply log entry")
	}
	for _, entry := range filtered.Entries {
		if entry.Operation != "apply" {
			t.Fatalf("unexpected filtered operation %q", entry.Operation)
		}
	}

	resp, err = http.Get(baseURL + "/api/v1/websites/sample/logs?limit=2")
	if err != nil {
		t.Fatalf("GET website logs error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for website logs, got %d", resp.StatusCode)
	}
	var websiteLogs logsResponse
	if err := json.NewDecoder(resp.Body).Decode(&websiteLogs); err != nil {
		t.Fatalf("decode website logs: %v", err)
	}
	if websiteLogs.Limit != 2 {
		t.Fatalf("expected applied limit 2, got %d", websiteLogs.Limit)
	}
	if len(websiteLogs.Entries) > 2 {
		t.Fatalf("expected at most 2 entries for limit=2, got %d", len(websiteLogs.Entries))
	}
}

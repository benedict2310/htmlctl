package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRollbackHandlerServiceUnavailableAndNotFound(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/websites/sample/environments/staging/rollback", nil)
	rec := httptest.NewRecorder()
	s.handleRollback(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 503, got %d body=%s", resp.StatusCode, string(body))
	}

	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	resp2, err := http.Post(baseURL+"/api/v1/websites/missing/environments/staging/rollback", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /rollback missing website error = %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 404, got %d body=%s", resp2.StatusCode, string(body))
	}
}

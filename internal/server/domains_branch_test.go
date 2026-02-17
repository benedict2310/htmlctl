package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDomainsHandlerServiceUnavailableAndNotFound(t *testing.T) {
	srv := &Server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/domains", nil)
	srv.handleDomains(rr, req)
	if rr.Code != 503 {
		t.Fatalf("expected 503 when db not initialized, got %d", rr.Code)
	}

	started := startTestServer(t)
	baseURL := "http://" + started.Addr()
	resp, err := http.Get(baseURL + "/api/v1/domains/missing.example.com")
	if err != nil {
		t.Fatalf("GET /domains missing error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404 for missing domain, got %d", resp.StatusCode)
	}
}

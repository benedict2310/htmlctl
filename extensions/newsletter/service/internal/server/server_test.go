package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	ts := httptest.NewServer(New())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNewsletterPathsAreHandledWithoutRewrite(t *testing.T) {
	ts := httptest.NewServer(New())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/signup")
	if err != nil {
		t.Fatalf("GET /newsletter/signup: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501 for /newsletter/* placeholder, got %d", resp.StatusCode)
	}
}

func TestNewsletterRootPathDoesNotRedirect(t *testing.T) {
	ts := httptest.NewServer(New())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter")
	if err != nil {
		t.Fatalf("GET /newsletter: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501 for /newsletter placeholder, got %d", resp.StatusCode)
	}
}

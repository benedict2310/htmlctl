package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

type fakeCaddyReloader struct {
	count int
	err   error
}

func (f *fakeCaddyReloader) Reload(ctx context.Context, reason string) error {
	f.count++
	return f.err
}

func TestDomainsCreateListGetDelete(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	reloader := &fakeCaddyReloader{}
	srv.caddyReloader = reloader

	body := bytes.NewBufferString(`{"domain":"Example.Com","website":"sample","environment":"staging"}`)
	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", body)
	if err != nil {
		t.Fatalf("POST /domains error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(b))
	}
	var created domainBindingResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()
	if created.Domain != "example.com" || created.Website != "sample" || created.Environment != "staging" {
		t.Fatalf("unexpected create response: %#v", created)
	}

	listResp, err := http.Get(baseURL + "/api/v1/domains?website=sample")
	if err != nil {
		t.Fatalf("GET /domains error = %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(listResp.Body)
		listResp.Body.Close()
		t.Fatalf("expected 200 list, got %d body=%s", listResp.StatusCode, string(b))
	}
	var listed domainBindingsResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		listResp.Body.Close()
		t.Fatalf("decode list response: %v", err)
	}
	listResp.Body.Close()
	if len(listed.Domains) != 1 || listed.Domains[0].Domain != "example.com" {
		t.Fatalf("unexpected listed domains: %#v", listed.Domains)
	}

	getResp, err := http.Get(baseURL + "/api/v1/domains/example.com")
	if err != nil {
		t.Fatalf("GET /domains/{domain} error = %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		t.Fatalf("expected 200 get, got %d body=%s", getResp.StatusCode, string(b))
	}
	var got domainBindingResponse
	if err := json.NewDecoder(getResp.Body).Decode(&got); err != nil {
		getResp.Body.Close()
		t.Fatalf("decode get response: %v", err)
	}
	getResp.Body.Close()
	if got.Domain != "example.com" {
		t.Fatalf("unexpected get response: %#v", got)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/example.com", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /domains/{domain} error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 delete, got %d", deleteResp.StatusCode)
	}

	listResp, err = http.Get(baseURL + "/api/v1/domains?website=sample")
	if err != nil {
		t.Fatalf("GET /domains after delete error = %v", err)
	}
	var empty domainBindingsResponse
	if err := json.NewDecoder(listResp.Body).Decode(&empty); err != nil {
		listResp.Body.Close()
		t.Fatalf("decode empty list response: %v", err)
	}
	listResp.Body.Close()
	if len(empty.Domains) != 0 {
		t.Fatalf("expected no domains after delete, got %#v", empty.Domains)
	}
	if reloader.count != 2 {
		t.Fatalf("expected 2 caddy reload calls, got %d", reloader.count)
	}
}

func TestDomainsRejectInvalidAndDuplicate(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	seedDomainWebsiteEnv(t, srv, "sample", "prod")
	srv.caddyReloader = &fakeCaddyReloader{}

	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"bad domain","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST invalid domain error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid domain, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST first domain error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for first bind, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"prod"}`))
	if err != nil {
		t.Fatalf("POST duplicate domain error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate bind, got %d", resp.StatusCode)
	}
}

func TestDomainsReloadFailureReturnsError(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{err: context.DeadlineExceeded}

	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST /domains error = %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 500 when reload fails, got %d body=%s", resp.StatusCode, string(b))
	}
	resp.Body.Close()

	q := dbpkg.NewQueries(srv.db)
	rows, err := q.ListDomainBindings(context.Background(), "sample", "staging")
	if err != nil {
		t.Fatalf("ListDomainBindings() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no persisted domain binding after rollback on reload failure, got %#v", rows)
	}
}

func seedDomainWebsiteEnv(t *testing.T, srv *Server, website, environment string) {
	t.Helper()
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteRow, err := q.GetWebsiteByName(ctx, website)
	if err != nil {
		websiteID, insErr := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
			Name:               website,
			DefaultStyleBundle: "default",
			BaseTemplate:       "default",
		})
		if insErr != nil {
			t.Fatalf("InsertWebsite(%q) error = %v", website, insErr)
		}
		websiteRow = dbpkg.WebsiteRow{ID: websiteID, Name: website}
	}
	if _, err := q.GetEnvironmentByName(ctx, websiteRow.ID, environment); err == nil {
		return
	}
	if _, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{
		WebsiteID: websiteRow.ID,
		Name:      environment,
	}); err != nil {
		t.Fatalf("InsertEnvironment(%q) error = %v", environment, err)
	}
}

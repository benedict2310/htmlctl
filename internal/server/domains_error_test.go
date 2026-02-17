package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestParseDomainItemPathInvalidCases(t *testing.T) {
	cases := []string{
		"/api/v1/domain/futurelab.studio",
		"/api/v1/domains",
		"/api/v1/domains/",
		"/api/v1/domains/futurelab.studio/extra",
	}
	for _, in := range cases {
		if domain, ok := parseDomainItemPath(in); ok || domain != "" {
			t.Fatalf("parseDomainItemPath(%q) = (%q,%v), expected invalid", in, domain, ok)
		}
	}
}

func TestDomainsMethodNotAllowed(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()

	req, err := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/domains", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /domains error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for collection, got %d", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodPut, baseURL+"/api/v1/domains/futurelab.studio", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /domains/{domain} error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for item, got %d", resp.StatusCode)
	}
}

func TestDomainsCreateBadRequestAndNotFoundBranches(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	srv.caddyReloader = &fakeCaddyReloader{}

	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{`))
	if err != nil {
		t.Fatalf("POST invalid json error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"futurelab.studio"}`))
	if err != nil {
		t.Fatalf("POST missing fields error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"futurelab.studio","website":"missing","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST missing website error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing website, got %d", resp.StatusCode)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteID, err := q.InsertWebsite(context.Background(), dbpkg.WebsiteRow{
		Name:               "futurelab",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	_, _ = websiteID, q

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"futurelab.studio","website":"futurelab","environment":"missing"}`))
	if err != nil {
		t.Fatalf("POST missing environment error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing environment, got %d", resp.StatusCode)
	}
}

func TestDomainsGetDeleteValidationAndReloadFailureBranch(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "futurelab", "staging")
	srv.caddyReloader = &fakeCaddyReloader{}

	createResp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"futurelab.studio","website":"futurelab","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST create domain error = %v", err)
	}
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 create, got %d", createResp.StatusCode)
	}

	getResp, err := http.Get(baseURL + "/api/v1/domains/not-a-domain")
	if err != nil {
		t.Fatalf("GET invalid domain error = %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid domain get, got %d", getResp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/not-a-domain", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE invalid domain error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid domain delete, got %d", deleteResp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/missing.example.com", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE missing domain error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing domain delete, got %d", deleteResp.StatusCode)
	}

	srv.caddyReloader = &fakeCaddyReloader{err: context.DeadlineExceeded}
	req, err = http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/futurelab.studio", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE existing domain error = %v", err)
	}
	body, _ := io.ReadAll(deleteResp.Body)
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for reload failure, got %d body=%s", deleteResp.StatusCode, string(body))
	}

	getResp, err = http.Get(baseURL + "/api/v1/domains/futurelab.studio")
	if err != nil {
		t.Fatalf("GET deleted domain error = %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected domain restored after failed reload, got status %d", getResp.StatusCode)
	}
}

func TestDomainsCreateDeleteWithoutReloader(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "futurelab", "staging")
	srv.caddyReloader = nil

	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"futurelab.studio","website":"futurelab","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST create domain error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 create without reloader, got %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/futurelab.studio", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE domain error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 delete without reloader, got %d", deleteResp.StatusCode)
	}
}

func TestDomainsInternalDatabaseErrors(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "futurelab", "staging")
	srv.caddyReloader = nil

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}

	listResp, err := http.Get(baseURL + "/api/v1/domains")
	if err != nil {
		t.Fatalf("GET /domains error = %v", err)
	}
	listResp.Body.Close()
	if listResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 list on closed db, got %d", listResp.StatusCode)
	}

	getResp, err := http.Get(baseURL + "/api/v1/domains/futurelab.studio")
	if err != nil {
		t.Fatalf("GET /domains/{domain} error = %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 get on closed db, got %d", getResp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/futurelab.studio", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /domains/{domain} error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 delete on closed db, got %d", deleteResp.StatusCode)
	}

	srv.db = nil
}

func TestIsDomainUniqueConstraintError(t *testing.T) {
	srv := startTestServer(t)
	seedDomainWebsiteEnv(t, srv, "futurelab", "staging")

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if _, err := q.InsertDomainBinding(context.Background(), dbpkg.DomainBindingRow{
		Domain:        "futurelab.studio",
		EnvironmentID: envRow.ID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}
	_, err = q.InsertDomainBinding(context.Background(), dbpkg.DomainBindingRow{
		Domain:        "futurelab.studio",
		EnvironmentID: envRow.ID,
	})
	if err == nil {
		t.Fatalf("expected duplicate insert error")
	}
	if !isDomainUniqueConstraintError(err) {
		t.Fatalf("expected unique constraint classification, got %v", err)
	}
	if isDomainUniqueConstraintError(fmt.Errorf("not unique")) {
		t.Fatalf("expected non-unique error to be rejected")
	}
}

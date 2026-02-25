package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestParseDomainItemPathInvalidCases(t *testing.T) {
	cases := []string{
		"/api/v1/domain/example.com",
		"/api/v1/domains",
		"/api/v1/domains/",
		"/api/v1/domains/   ",
		"/api/v1/domains/example.com/extra",
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

	req, err = http.NewRequest(http.MethodPut, baseURL+"/api/v1/domains/example.com", nil)
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

	notFoundResp, err := http.Get(baseURL + "/api/v1/domains/example.com/extra")
	if err != nil {
		t.Fatalf("GET invalid item path error = %v", err)
	}
	notFoundResp.Body.Close()
	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid item path, got %d", notFoundResp.StatusCode)
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

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com"}`))
	if err != nil {
		t.Fatalf("POST missing fields error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"missing","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST missing website error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing website, got %d", resp.StatusCode)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteID, err := q.InsertWebsite(context.Background(), dbpkg.WebsiteRow{
		Name:               "sample",
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	})
	if err != nil {
		t.Fatalf("InsertWebsite() error = %v", err)
	}
	_, _ = websiteID, q

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"missing"}`))
	if err != nil {
		t.Fatalf("POST missing environment error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing environment, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"future.lab","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST invalid website name error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid website name, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString("{\"domain\":\"example.com\",\"website\":\"sample\",\"environment\":\"staging\\nprod\"}"))
	if err != nil {
		t.Fatalf("POST invalid environment name error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid environment name, got %d", resp.StatusCode)
	}
}

func TestDomainsGetDeleteValidationAndReloadFailureBranch(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{}

	createResp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST create domain error = %v", err)
	}
	var created domainBindingResponse
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		createResp.Body.Close()
		t.Fatalf("decode create response: %v", err)
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
	req, err = http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/example.com", nil)
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

	getResp, err = http.Get(baseURL + "/api/v1/domains/example.com")
	if err != nil {
		t.Fatalf("GET deleted domain error = %v", err)
	}
	var restored domainBindingResponse
	if err := json.NewDecoder(getResp.Body).Decode(&restored); err != nil {
		getResp.Body.Close()
		t.Fatalf("decode restored domain response: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected domain restored after failed reload, got status %d", getResp.StatusCode)
	}
	if restored.ID != created.ID || restored.CreatedAt != created.CreatedAt || restored.UpdatedAt != created.UpdatedAt {
		t.Fatalf("expected restored metadata to match created metadata, created=%#v restored=%#v", created, restored)
	}
}

func TestDomainsCreateDeleteWithoutReloader(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = nil

	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST create domain error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 create without reloader, got %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/example.com", nil)
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
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
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

	getResp, err := http.Get(baseURL + "/api/v1/domains/example.com")
	if err != nil {
		t.Fatalf("GET /domains/{domain} error = %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 get on closed db, got %d", getResp.StatusCode)
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
	if deleteResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 delete on closed db, got %d", deleteResp.StatusCode)
	}

	srv.db = nil
}

func TestIsDomainUniqueConstraintError(t *testing.T) {
	srv := startTestServer(t)
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if _, err := q.InsertDomainBinding(context.Background(), dbpkg.DomainBindingRow{
		Domain:        "example.com",
		EnvironmentID: envRow.ID,
	}); err != nil {
		t.Fatalf("InsertDomainBinding() error = %v", err)
	}
	_, err = q.InsertDomainBinding(context.Background(), dbpkg.DomainBindingRow{
		Domain:        "example.com",
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

type blockingDeleteReloadFailure struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingDeleteReloadFailure) Reload(ctx context.Context, reason string) error {
	if !strings.HasPrefix(reason, "domain.remove ") {
		return nil
	}
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	select {
	case <-b.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return context.DeadlineExceeded
}

func TestDomainsSameDomainDeleteCreateSerialized(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{}

	createResp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST create seed domain error = %v", err)
	}
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for seed domain, got %d", createResp.StatusCode)
	}

	reloader := &blockingDeleteReloadFailure{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	srv.caddyReloader = reloader

	deleteResultCh := make(chan struct {
		status int
		body   string
		err    error
	}, 1)
	go func() {
		req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/domains/example.com", nil)
		if err != nil {
			deleteResultCh <- struct {
				status int
				body   string
				err    error
			}{err: err}
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			deleteResultCh <- struct {
				status int
				body   string
				err    error
			}{err: err}
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		deleteResultCh <- struct {
			status int
			body   string
			err    error
		}{status: resp.StatusCode, body: string(body)}
	}()

	select {
	case <-reloader.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for delete reload to start")
	}

	createResultCh := make(chan struct {
		status int
		body   string
		err    error
	}, 1)
	go func() {
		resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
		if err != nil {
			createResultCh <- struct {
				status int
				body   string
				err    error
			}{err: err}
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		createResultCh <- struct {
			status int
			body   string
			err    error
		}{status: resp.StatusCode, body: string(body)}
	}()

	select {
	case result := <-createResultCh:
		close(reloader.release)
		t.Fatalf("create completed while delete held same-domain lock: status=%d body=%s err=%v", result.status, result.body, result.err)
	case <-time.After(100 * time.Millisecond):
	}

	close(reloader.release)

	deleteResult := <-deleteResultCh
	if deleteResult.err != nil {
		t.Fatalf("DELETE domain error = %v", deleteResult.err)
	}
	if deleteResult.status != http.StatusInternalServerError {
		t.Fatalf("expected delete 500 during reload failure, got %d body=%s", deleteResult.status, deleteResult.body)
	}

	createResult := <-createResultCh
	if createResult.err != nil {
		t.Fatalf("POST concurrent create error = %v", createResult.err)
	}
	if createResult.status != http.StatusConflict {
		t.Fatalf("expected concurrent create 409 after serialized rollback, got %d body=%s", createResult.status, createResult.body)
	}
}

type rollbackFailureThenReconcileSuccessReloader struct {
	srv   *Server
	calls int
}

func (r *rollbackFailureThenReconcileSuccessReloader) Reload(ctx context.Context, reason string) error {
	r.calls++
	switch r.calls {
	case 1:
		if err := r.srv.db.Close(); err != nil {
			return err
		}
		return context.DeadlineExceeded
	case 2:
		return nil
	default:
		return nil
	}
}

func TestDomainsCreateUsesKnownRowAfterReconcileRecovery(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	reloader := &rollbackFailureThenReconcileSuccessReloader{srv: srv}
	srv.caddyReloader = reloader

	resp, err := http.Post(baseURL+"/api/v1/domains", "application/json", bytes.NewBufferString(`{"domain":"example.com","website":"sample","environment":"staging"}`))
	if err != nil {
		t.Fatalf("POST create domain error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("expected 201 after reconcile recovery, got %d body=%s", resp.StatusCode, string(body))
	}
	var out domainBindingResponse
	if err := json.Unmarshal(body, &out); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v body=%s", err, string(body))
	}
	resp.Body.Close()
	if out.Domain != "example.com" || out.Website != "sample" || out.Environment != "staging" {
		t.Fatalf("unexpected create response after recovery: %#v", out)
	}
	if reloader.calls < 2 {
		t.Fatalf("expected initial reload + reconcile reload, got %d call(s)", reloader.calls)
	}
	if err := srv.db.PingContext(context.Background()); err == nil {
		t.Fatalf("expected closed database after simulated rollback failure")
	}

	// Prevent shutdown from trying to close the already closed database again.
	srv.db = nil
}

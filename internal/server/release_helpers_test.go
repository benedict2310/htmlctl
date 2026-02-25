package server

import (
	"net/http/httptest"
	"testing"
)

func TestParseListReleasesPagination(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/websites/sample/environments/staging/releases", nil)
	limit, offset, err := parseListReleasesPagination(req)
	if err != nil {
		t.Fatalf("parseListReleasesPagination(default) error = %v", err)
	}
	if limit != 20 || offset != 0 {
		t.Fatalf("unexpected default pagination: limit=%d offset=%d", limit, offset)
	}

	req = httptest.NewRequest("GET", "/api/v1/websites/sample/environments/staging/releases?limit=999&offset=3", nil)
	limit, offset, err = parseListReleasesPagination(req)
	if err != nil {
		t.Fatalf("parseListReleasesPagination(clamped) error = %v", err)
	}
	if limit != 200 || offset != 3 {
		t.Fatalf("unexpected clamped pagination: limit=%d offset=%d", limit, offset)
	}

	req = httptest.NewRequest("GET", "/api/v1/websites/sample/environments/staging/releases?limit=-1", nil)
	if _, _, err := parseListReleasesPagination(req); err == nil {
		t.Fatalf("expected negative limit error")
	}
	req = httptest.NewRequest("GET", "/api/v1/websites/sample/environments/staging/releases?offset=-1", nil)
	if _, _, err := parseListReleasesPagination(req); err == nil {
		t.Fatalf("expected negative offset error")
	}
}

func TestParseReleaseRollbackPromotePathsRejectInvalid(t *testing.T) {
	if _, _, ok, _ := parseReleasePath("/api/v1/websites/sample/environments/staging/release"); ok {
		t.Fatalf("expected invalid release path to fail parsing")
	}
	if _, _, ok, _ := parseReleasePath("/api/v1/websites//environments/staging/releases"); ok {
		t.Fatalf("expected empty website release path to fail parsing")
	}
	if _, _, ok, _ := parseRollbackPath("/api/v1/websites/sample/environments/staging/rollbacks"); ok {
		t.Fatalf("expected invalid rollback path to fail parsing")
	}
	if _, _, ok, _ := parseRollbackPath("/api/v1/websites/sample/environments//rollback"); ok {
		t.Fatalf("expected empty env rollback path to fail parsing")
	}
	if _, ok, _ := parsePromotePath("/api/v1/websites//promote"); ok {
		t.Fatalf("expected invalid promote path to fail parsing")
	}
	if _, ok, _ := parsePromotePath("/api/v1/website/sample/promote"); ok {
		t.Fatalf("expected malformed promote path to fail parsing")
	}
}

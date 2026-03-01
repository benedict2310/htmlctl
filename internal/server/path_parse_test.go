package server

import "testing"

func TestParsePathsWithValidNames(t *testing.T) {
	website := "sample"
	env := "staging-1"

	if gotWebsite, gotEnv, ok, err := parseApplyPath("/api/v1/websites/" + website + "/environments/" + env + "/apply"); !ok || err != nil || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseApplyPath() = (%q, %q, %v, %v)", gotWebsite, gotEnv, ok, err)
	}
	if gotWebsite, gotEnv, ok, err := parseReleasePath("/api/v1/websites/" + website + "/environments/" + env + "/releases"); !ok || err != nil || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseReleasePath() = (%q, %q, %v, %v)", gotWebsite, gotEnv, ok, err)
	}
	if gotWebsite, gotEnv, ok, err := parseRollbackPath("/api/v1/websites/" + website + "/environments/" + env + "/rollback"); !ok || err != nil || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseRollbackPath() = (%q, %q, %v, %v)", gotWebsite, gotEnv, ok, err)
	}
	if gotWebsite, gotEnv, ok, err := parseStatusPath("/api/v1/websites/" + website + "/environments/" + env + "/status"); !ok || err != nil || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseStatusPath() = (%q, %q, %v, %v)", gotWebsite, gotEnv, ok, err)
	}
	if gotWebsite, gotEnv, ok, err := parseManifestPath("/api/v1/websites/" + website + "/environments/" + env + "/manifest"); !ok || err != nil || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseManifestPath() = (%q, %q, %v, %v)", gotWebsite, gotEnv, ok, err)
	}
	if gotWebsite, gotEnv, ok, err := parseBackendsPath("/api/v1/websites/" + website + "/environments/" + env + "/backends"); !ok || err != nil || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseBackendsPath() = (%q, %q, %v, %v)", gotWebsite, gotEnv, ok, err)
	}
	if gotWebsite, gotEnv, envScoped, ok, err := parseLogsPath("/api/v1/websites/" + website + "/environments/" + env + "/logs"); !ok || err != nil || !envScoped || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseLogsPath(env) = (%q, %q, %v, %v, %v)", gotWebsite, gotEnv, envScoped, ok, err)
	}
	if gotWebsite, gotEnv, envScoped, ok, err := parseLogsPath("/api/v1/websites/" + website + "/logs"); !ok || err != nil || envScoped || gotWebsite != website || gotEnv != "" {
		t.Fatalf("parseLogsPath(website) = (%q, %q, %v, %v, %v)", gotWebsite, gotEnv, envScoped, ok, err)
	}
	if gotWebsite, ok, err := parseEnvironmentsPath("/api/v1/websites/" + website + "/environments"); !ok || err != nil || gotWebsite != website {
		t.Fatalf("parseEnvironmentsPath() = (%q, %v, %v)", gotWebsite, ok, err)
	}
	if gotWebsite, ok, err := parsePromotePath("/api/v1/websites/" + website + "/promote"); !ok || err != nil || gotWebsite != website {
		t.Fatalf("parsePromotePath() = (%q, %v, %v)", gotWebsite, ok, err)
	}
	if gotDomain, ok := parseDomainItemPath("/api/v1/domains/example.com"); !ok || gotDomain != "example.com" {
		t.Fatalf("parseDomainItemPath() = (%q, %v)", gotDomain, ok)
	}
}

func TestParsePathsRejectInvalidNames(t *testing.T) {
	if _, _, ok, err := parseApplyPath("/api/v1/websites/future.lab/environments/staging/apply"); ok || err == nil {
		t.Fatalf("expected apply path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := parseReleasePath("/api/v1/websites/sample/environments/staging%1/releases"); ok || err == nil {
		t.Fatalf("expected release path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := parseRollbackPath("/api/v1/websites/sample/environments/../rollback"); ok || err == nil {
		t.Fatalf("expected rollback path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, ok, err := parsePromotePath("/api/v1/websites/future%lab/promote"); ok || err == nil {
		t.Fatalf("expected promote path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := parseStatusPath("/api/v1/websites/future.lab/environments/staging/status"); ok || err == nil {
		t.Fatalf("expected status path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := parseManifestPath("/api/v1/websites/sample/environments/staging%1/manifest"); ok || err == nil {
		t.Fatalf("expected manifest path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := parseBackendsPath("/api/v1/websites/sample/environments/staging%1/backends"); ok || err == nil {
		t.Fatalf("expected backends path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, _, ok, err := parseLogsPath("/api/v1/websites/future%lab/logs"); ok || err == nil {
		t.Fatalf("expected website logs path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, _, _, ok, err := parseLogsPath("/api/v1/websites/sample/environments/staging%1/logs"); ok || err == nil {
		t.Fatalf("expected environment logs path name validation error, got ok=%v err=%v", ok, err)
	}
	if _, ok, err := parseEnvironmentsPath("/api/v1/websites/future.lab/environments"); ok || err == nil {
		t.Fatalf("expected environments path name validation error, got ok=%v err=%v", ok, err)
	}
}

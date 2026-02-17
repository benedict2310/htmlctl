package server

import "testing"

func TestParsePathsAllowPercentInDecodedSegments(t *testing.T) {
	website := "future%lab"
	env := "staging%1"

	if gotWebsite, gotEnv, ok := parseApplyPath("/api/v1/websites/" + website + "/environments/" + env + "/apply"); !ok || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseApplyPath() = (%q, %q, %v)", gotWebsite, gotEnv, ok)
	}
	if gotWebsite, gotEnv, ok := parseReleasePath("/api/v1/websites/" + website + "/environments/" + env + "/releases"); !ok || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseReleasePath() = (%q, %q, %v)", gotWebsite, gotEnv, ok)
	}
	if gotWebsite, gotEnv, ok := parseRollbackPath("/api/v1/websites/" + website + "/environments/" + env + "/rollback"); !ok || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseRollbackPath() = (%q, %q, %v)", gotWebsite, gotEnv, ok)
	}
	if gotWebsite, gotEnv, ok := parseStatusPath("/api/v1/websites/" + website + "/environments/" + env + "/status"); !ok || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseStatusPath() = (%q, %q, %v)", gotWebsite, gotEnv, ok)
	}
	if gotWebsite, gotEnv, ok := parseManifestPath("/api/v1/websites/" + website + "/environments/" + env + "/manifest"); !ok || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseManifestPath() = (%q, %q, %v)", gotWebsite, gotEnv, ok)
	}
	if gotWebsite, gotEnv, envScoped, ok := parseLogsPath("/api/v1/websites/" + website + "/environments/" + env + "/logs"); !ok || !envScoped || gotWebsite != website || gotEnv != env {
		t.Fatalf("parseLogsPath(env) = (%q, %q, %v, %v)", gotWebsite, gotEnv, envScoped, ok)
	}
	if gotWebsite, gotEnv, envScoped, ok := parseLogsPath("/api/v1/websites/" + website + "/logs"); !ok || envScoped || gotWebsite != website || gotEnv != "" {
		t.Fatalf("parseLogsPath(website) = (%q, %q, %v, %v)", gotWebsite, gotEnv, envScoped, ok)
	}
	if gotWebsite, ok := parseEnvironmentsPath("/api/v1/websites/" + website + "/environments"); !ok || gotWebsite != website {
		t.Fatalf("parseEnvironmentsPath() = (%q, %v)", gotWebsite, ok)
	}
	if gotWebsite, ok := parsePromotePath("/api/v1/websites/" + website + "/promote"); !ok || gotWebsite != website {
		t.Fatalf("parsePromotePath() = (%q, %v)", gotWebsite, ok)
	}
	if gotDomain, ok := parseDomainItemPath("/api/v1/domains/futurelab.studio"); !ok || gotDomain != "futurelab.studio" {
		t.Fatalf("parseDomainItemPath() = (%q, %v)", gotDomain, ok)
	}
}

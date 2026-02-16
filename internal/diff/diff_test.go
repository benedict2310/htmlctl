package diff

import "testing"

func TestComputeMixedChanges(t *testing.T) {
	local := []FileRecord{
		{Path: "components/header.html", Hash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{Path: "pages/index.page.yaml", Hash: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		{Path: "styles/default.css", Hash: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}
	remote := []FileRecord{
		{Path: "components/header.html", Hash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{Path: "pages/index.page.yaml", Hash: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		{Path: "assets/logo.svg", Hash: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
	}

	out, err := Compute(local, remote)
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if out.Summary.Added != 1 || out.Summary.Modified != 1 || out.Summary.Removed != 1 || out.Summary.Unchanged != 1 {
		t.Fatalf("unexpected summary: %#v", out.Summary)
	}
	if len(out.Changes) != 3 {
		t.Fatalf("expected 3 changed files, got %d", len(out.Changes))
	}
}

func TestComputeEmptyStates(t *testing.T) {
	out, err := Compute(nil, nil)
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if out.HasChanges() {
		t.Fatalf("expected no changes")
	}
	if out.Summary.Unchanged != 0 {
		t.Fatalf("expected unchanged 0, got %d", out.Summary.Unchanged)
	}
}

func TestComputeRejectsConflictingDuplicatePaths(t *testing.T) {
	_, err := Compute(
		[]FileRecord{
			{Path: "assets/logo.svg", Hash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{Path: "assets/logo.svg", Hash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		},
		nil,
	)
	if err == nil {
		t.Fatalf("expected duplicate hash conflict error")
	}
}

func TestResourceTypeDetection(t *testing.T) {
	cases := map[string]string{
		"pages/index.page.yaml":    "pages",
		"components/header.html":   "components",
		"styles/default.css":       "styles",
		"scripts/site.js":          "scripts",
		"assets/logo.svg":          "assets",
		"app.mjs":                  "scripts",
		"images/hero.jpeg":         "assets",
		"pages\\nested\\home.yaml": "pages",
	}
	for path, want := range cases {
		if got := resourceTypeForPath(path); got != want {
			t.Fatalf("resourceTypeForPath(%q)=%q want %q", path, got, want)
		}
	}
}

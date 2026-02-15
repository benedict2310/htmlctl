package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestValidateComponentRules(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		componentName string
		cfg           Config
		wantErrRule   string
		wantErrText   string
	}{
		{name: "valid section", file: "valid-section.html", componentName: "pricing", cfg: Config{RequireAnchorID: true, ExpectedAnchorID: "pricing"}},
		{name: "valid div", file: "valid-div.html", componentName: "content", cfg: DefaultConfig()},
		{name: "multi root", file: "multi-root.html", componentName: "multi", cfg: DefaultConfig(), wantErrRule: "single-root", wantErrText: "multiple root elements"},
		{name: "text root", file: "text-root.html", componentName: "text-root", cfg: DefaultConfig(), wantErrRule: "single-root", wantErrText: "text is not allowed"},
		{name: "bad tag", file: "bad-tag.html", componentName: "bad-tag", cfg: DefaultConfig(), wantErrRule: "root-tag-allowlist", wantErrText: "not allowed"},
		{name: "script direct", file: "has-script.html", componentName: "has-script", cfg: DefaultConfig(), wantErrRule: "script-disallow", wantErrText: "scripts/site.js"},
		{name: "script nested", file: "nested-script.html", componentName: "nested-script", cfg: DefaultConfig(), wantErrRule: "script-disallow", wantErrText: "scripts/site.js"},
		{name: "missing anchor id", file: "missing-id.html", componentName: "pricing", cfg: Config{RequireAnchorID: true, ExpectedAnchorID: "pricing"}, wantErrRule: "anchor-id", wantErrText: "must include id"},
		{name: "wrong anchor id", file: "wrong-id.html", componentName: "pricing", cfg: Config{RequireAnchorID: true, ExpectedAnchorID: "pricing"}, wantErrRule: "anchor-id", wantErrText: "expected \"pricing\", got \"wrong\""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html := mustReadComponentFixture(t, tc.file)
			cfg := tc.cfg
			if len(cfg.AllowedRootTags) == 0 {
				cfg.AllowedRootTags = DefaultConfig().AllowedRootTags
			}

			errs := ValidateComponentWithConfig(&model.Component{Name: tc.componentName, HTML: html}, cfg)
			if tc.wantErrRule == "" {
				if len(errs) != 0 {
					t.Fatalf("expected no errors, got: %v", errs)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected error rule %q, got none", tc.wantErrRule)
			}
			matched := false
			for _, err := range errs {
				if err.Rule == tc.wantErrRule && strings.Contains(err.Message, tc.wantErrText) {
					matched = true
					break
				}
			}
			if !matched {
				t.Fatalf("expected error rule=%q containing %q, got: %#v", tc.wantErrRule, tc.wantErrText, errs)
			}
		})
	}
}

func TestValidateComponentEmptyAndWhitespaceFail(t *testing.T) {
	for _, html := range []string{"", "\n\t  \n"} {
		errs := ValidateComponent(&model.Component{Name: "empty", HTML: html})
		if len(errs) == 0 {
			t.Fatalf("expected errors for empty/whitespace component")
		}
		if errs[0].Rule != "single-root" {
			t.Fatalf("expected single-root error, got %#v", errs)
		}
	}
}

func TestCommentsAtRootAreIgnored(t *testing.T) {
	html := "<!-- comment -->\n<section id=\"x\"></section>\n"
	errs := ValidateComponent(&model.Component{Name: "x", HTML: html})
	if len(errs) != 0 {
		t.Fatalf("expected comment to be ignored, got: %v", errs)
	}
}

func TestCustomAllowlistOverride(t *testing.T) {
	errs := ValidateComponentWithConfig(&model.Component{Name: "aside", HTML: "<aside id=\"aside\"></aside>"}, Config{
		AllowedRootTags: []string{"aside"},
	})
	if len(errs) != 0 {
		t.Fatalf("expected custom allowlist to accept aside, got: %v", errs)
	}
}

func mustReadComponentFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "components", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(b)
}

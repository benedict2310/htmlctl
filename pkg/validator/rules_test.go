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

func TestValidateComponentRejectsInlineEventHandlers(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		wantErrText string
	}{
		{
			name:        "reject onerror",
			html:        `<section id="x"><img src="x" onerror="evil()"></section>`,
			wantErrText: `attribute "onerror"`,
		},
		{
			name:        "reject onclick",
			html:        `<section id="x" onclick="evil()">text</section>`,
			wantErrText: `attribute "onclick"`,
		},
		{
			name:        "reject nested onclick",
			html:        `<section id="x"><div><p onclick="evil()">text</p></div></section>`,
			wantErrText: `attribute "onclick"`,
		},
		{
			name:        "reject onload",
			html:        `<section id="x"><img src="x" onload="evil()"></section>`,
			wantErrText: `attribute "onload"`,
		},
		{
			name:        "reject onmouseover",
			html:        `<section id="x"><div onmouseover="evil()">text</div></section>`,
			wantErrText: `attribute "onmouseover"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateComponent(&model.Component{Name: "x", HTML: tc.html})
			if len(errs) == 0 {
				t.Fatalf("expected event-handler validation error, got none")
			}
			found := false
			for _, err := range errs {
				if err.Rule == "event-handler-disallow" && strings.Contains(err.Message, tc.wantErrText) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected event-handler-disallow containing %q, got: %#v", tc.wantErrText, errs)
			}
		})
	}
}

func TestValidateComponentAllowsNonEventAttributes(t *testing.T) {
	errs := ValidateComponent(&model.Component{
		Name: "x",
		HTML: `<section id="x"><a href="page.html" class="btn" style="color:red">link</a></section>`,
	})
	if len(errs) != 0 {
		t.Fatalf("expected non-event attributes to be accepted, got: %v", errs)
	}
}

func TestValidateComponentReportsAllUnsafeHTMLViolations(t *testing.T) {
	errs := ValidateComponent(&model.Component{
		Name: "x",
		HTML: `<section id="x" onclick="evil()"><script>bad()</script></section>`,
	})
	if len(errs) < 2 {
		t.Fatalf("expected multiple unsafe html violations, got: %#v", errs)
	}

	var hasScript, hasEvent bool
	for _, err := range errs {
		if err.Rule == "script-disallow" {
			hasScript = true
		}
		if err.Rule == "event-handler-disallow" {
			hasEvent = true
		}
	}
	if !hasScript || !hasEvent {
		t.Fatalf("expected both script-disallow and event-handler-disallow, got: %#v", errs)
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

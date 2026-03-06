package release

import (
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestGenerateLLMsTxt_DeterministicContent(t *testing.T) {
	website := model.Website{
		Metadata: model.Metadata{Name: "sample"},
		Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
			PublicBaseURL: "https://example.com",
			DisplayName:   "Sample Studio",
			Description:   "Docs and product pages.",
			LLMsTxt:       &model.WebsiteLLMsTxt{Enabled: true},
		}},
	}
	pages := map[string]model.Page{
		"index": {
			Metadata: model.Metadata{Name: "index"},
			Spec: model.PageSpec{
				Route:       "/",
				Title:       "Home",
				Description: "Landing page",
			},
		},
		"docs": {
			Metadata: model.Metadata{Name: "docs"},
			Spec: model.PageSpec{
				Route:       "/docs",
				Title:       "Docs",
				Description: "Guides",
			},
		},
	}

	got, err := GenerateLLMsTxt(website, pages, newBuildLog())
	if err != nil {
		t.Fatalf("GenerateLLMsTxt() error = %v", err)
	}
	want := strings.Join([]string{
		"# Sample Studio",
		"",
		"> Docs and product pages.",
		"",
		"## Pages",
		"",
		"- [Home](https://example.com/): Landing page",
		"- [Docs](https://example.com/docs): Guides",
		"",
	}, "\n")
	if string(got) != want {
		t.Fatalf("unexpected llms.txt:\n%s", string(got))
	}
}

func TestGenerateLLMsTxt_ExcludesNoindexPages(t *testing.T) {
	website := model.Website{
		Metadata: model.Metadata{Name: "sample"},
		Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
			PublicBaseURL: "https://example.com",
			LLMsTxt:       &model.WebsiteLLMsTxt{Enabled: true},
		}},
	}
	pages := map[string]model.Page{
		"index": {
			Metadata: model.Metadata{Name: "index"},
			Spec:     model.PageSpec{Route: "/", Title: "Home"},
		},
		"draft": {
			Metadata: model.Metadata{Name: "draft"},
			Spec: model.PageSpec{
				Route: "/draft",
				Title: "Draft",
				Head: &model.PageHead{Meta: map[string]string{
					"robots": "noindex,follow",
				}},
			},
		},
	}

	got, err := GenerateLLMsTxt(website, pages, newBuildLog())
	if err != nil {
		t.Fatalf("GenerateLLMsTxt() error = %v", err)
	}
	text := string(got)
	if strings.Contains(text, "Draft") {
		t.Fatalf("expected noindex page to be excluded, got:\n%s", text)
	}
	if !strings.Contains(text, "Home") {
		t.Fatalf("expected index page to remain included, got:\n%s", text)
	}
}

func TestGenerateLLMsTxt_OmitsDescriptionClauseWhenPageDescriptionEmpty(t *testing.T) {
	website := model.Website{
		Metadata: model.Metadata{Name: "sample"},
		Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
			PublicBaseURL: "https://example.com",
			LLMsTxt:       &model.WebsiteLLMsTxt{Enabled: true},
		}},
	}
	pages := map[string]model.Page{
		"index": {
			Metadata: model.Metadata{Name: "index"},
			Spec:     model.PageSpec{Route: "/", Title: "Home"},
		},
	}

	got, err := GenerateLLMsTxt(website, pages, newBuildLog())
	if err != nil {
		t.Fatalf("GenerateLLMsTxt() error = %v", err)
	}
	if strings.Contains(string(got), "Home](https://example.com/):") {
		t.Fatalf("expected no description clause when page description is empty, got:\n%s", string(got))
	}
}

func TestGenerateLLMsTxt_ValidationAndDisabledCases(t *testing.T) {
	tests := []struct {
		name      string
		website   model.Website
		wantError string
		wantNil   bool
	}{
		{
			name: "disabled",
			website: model.Website{Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
				PublicBaseURL: "https://example.com",
				LLMsTxt:       &model.WebsiteLLMsTxt{Enabled: false},
			}}},
			wantNil: true,
		},
		{
			name: "missing public base",
			website: model.Website{Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
				LLMsTxt: &model.WebsiteLLMsTxt{Enabled: true},
			}}},
			wantError: "publicBaseURL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GenerateLLMsTxt(tc.website, map[string]model.Page{}, newBuildLog())
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateLLMsTxt() error = %v", err)
			}
			if tc.wantNil && got != nil {
				t.Fatalf("expected nil llms.txt content, got %q", string(got))
			}
		})
	}
}

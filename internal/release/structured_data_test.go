package release

import (
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestGenerateWebsiteStructuredDataBlocks(t *testing.T) {
	website := model.Website{
		Metadata: model.Metadata{Name: "sample"},
		Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
			PublicBaseURL:  "https://example.com/",
			DisplayName:    "Sample Studio",
			StructuredData: &model.WebsiteStructuredData{Enabled: true},
		}},
	}

	blocks, err := GenerateWebsiteStructuredDataBlocks(website)
	if err != nil {
		t.Fatalf("GenerateWebsiteStructuredDataBlocks() error = %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Payload["@type"] != "Organization" {
		t.Fatalf("expected first block type Organization, got %#v", blocks[0].Payload["@type"])
	}
	if blocks[1].Payload["@type"] != "WebSite" {
		t.Fatalf("expected second block type WebSite, got %#v", blocks[1].Payload["@type"])
	}
	for _, block := range blocks {
		if block.Payload["name"] != "Sample Studio" {
			t.Fatalf("expected block name Sample Studio, got %#v", block.Payload["name"])
		}
		if block.Payload["url"] != "https://example.com/" {
			t.Fatalf("expected normalized public base url, got %#v", block.Payload["url"])
		}
	}
}

func TestInjectWebsiteStructuredData_PrependsAndSkipsDuplicates(t *testing.T) {
	site := &model.Site{
		Website: model.Website{
			Metadata: model.Metadata{Name: "sample"},
			Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
				PublicBaseURL:  "https://example.com",
				StructuredData: &model.WebsiteStructuredData{Enabled: true},
			}},
		},
		Pages: map[string]model.Page{
			"index": {
				Metadata: model.Metadata{Name: "index"},
				Spec: model.PageSpec{
					Route: "/",
					Head: &model.PageHead{JSONLD: []model.JSONLDBlock{{
						ID: "manual-app",
						Payload: map[string]any{
							"@type": "SoftwareApplication",
							"name":  "Ora",
						},
					}}},
				},
			},
			"docs": {
				Metadata: model.Metadata{Name: "docs"},
				Spec: model.PageSpec{
					Route: "/docs",
					Head: &model.PageHead{JSONLD: []model.JSONLDBlock{{
						ID: "manual-org",
						Payload: map[string]any{
							"@type": "Organization",
							"name":  "Manual Org",
						},
					}}},
				},
			},
		},
	}

	if err := injectWebsiteStructuredData(site, newBuildLog()); err != nil {
		t.Fatalf("injectWebsiteStructuredData() error = %v", err)
	}

	indexBlocks := site.Pages["index"].Spec.Head.JSONLD
	if len(indexBlocks) != 3 {
		t.Fatalf("expected 3 index blocks, got %d", len(indexBlocks))
	}
	if indexBlocks[0].Payload["@type"] != "Organization" || indexBlocks[1].Payload["@type"] != "WebSite" {
		t.Fatalf("expected website blocks to be prepended in deterministic order, got %#v %#v", indexBlocks[0].Payload["@type"], indexBlocks[1].Payload["@type"])
	}
	if indexBlocks[2].Payload["@type"] != "SoftwareApplication" {
		t.Fatalf("expected existing page JSON-LD to remain last, got %#v", indexBlocks[2].Payload["@type"])
	}

	docsBlocks := site.Pages["docs"].Spec.Head.JSONLD
	if len(docsBlocks) != 2 {
		t.Fatalf("expected 2 docs blocks, got %d", len(docsBlocks))
	}
	if docsBlocks[0].Payload["@type"] != "WebSite" {
		t.Fatalf("expected only missing WebSite block to be prepended, got %#v", docsBlocks[0].Payload["@type"])
	}
	if docsBlocks[1].Payload["@type"] != "Organization" {
		t.Fatalf("expected existing Organization block to remain, got %#v", docsBlocks[1].Payload["@type"])
	}
}

func TestGenerateWebsiteStructuredDataBlocks_ValidationAndDisabled(t *testing.T) {
	tests := []struct {
		name      string
		website   model.Website
		wantError string
		wantNil   bool
	}{
		{
			name: "disabled",
			website: model.Website{Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
				PublicBaseURL:  "https://example.com",
				StructuredData: &model.WebsiteStructuredData{Enabled: false},
			}}},
			wantNil: true,
		},
		{
			name: "missing public base",
			website: model.Website{Spec: model.WebsiteSpec{SEO: &model.WebsiteSEO{
				StructuredData: &model.WebsiteStructuredData{Enabled: true},
			}}},
			wantError: "publicBaseURL",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blocks, err := GenerateWebsiteStructuredDataBlocks(tc.website)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateWebsiteStructuredDataBlocks() error = %v", err)
			}
			if tc.wantNil && blocks != nil {
				t.Fatalf("expected nil blocks, got %#v", blocks)
			}
		})
	}
}

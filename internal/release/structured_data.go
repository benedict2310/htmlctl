package release

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/model"
)

func GenerateWebsiteStructuredDataBlocks(website model.Website) ([]model.JSONLDBlock, error) {
	seo := website.Spec.SEO
	if seo == nil || seo.StructuredData == nil || !seo.StructuredData.Enabled {
		return nil, nil
	}

	publicBaseURL, err := loader.NormalizePublicBaseURL(seo.PublicBaseURL)
	if err != nil {
		return nil, fmt.Errorf("normalize structuredData publicBaseURL: %w", err)
	}

	name := strings.TrimSpace(seo.DisplayName)
	if name == "" {
		name = strings.TrimSpace(website.Metadata.Name)
	}
	if name == "" {
		name = "Website"
	}

	blocks := []model.JSONLDBlock{
		{
			ID: "website-organization",
			Payload: map[string]any{
				"@context": "https://schema.org",
				"@type":    "Organization",
				"name":     name,
				"url":      publicBaseURL,
			},
		},
		{
			ID: "website-web-site",
			Payload: map[string]any{
				"@context": "https://schema.org",
				"@type":    "WebSite",
				"name":     name,
				"url":      publicBaseURL,
			},
		},
	}
	return blocks, nil
}

func injectWebsiteStructuredData(site *model.Site, log *buildLog) error {
	if site == nil {
		return fmt.Errorf("site is required")
	}
	websiteBlocks, err := GenerateWebsiteStructuredDataBlocks(site.Website)
	if err != nil {
		return err
	}
	if len(websiteBlocks) == 0 {
		return nil
	}

	for _, pageName := range sortedPageNames(site.Pages) {
		page := site.Pages[pageName]
		if page.Spec.Head == nil {
			page.Spec.Head = &model.PageHead{}
		}
		existingTypes := jsonLDTypes(page.Spec.Head.JSONLD)

		toInject := make([]model.JSONLDBlock, 0, len(websiteBlocks))
		for _, block := range websiteBlocks {
			blockTypes := jsonLDTypes([]model.JSONLDBlock{block})
			skip := false
			for blockType := range blockTypes {
				if _, ok := existingTypes[blockType]; ok {
					skip = true
					if log != nil {
						log.Addf("info: page=%s website structuredData block skipped for existing type=%s", pageName, blockType)
					}
					break
				}
			}
			if skip {
				continue
			}
			toInject = append(toInject, block)
		}
		if len(toInject) == 0 {
			continue
		}
		page.Spec.Head.JSONLD = append(toInject, page.Spec.Head.JSONLD...)
		site.Pages[pageName] = page
	}

	return nil
}

func jsonLDTypes(blocks []model.JSONLDBlock) map[string]struct{} {
	types := make(map[string]struct{})
	for _, block := range blocks {
		for _, typ := range jsonLDTypesFromPayload(block.Payload) {
			types[typ] = struct{}{}
		}
	}
	return types
}

func jsonLDTypesFromPayload(payload map[string]any) []string {
	if len(payload) == 0 {
		return nil
	}
	raw, ok := payload["@type"]
	if !ok {
		return nil
	}
	values := make([]string, 0, 2)
	appendType := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return
		}
		for _, existing := range values {
			if existing == s {
				return
			}
		}
		values = append(values, s)
	}
	switch v := raw.(type) {
	case string:
		appendType(v)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				appendType(s)
			}
		}
	case []string:
		for _, item := range v {
			appendType(item)
		}
	}
	return values
}

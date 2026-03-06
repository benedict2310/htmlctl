package release

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/model"
)

type llmsPageEntry struct {
	URL         string
	Title       string
	Description string
}

func GenerateLLMsTxt(website model.Website, pages map[string]model.Page, log *buildLog) ([]byte, error) {
	seo := website.Spec.SEO
	if seo == nil || seo.LLMsTxt == nil || !seo.LLMsTxt.Enabled {
		return nil, nil
	}

	base, err := loader.NormalizePublicBaseURL(seo.PublicBaseURL)
	if err != nil {
		return nil, fmt.Errorf("normalize llms.txt publicBaseURL: %w", err)
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse normalized llms.txt publicBaseURL %q: %w", base, err)
	}

	title := strings.TrimSpace(seo.DisplayName)
	if title == "" {
		title = strings.TrimSpace(website.Metadata.Name)
	}
	if title == "" {
		title = "Website"
	}
	description := strings.TrimSpace(seo.Description)

	pageNames := make([]string, 0, len(pages))
	for name := range pages {
		pageNames = append(pageNames, name)
	}
	sort.Strings(pageNames)

	entries := make([]llmsPageEntry, 0, len(pageNames))
	for _, pageName := range pageNames {
		page := pages[pageName]
		if shouldExcludeFromSitemap(pageRobotsMeta(page.Spec.Head)) {
			continue
		}
		loc, include := effectiveSitemapURL(baseURL, pageName, page, log)
		if !include {
			continue
		}

		pageTitle := strings.TrimSpace(page.Spec.Title)
		if pageTitle == "" {
			pageTitle = strings.TrimSpace(page.Metadata.Name)
		}
		if pageTitle == "" {
			pageTitle = pageName
		}

		entries = append(entries, llmsPageEntry{
			URL:         loc,
			Title:       pageTitle,
			Description: strings.TrimSpace(page.Spec.Description),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].URL == entries[j].URL {
			return entries[i].Title < entries[j].Title
		}
		return entries[i].URL < entries[j].URL
	})

	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if description != "" {
		b.WriteString("> ")
		b.WriteString(description)
		b.WriteString("\n\n")
	}
	b.WriteString("## Pages\n\n")
	for _, entry := range entries {
		b.WriteString("- [")
		b.WriteString(entry.Title)
		b.WriteString("](")
		b.WriteString(entry.URL)
		b.WriteString(")")
		if entry.Description != "" {
			b.WriteString(": ")
			b.WriteString(entry.Description)
		}
		b.WriteString("\n")
	}

	return []byte(b.String()), nil
}

func (b *Builder) materializeLLMsTxtFile(ctx context.Context, site *model.Site, log *buildLog, releaseDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if site == nil {
		return fmt.Errorf("site is required")
	}
	content, err := GenerateLLMsTxt(site.Website, site.Pages, log)
	if err != nil {
		return err
	}
	if content == nil {
		return nil
	}
	if site.Website.Spec.SEO != nil && strings.TrimSpace(site.Website.Spec.SEO.Description) == "" && log != nil {
		log.Addf("info: llms.txt generated without website seo description")
	}
	if err := writeFile(filepath.Join(releaseDir, "llms.txt"), content); err != nil {
		return fmt.Errorf("write llms.txt: %w", err)
	}
	return nil
}

package release

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/model"
)

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

type sitemapEntry struct {
	Loc string
}

func GenerateSitemap(seo *model.WebsiteSEO, pages map[string]model.Page, log *buildLog) ([]byte, error) {
	if seo == nil || seo.Sitemap == nil || !seo.Sitemap.Enabled {
		return nil, nil
	}

	base, err := loader.NormalizePublicBaseURL(seo.PublicBaseURL)
	if err != nil {
		return nil, fmt.Errorf("normalize sitemap publicBaseURL: %w", err)
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse normalized publicBaseURL %q: %w", base, err)
	}

	pageNames := make([]string, 0, len(pages))
	for name := range pages {
		pageNames = append(pageNames, name)
	}
	sort.Strings(pageNames)

	entries := make([]sitemapEntry, 0, len(pageNames))
	for _, pageName := range pageNames {
		page := pages[pageName]
		if shouldExcludeFromSitemap(pageRobotsMeta(page.Spec.Head)) {
			continue
		}
		loc, include := effectiveSitemapURL(baseURL, pageName, page, log)
		if include {
			entries = append(entries, sitemapEntry{Loc: loc})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Loc < entries[j].Loc
	})

	urls := make([]sitemapURL, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, sitemapURL{Loc: entry.Loc})
	}

	payload, err := xml.MarshalIndent(sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sitemap xml: %w", err)
	}
	out := append([]byte(xml.Header), payload...)
	out = append(out, '\n')
	return out, nil
}

func (b *Builder) materializeSitemapFile(ctx context.Context, site *model.Site, log *buildLog, releaseDir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if site == nil {
		return "", fmt.Errorf("site is required")
	}
	content, err := GenerateSitemap(site.Website.Spec.SEO, site.Pages, log)
	if err != nil {
		return "", err
	}
	if content == nil {
		return "", nil
	}
	if err := writeFile(filepath.Join(releaseDir, "sitemap.xml"), content); err != nil {
		return "", fmt.Errorf("write sitemap.xml: %w", err)
	}
	return sitemapURLForWebsite(site.Website.Spec.SEO)
}

func shouldExcludeFromSitemap(robotsMeta string) bool {
	for _, raw := range strings.Split(robotsMeta, ",") {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token == "noindex" || token == "none" {
			return true
		}
	}
	return false
}

func pageRobotsMeta(head *model.PageHead) string {
	if head == nil || head.Meta == nil {
		return ""
	}
	for name, value := range head.Meta {
		if strings.EqualFold(strings.TrimSpace(name), "robots") {
			return value
		}
	}
	return ""
}

func effectiveSitemapURL(baseURL *url.URL, pageName string, page model.Page, log *buildLog) (string, bool) {
	canonical := ""
	if page.Spec.Head != nil {
		canonical = strings.TrimSpace(page.Spec.Head.CanonicalURL)
	}
	if canonical != "" {
		parsedCanonical, err := url.Parse(canonical)
		if err == nil && parsedCanonical.IsAbs() {
			if canonicalWithinPublicBaseScope(baseURL, parsedCanonical) {
				return parsedCanonical.String(), true
			}
			if log != nil {
				log.Addf("warning: page=%s skipped from sitemap because canonicalURL %q does not match publicBaseURL %q", pageName, canonical, baseURL.String())
			}
			return "", false
		}
		if err == nil && strings.HasPrefix(parsedCanonical.Path, "/") {
			resolvedCanonical := resolveRelativeCanonicalURL(baseURL, parsedCanonical)
			if canonicalWithinPublicBaseScope(baseURL, resolvedCanonical) {
				return resolvedCanonical.String(), true
			}
			if log != nil {
				log.Addf("warning: page=%s skipped from sitemap because canonicalURL %q does not match publicBaseURL %q", pageName, canonical, baseURL.String())
			}
			return "", false
		}
	}
	return deriveSitemapURL(baseURL, page.Spec.Route), true
}

func deriveSitemapURL(baseURL *url.URL, route string) string {
	derived := *baseURL
	basePath := strings.TrimRight(derived.Path, "/")
	if route == "/" {
		derived.Path = basePath + "/"
		if derived.Path == "" {
			derived.Path = "/"
		}
		return derived.String()
	}
	derived.Path = basePath + route
	if derived.Path == "" {
		derived.Path = "/"
	}
	return derived.String()
}

func sitemapURLForWebsite(seo *model.WebsiteSEO) (string, error) {
	if seo == nil || seo.Sitemap == nil || !seo.Sitemap.Enabled {
		return "", nil
	}
	base, err := loader.NormalizePublicBaseURL(seo.PublicBaseURL)
	if err != nil {
		return "", fmt.Errorf("normalize sitemap publicBaseURL: %w", err)
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse normalized publicBaseURL %q: %w", base, err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/sitemap.xml"
	if parsed.Path == "" {
		parsed.Path = "/sitemap.xml"
	}
	return parsed.String(), nil
}

func canonicalWithinPublicBaseScope(baseURL, canonicalURL *url.URL) bool {
	if !sameOrigin(baseURL, canonicalURL) {
		return false
	}
	basePath := normalizedURLPath(baseURL.Path)
	canonicalPath := normalizedURLPath(canonicalURL.Path)
	if basePath == "/" {
		return true
	}
	return canonicalPath == basePath || strings.HasPrefix(canonicalPath, basePath+"/")
}

func resolveRelativeCanonicalURL(baseURL, relative *url.URL) *url.URL {
	resolved := &url.URL{
		Scheme:   baseURL.Scheme,
		Host:     baseURL.Host,
		Path:     relative.Path,
		RawPath:  relative.RawPath,
		RawQuery: relative.RawQuery,
		Fragment: relative.Fragment,
	}
	return resolved
}

func normalizedURLPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	value = path.Clean(value)
	if value == "." || value == "" {
		return "/"
	}
	return value
}

func sameOrigin(baseURL, candidateURL *url.URL) bool {
	if !strings.EqualFold(baseURL.Scheme, candidateURL.Scheme) {
		return false
	}
	if !strings.EqualFold(baseURL.Hostname(), candidateURL.Hostname()) {
		return false
	}
	return normalizedURLPort(baseURL) == normalizedURLPort(candidateURL)
}

func normalizedURLPort(u *url.URL) string {
	if port := strings.TrimSpace(u.Port()); port != "" {
		return port
	}
	switch strings.ToLower(strings.TrimSpace(u.Scheme)) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

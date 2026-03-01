package loader

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/benedict2310/htmlctl/pkg/model"
)

const (
	maxHeadURLLength          = 2048
	maxHeadMetaEntries        = 32
	maxHeadMetaKeyLength      = 128
	maxHeadMetaValueLength    = 512
	maxHeadTextFieldLength    = 512
	maxHeadJSONLDBlocks       = 5
	maxHeadJSONLDIDLength     = 128
	maxHeadJSONLDPayloadBytes = 16 * 1024
	maxRobotsGroups           = 50
	maxRobotsUserAgents       = 10
	maxRobotsUserAgentLen     = 256
	maxRobotsAllowRules       = 50
	maxRobotsDisallowRules    = 50
	maxRobotsRuleLen          = 512
	maxPublicBaseURLLen       = 2048
)

// ValidateSite validates cross-resource relationships required for safe parsing.
func ValidateSite(site *model.Site) error {
	if site == nil {
		return fmt.Errorf("site is nil")
	}
	if site.Website.Metadata.Name == "" {
		return fmt.Errorf("website metadata.name is required")
	}
	if err := validateWebsiteHead(site); err != nil {
		return err
	}
	if err := NormalizeWebsiteSEO(site.Website.Spec.SEO); err != nil {
		return err
	}
	if len(site.Pages) == 0 {
		return fmt.Errorf("at least one page is required")
	}

	routes := make(map[string]string, len(site.Pages))
	for pageName, page := range site.Pages {
		route := NormalizeRoute(page.Spec.Route)
		if route == "" {
			return fmt.Errorf("page %q has empty route", pageName)
		}

		if existingPage, exists := routes[route]; exists {
			return fmt.Errorf("duplicate route %q in pages %q and %q", route, existingPage, pageName)
		}
		routes[route] = pageName

		page.Spec.Route = route
		site.Pages[pageName] = page

		if err := validatePageHead(pageName, page.Spec.Head); err != nil {
			return err
		}

		for _, item := range page.Spec.Layout {
			include := strings.TrimSpace(item.Include)
			if include == "" {
				return fmt.Errorf("page %q has an empty include", pageName)
			}
			if _, exists := site.Components[include]; !exists {
				return fmt.Errorf("page %q references missing component %q", pageName, include)
			}
		}
	}

	return nil
}

// NormalizeWebsiteSEO validates website SEO metadata and stores canonical forms
// back into the provided struct.
func NormalizeWebsiteSEO(seo *model.WebsiteSEO) error {
	if seo == nil {
		return nil
	}
	if value := strings.TrimSpace(seo.PublicBaseURL); value != "" {
		normalized, err := NormalizePublicBaseURL(value)
		if err != nil {
			return fmt.Errorf("website seo publicBaseURL: %w", err)
		}
		seo.PublicBaseURL = normalized
	}
	if seo.Robots == nil {
		if seo.Sitemap != nil && seo.Sitemap.Enabled && strings.TrimSpace(seo.PublicBaseURL) == "" {
			return fmt.Errorf("website seo sitemap.enabled requires publicBaseURL")
		}
		return nil
	}
	if len(seo.Robots.Groups) > maxRobotsGroups {
		return fmt.Errorf("website seo robots has too many groups: %d > %d", len(seo.Robots.Groups), maxRobotsGroups)
	}
	for i := range seo.Robots.Groups {
		group := &seo.Robots.Groups[i]
		field := fmt.Sprintf("website seo robots group %d", i)
		if len(group.UserAgents) == 0 {
			return fmt.Errorf("%s must include at least one userAgent", field)
		}
		if len(group.UserAgents) > maxRobotsUserAgents {
			return fmt.Errorf("%s has too many userAgents: %d > %d", field, len(group.UserAgents), maxRobotsUserAgents)
		}
		if len(group.Allow) > maxRobotsAllowRules {
			return fmt.Errorf("%s has too many allow rules: %d > %d", field, len(group.Allow), maxRobotsAllowRules)
		}
		if len(group.Disallow) > maxRobotsDisallowRules {
			return fmt.Errorf("%s has too many disallow rules: %d > %d", field, len(group.Disallow), maxRobotsDisallowRules)
		}
		for j := range group.UserAgents {
			value := strings.TrimSpace(group.UserAgents[j])
			if value == "" {
				return fmt.Errorf("%s has empty userAgent at index %d", field, j)
			}
			if err := validateRobotsLine(field, "userAgent", value, maxRobotsUserAgentLen); err != nil {
				return err
			}
			group.UserAgents[j] = value
		}
		for j := range group.Allow {
			value, err := normalizeRobotsRule(field, "allow", group.Allow[j])
			if err != nil {
				return err
			}
			group.Allow[j] = value
		}
		for j := range group.Disallow {
			value, err := normalizeRobotsRule(field, "disallow", group.Disallow[j])
			if err != nil {
				return err
			}
			group.Disallow[j] = value
		}
	}
	if seo.Sitemap != nil && seo.Sitemap.Enabled && strings.TrimSpace(seo.PublicBaseURL) == "" {
		return fmt.Errorf("website seo sitemap.enabled requires publicBaseURL")
	}
	return nil
}

// NormalizePublicBaseURL validates and canonicalizes an operator-declared
// canonical site URL used by release-generated crawl artifacts.
func NormalizePublicBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) > maxPublicBaseURLLen {
		return "", fmt.Errorf("must be at most %d characters", maxPublicBaseURLLen)
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("is invalid: %w", err)
	}
	if !u.IsAbs() {
		return "", fmt.Errorf("must be absolute")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", fmt.Errorf("has unsupported scheme %q", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("must include a host")
	}
	if u.RawQuery != "" {
		return "", fmt.Errorf("must not include a query string")
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("must not include a fragment")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
		if u.Path == "" {
			u.Path = "/"
		}
	}
	return u.String(), nil
}

func validateWebsiteHead(site *model.Site) error {
	head := site.Website.Spec.Head
	if head == nil || head.Icons == nil {
		return nil
	}
	for slot, pathValue := range websiteIconPaths(head.Icons) {
		rel, err := normalizeBrandingPath(pathValue)
		if err != nil {
			return fmt.Errorf("website head icon %q: %w", slot, err)
		}
		asset, ok := site.Branding[slot]
		if !ok || asset.SourcePath != rel {
			return fmt.Errorf("website head icon %q references missing branding file %q", slot, rel)
		}
		switch slot {
		case "svg":
			if strings.ToLower(filepath.Ext(rel)) != ".svg" {
				return fmt.Errorf("website head icon %q must reference an .svg file", slot)
			}
		case "ico":
			if strings.ToLower(filepath.Ext(rel)) != ".ico" {
				return fmt.Errorf("website head icon %q must reference an .ico file", slot)
			}
		case "apple_touch":
			if strings.ToLower(filepath.Ext(rel)) != ".png" {
				return fmt.Errorf("website head icon %q must reference a .png file", slot)
			}
		default:
			return fmt.Errorf("unsupported website head icon slot %q", slot)
		}
	}
	return nil
}

// NormalizeRoute normalizes routes to a deterministic representation.
func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return ""
	}

	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	if len(route) > 1 {
		route = strings.TrimRight(route, "/")
	}

	return route
}

func validatePageHead(pageName string, head *model.PageHead) error {
	if head == nil {
		return nil
	}
	if err := validateHeadURLField(pageName, "canonicalURL", head.CanonicalURL, maxHeadURLLength); err != nil {
		return err
	}
	if len(head.Meta) > maxHeadMetaEntries {
		return fmt.Errorf("page %q has too many head.meta entries: %d > %d", pageName, len(head.Meta), maxHeadMetaEntries)
	}
	for name := range head.Meta {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("page %q has head.meta entry with empty key", pageName)
		}
		if err := validateMaxLen(pageName, "head.meta key", name, maxHeadMetaKeyLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "head.meta value", head.Meta[name], maxHeadMetaValueLength); err != nil {
			return err
		}
	}
	if head.OpenGraph != nil {
		if err := validateMaxLen(pageName, "openGraph.type", head.OpenGraph.Type, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateHeadURLField(pageName, "openGraph.url", head.OpenGraph.URL, maxHeadURLLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "openGraph.siteName", head.OpenGraph.SiteName, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "openGraph.locale", head.OpenGraph.Locale, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "openGraph.title", head.OpenGraph.Title, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "openGraph.description", head.OpenGraph.Description, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateHeadURLField(pageName, "openGraph.image", head.OpenGraph.Image, maxHeadURLLength); err != nil {
			return err
		}
	}
	if head.Twitter != nil {
		if err := validateMaxLen(pageName, "twitter.card", head.Twitter.Card, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateHeadURLField(pageName, "twitter.url", head.Twitter.URL, maxHeadURLLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "twitter.title", head.Twitter.Title, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateMaxLen(pageName, "twitter.description", head.Twitter.Description, maxHeadTextFieldLength); err != nil {
			return err
		}
		if err := validateHeadURLField(pageName, "twitter.image", head.Twitter.Image, maxHeadURLLength); err != nil {
			return err
		}
	}
	if len(head.JSONLD) > maxHeadJSONLDBlocks {
		return fmt.Errorf("page %q has too many jsonLD blocks: %d > %d", pageName, len(head.JSONLD), maxHeadJSONLDBlocks)
	}
	for i, block := range head.JSONLD {
		field := fmt.Sprintf("jsonLD[%d]", i)
		if err := validateMaxLen(pageName, field+".id", block.ID, maxHeadJSONLDIDLength); err != nil {
			return err
		}
		payloadBytes, err := json.Marshal(block.Payload)
		if err != nil {
			return fmt.Errorf("page %q has invalid %s payload: %w", pageName, field, err)
		}
		if len(payloadBytes) > maxHeadJSONLDPayloadBytes {
			return fmt.Errorf("page %q has %s payload larger than %d bytes", pageName, field, maxHeadJSONLDPayloadBytes)
		}
	}
	return nil
}

func validateHeadURLField(pageName, field, raw string, maxLen int) error {
	if err := validateMaxLen(pageName, field, raw, maxLen); err != nil {
		return err
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("page %q has invalid %s value %q: %w", pageName, field, raw, err)
	}
	if u.Scheme == "" {
		return nil
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("page %q has unsupported %s scheme %q", pageName, field, u.Scheme)
	}
}

func validateMaxLen(pageName, field, value string, maxLen int) error {
	if utf8.RuneCountInString(value) > maxLen {
		return fmt.Errorf("page %q has %s longer than %d characters", pageName, field, maxLen)
	}
	return nil
}

func normalizeRobotsRule(groupField, kind, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("%s has empty %s rule", groupField, kind)
	}
	if err := validateRobotsLine(groupField, kind, value, maxRobotsRuleLen); err != nil {
		return "", err
	}
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("%s %s rule %q must start with /", groupField, kind, raw)
	}
	return value, nil
}

func validateRobotsLine(groupField, kind, value string, maxLen int) error {
	if utf8.RuneCountInString(value) > maxLen {
		return fmt.Errorf("%s has %s longer than %d characters", groupField, kind, maxLen)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s %s %q must not contain newlines", groupField, kind, value)
	}
	return nil
}

package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
)

var (
	canonicalTagTemplate = template.Must(template.New("canonical-tag").Parse(`  <link rel="canonical" href="{{.URL}}">` + "\n"))
	nameMetaTagTemplate  = template.Must(template.New("name-meta-tag").Parse(`  <meta name="{{.Name}}" content="{{.Content}}">` + "\n"))
	propMetaTagTemplate  = template.Must(template.New("prop-meta-tag").Parse(`  <meta property="{{.Property}}" content="{{.Content}}">` + "\n"))
	linkTagTemplate      = template.Must(template.New("link-tag").Parse(`  <link rel="{{.Rel}}"{{if .Type}} type="{{.Type}}"{{end}} href="{{.Href}}">` + "\n"))
)

type canonicalTagData struct {
	URL string
}

type nameMetaTagData struct {
	Name    string
	Content string
}

type propertyMetaTagData struct {
	Property string
	Content  string
}

type linkTagData struct {
	Rel  string
	Type string
	Href string
}

func renderHeadMeta(head *model.PageHead) (template.HTML, error) {
	if head == nil {
		return "", nil
	}

	var buf bytes.Buffer

	if canonical := strings.TrimSpace(head.CanonicalURL); canonical != "" {
		if err := canonicalTagTemplate.Execute(&buf, canonicalTagData{URL: canonical}); err != nil {
			return "", fmt.Errorf("render canonical tag: %w", err)
		}
	}

	metaNames := make([]string, 0, len(head.Meta))
	for name := range head.Meta {
		if strings.TrimSpace(name) == "" {
			continue
		}
		metaNames = append(metaNames, name)
	}
	sort.Strings(metaNames)
	for _, name := range metaNames {
		if err := nameMetaTagTemplate.Execute(&buf, nameMetaTagData{
			Name:    name,
			Content: head.Meta[name],
		}); err != nil {
			return "", fmt.Errorf("render meta[name] tag %q: %w", name, err)
		}
	}

	if head.OpenGraph != nil {
		openGraphProperties := []propertyMetaTagData{
			{Property: "og:type", Content: head.OpenGraph.Type},
			{Property: "og:url", Content: head.OpenGraph.URL},
			{Property: "og:site_name", Content: head.OpenGraph.SiteName},
			{Property: "og:locale", Content: head.OpenGraph.Locale},
			{Property: "og:title", Content: head.OpenGraph.Title},
			{Property: "og:description", Content: head.OpenGraph.Description},
			{Property: "og:image", Content: head.OpenGraph.Image},
		}
		for _, property := range openGraphProperties {
			if strings.TrimSpace(property.Content) == "" {
				continue
			}
			if err := propMetaTagTemplate.Execute(&buf, property); err != nil {
				return "", fmt.Errorf("render open graph tag %q: %w", property.Property, err)
			}
		}
	}

	if head.Twitter != nil {
		twitterProperties := []propertyMetaTagData{
			{Property: "twitter:card", Content: head.Twitter.Card},
			{Property: "twitter:url", Content: head.Twitter.URL},
			{Property: "twitter:title", Content: head.Twitter.Title},
			{Property: "twitter:description", Content: head.Twitter.Description},
			{Property: "twitter:image", Content: head.Twitter.Image},
		}
		for _, property := range twitterProperties {
			if strings.TrimSpace(property.Content) == "" {
				continue
			}
			if err := propMetaTagTemplate.Execute(&buf, property); err != nil {
				return "", fmt.Errorf("render twitter tag %q: %w", property.Property, err)
			}
		}
	}

	for _, block := range head.JSONLD {
		if len(block.Payload) == 0 {
			continue
		}
		payload, err := json.Marshal(block.Payload)
		if err != nil {
			if strings.TrimSpace(block.ID) != "" {
				return "", fmt.Errorf("marshal JSON-LD block %q: %w", block.ID, err)
			}
			return "", fmt.Errorf("marshal JSON-LD block: %w", err)
		}
		// Payload is safe to embed because json.Marshal escapes <, >, and &.
		if _, err := buf.WriteString(`  <script type="application/ld+json">`); err != nil {
			return "", fmt.Errorf("render JSON-LD opening tag: %w", err)
		}
		if _, err := buf.Write(payload); err != nil {
			if strings.TrimSpace(block.ID) != "" {
				return "", fmt.Errorf("render JSON-LD block %q payload: %w", block.ID, err)
			}
			return "", fmt.Errorf("render JSON-LD payload: %w", err)
		}
		if _, err := buf.WriteString(`</script>` + "\n"); err != nil {
			return "", fmt.Errorf("render JSON-LD closing tag: %w", err)
		}
	}

	return template.HTML(buf.String()), nil
}

func renderWebsiteIcons(head *model.WebsiteHead, branding map[string]model.BrandingAsset) (template.HTML, error) {
	if head == nil || head.Icons == nil || len(branding) == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	type iconSpec struct {
		slot string
		rel  string
		typ  string
		href string
	}
	icons := []iconSpec{
		{slot: "svg", rel: "icon", typ: "image/svg+xml", href: "/favicon.svg"},
		{slot: "ico", rel: "icon", href: "/favicon.ico"},
		{slot: "apple_touch", rel: "apple-touch-icon", href: "/apple-touch-icon.png"},
	}
	for _, icon := range icons {
		if _, ok := branding[icon.slot]; !ok {
			continue
		}
		if err := linkTagTemplate.Execute(&buf, linkTagData{Rel: icon.rel, Type: icon.typ, Href: icon.href}); err != nil {
			return "", fmt.Errorf("render website icon tag for %q: %w", icon.slot, err)
		}
	}
	return template.HTML(buf.String()), nil
}

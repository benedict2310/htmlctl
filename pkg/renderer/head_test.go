package renderer

import (
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestRenderHeadMetaOrderAndFields(t *testing.T) {
	out, err := renderHeadMeta(&model.PageHead{
		CanonicalURL: "https://example.com/ora",
		Meta: map[string]string{
			"robots":   "index,follow",
			"author":   "Benedict",
			"keywords": "Ora, macOS voice assistant",
		},
		OpenGraph: &model.OpenGraph{
			Type:        "website",
			URL:         "https://example.com/ora",
			SiteName:    "Sample Studio",
			Locale:      "en_US",
			Title:       "Ora for macOS",
			Description: "Local-first voice assistant",
			Image:       "/assets/ora/og-image.jpg",
		},
		Twitter: &model.TwitterCard{
			Card:        "summary_large_image",
			URL:         "https://example.com/ora",
			Title:       "Ora for macOS",
			Description: "Local-first voice assistant",
			Image:       "/assets/ora/og-image.jpg",
		},
		JSONLD: []model.JSONLDBlock{
			{
				ID: "a",
				Payload: map[string]any{
					"@context": "https://schema.org",
					"@type":    "WebSite",
					"name":     "Sample Studio",
				},
			},
			{
				ID: "b",
				Payload: map[string]any{
					"@context": "https://schema.org",
					"@type":    "SoftwareApplication",
					"name":     "Ora",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("renderHeadMeta() error = %v", err)
	}

	html := string(out)
	needles := []string{
		`<link rel="canonical" href="https://example.com/ora">`,
		`<meta name="author" content="Benedict">`,
		`<meta name="keywords" content="Ora, macOS voice assistant">`,
		`<meta name="robots" content="index,follow">`,
		`<meta property="og:type" content="website">`,
		`<meta property="og:url" content="https://example.com/ora">`,
		`<meta property="og:site_name" content="Sample Studio">`,
		`<meta property="og:locale" content="en_US">`,
		`<meta property="og:title" content="Ora for macOS">`,
		`<meta property="og:description" content="Local-first voice assistant">`,
		`<meta property="og:image" content="/assets/ora/og-image.jpg">`,
		`<meta property="twitter:card" content="summary_large_image">`,
		`<meta property="twitter:url" content="https://example.com/ora">`,
		`<meta property="twitter:title" content="Ora for macOS">`,
		`<meta property="twitter:description" content="Local-first voice assistant">`,
		`<meta property="twitter:image" content="/assets/ora/og-image.jpg">`,
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"Sample Studio"}</script>`,
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"SoftwareApplication","name":"Ora"}</script>`,
	}

	last := -1
	for _, needle := range needles {
		idx := strings.Index(html, needle)
		if idx == -1 {
			t.Fatalf("expected head metadata to contain %q", needle)
		}
		if idx <= last {
			t.Fatalf("expected deterministic ordering for %q", needle)
		}
		last = idx
	}
}

func TestRenderHeadMetaEscapesAndSafeJSONLD(t *testing.T) {
	out, err := renderHeadMeta(&model.PageHead{
		Meta: map[string]string{
			`x" onmouseover="evil"`: `"><script>alert(1)</script>`,
		},
		JSONLD: []model.JSONLDBlock{
			{
				ID: "safe",
				Payload: map[string]any{
					"name": `foo</script><script>alert(1)</script>`,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("renderHeadMeta() error = %v", err)
	}

	html := string(out)
	if strings.Contains(html, `<script>alert(1)</script>`) {
		t.Fatalf("expected untrusted payload to remain escaped, got: %s", html)
	}
	if !strings.Contains(html, `name="x&#34; onmouseover=&#34;evil&#34;"`) {
		t.Fatalf("expected escaped meta key in output, got: %s", html)
	}
	if strings.Contains(html, `</script><script>`) {
		t.Fatalf("expected JSON-LD payload to avoid script-breakout, got: %s", html)
	}
	if !strings.Contains(html, `foo\u003c/script\u003e\u003cscript\u003ealert(1)\u003c/script\u003e`) {
		t.Fatalf("expected JSON-LD payload to be HTML-safe escaped, got: %s", html)
	}
}

func TestRenderHeadMetaJSONLDMarshalError(t *testing.T) {
	_, err := renderHeadMeta(&model.PageHead{
		JSONLD: []model.JSONLDBlock{
			{
				ID: "broken",
				Payload: map[string]any{
					"fn": func() {},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected JSON-LD marshal error")
	}
	if !strings.Contains(err.Error(), `marshal JSON-LD block "broken"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

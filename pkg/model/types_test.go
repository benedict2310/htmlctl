package model

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWebsiteYAMLDeserialization(t *testing.T) {
	input := []byte(`apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
      ico: branding/favicon.ico
      appleTouch: branding/apple-touch-icon.png
  seo:
    publicBaseURL: https://example.com/
    robots:
      enabled: true
      groups:
        - userAgents:
            - "*"
          allow:
            - /
          disallow:
            - /drafts/
`)

	var website Website
	if err := yaml.Unmarshal(input, &website); err != nil {
		t.Fatalf("unmarshal website: %v", err)
	}

	if website.Metadata.Name != "sample" {
		t.Fatalf("unexpected website name: %q", website.Metadata.Name)
	}
	if website.Spec.DefaultStyleBundle != "default" {
		t.Fatalf("unexpected style bundle: %q", website.Spec.DefaultStyleBundle)
	}
	if website.Spec.BaseTemplate != "default" {
		t.Fatalf("unexpected base template: %q", website.Spec.BaseTemplate)
	}
	if website.Spec.Head == nil || website.Spec.Head.Icons == nil {
		t.Fatalf("expected website head icons to be parsed")
	}
	if website.Spec.Head.Icons.SVG != "branding/favicon.svg" {
		t.Fatalf("unexpected svg icon path: %q", website.Spec.Head.Icons.SVG)
	}
	if website.Spec.Head.Icons.ICO != "branding/favicon.ico" {
		t.Fatalf("unexpected ico icon path: %q", website.Spec.Head.Icons.ICO)
	}
	if website.Spec.Head.Icons.AppleTouch != "branding/apple-touch-icon.png" {
		t.Fatalf("unexpected apple touch icon path: %q", website.Spec.Head.Icons.AppleTouch)
	}
	if website.Spec.SEO == nil || website.Spec.SEO.Robots == nil {
		t.Fatalf("expected website seo robots to be parsed")
	}
	if website.Spec.SEO.PublicBaseURL != "https://example.com/" {
		t.Fatalf("unexpected publicBaseURL: %q", website.Spec.SEO.PublicBaseURL)
	}
	if !website.Spec.SEO.Robots.Enabled {
		t.Fatalf("expected robots enabled to be parsed")
	}
	if len(website.Spec.SEO.Robots.Groups) != 1 {
		t.Fatalf("expected 1 robots group, got %d", len(website.Spec.SEO.Robots.Groups))
	}
	if got := website.Spec.SEO.Robots.Groups[0].Disallow; len(got) != 1 || got[0] != "/drafts/" {
		t.Fatalf("unexpected robots disallow rules: %#v", got)
	}
}

func TestWebsiteHeadJSONMarshaling(t *testing.T) {
	head := &WebsiteHead{
		Icons: &WebsiteIcons{
			SVG:        "branding/favicon.svg",
			ICO:        "branding/favicon.ico",
			AppleTouch: "branding/apple-touch-icon.png",
		},
	}

	b, err := json.Marshal(head)
	if err != nil {
		t.Fatalf("marshal website head: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"svg":"branding/favicon.svg"`,
		`"ico":"branding/favicon.ico"`,
		`"appleTouch":"branding/apple-touch-icon.png"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in marshaled output, got: %s", want, got)
		}
	}
}

func TestWebsiteSEOJSONMarshaling(t *testing.T) {
	seo := &WebsiteSEO{
		PublicBaseURL: "https://example.com",
		Robots: &WebsiteRobots{
			Enabled: true,
			Groups: []RobotsGroup{
				{
					UserAgents: []string{"*", "Googlebot"},
					Allow:      []string{"/"},
					Disallow:   []string{"/preview/"},
				},
			},
		},
	}

	b, err := json.Marshal(seo)
	if err != nil {
		t.Fatalf("marshal website seo: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"publicBaseURL":"https://example.com"`,
		`"enabled":true`,
		`"userAgents":["*","Googlebot"]`,
		`"allow":["/"]`,
		`"disallow":["/preview/"]`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in marshaled output, got: %s", want, got)
		}
	}
}

func TestPageYAMLDeserialization(t *testing.T) {
	input := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: product
spec:
  route: /product
  title: Product
  description: Product page
  layout:
    - include: header
    - include: pricing
  head:
    canonicalURL: https://example.com/product
    meta:
      robots: index,follow
      keywords: Product, Sample
    openGraph:
      type: website
      url: https://example.com/product
      siteName: Sample
      locale: en_US
      title: Product
      description: Product page
      image: /assets/product/og.jpg
    twitter:
      card: summary_large_image
      url: https://example.com/product
      title: Product
      description: Product page
      image: /assets/product/og.jpg
    jsonLD:
      - id: product
        payload:
          "@context": https://schema.org
          "@type": Product
          name: Product
`)

	var page Page
	if err := yaml.Unmarshal(input, &page); err != nil {
		t.Fatalf("unmarshal page: %v", err)
	}

	if page.Metadata.Name != "product" {
		t.Fatalf("unexpected page name: %q", page.Metadata.Name)
	}
	if len(page.Spec.Layout) != 2 {
		t.Fatalf("expected 2 layout includes, got %d", len(page.Spec.Layout))
	}
	if page.Spec.Layout[0].Include != "header" || page.Spec.Layout[1].Include != "pricing" {
		t.Fatalf("layout includes are not preserved in order: %#v", page.Spec.Layout)
	}
	if page.Spec.Head == nil {
		t.Fatalf("expected page head to be parsed")
	}
	if page.Spec.Head.CanonicalURL != "https://example.com/product" {
		t.Fatalf("unexpected canonicalURL: %q", page.Spec.Head.CanonicalURL)
	}
	if page.Spec.Head.OpenGraph == nil || page.Spec.Head.OpenGraph.SiteName != "Sample" {
		t.Fatalf("unexpected openGraph data: %#v", page.Spec.Head.OpenGraph)
	}
	if page.Spec.Head.Twitter == nil || page.Spec.Head.Twitter.Card != "summary_large_image" {
		t.Fatalf("unexpected twitter data: %#v", page.Spec.Head.Twitter)
	}
	if len(page.Spec.Head.JSONLD) != 1 || page.Spec.Head.JSONLD[0].ID != "product" {
		t.Fatalf("unexpected jsonLD blocks: %#v", page.Spec.Head.JSONLD)
	}
}

func TestPageLayoutItemJSONMarshaling(t *testing.T) {
	b, err := json.Marshal([]PageLayoutItem{{Include: "header"}})
	if err != nil {
		t.Fatalf("marshal layout: %v", err)
	}
	if string(b) != `[{"include":"header"}]` {
		t.Fatalf("unexpected json output: %s", string(b))
	}
}

func TestPageHeadJSONMarshaling(t *testing.T) {
	head := &PageHead{
		CanonicalURL: "https://example.com/ora",
		Meta: map[string]string{
			"application-name": "Ora",
			"author":           "Benedict",
		},
		OpenGraph: &OpenGraph{
			Type:  "website",
			Title: "Ora",
		},
		Twitter: &TwitterCard{
			Card:  "summary_large_image",
			Title: "Ora",
		},
		JSONLD: []JSONLDBlock{
			{
				ID: "ora-softwareapplication",
				Payload: map[string]any{
					"@context": "https://schema.org",
					"@type":    "SoftwareApplication",
					"name":     "Ora",
				},
			},
		},
	}

	b, err := json.Marshal(head)
	if err != nil {
		t.Fatalf("marshal page head: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"canonicalURL":"https://example.com/ora"`,
		`"meta":{"application-name":"Ora","author":"Benedict"}`,
		`"openGraph":{"type":"website","title":"Ora"}`,
		`"twitter":{"card":"summary_large_image","title":"Ora"}`,
		`"jsonLD":[{"id":"ora-softwareapplication","payload":{"@context":"https://schema.org","@type":"SoftwareApplication","name":"Ora"}}]`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in marshaled output, got: %s", want, got)
		}
	}
}

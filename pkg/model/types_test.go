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
  name: futurelab
spec:
  defaultStyleBundle: default
  baseTemplate: default
`)

	var website Website
	if err := yaml.Unmarshal(input, &website); err != nil {
		t.Fatalf("unmarshal website: %v", err)
	}

	if website.Metadata.Name != "futurelab" {
		t.Fatalf("unexpected website name: %q", website.Metadata.Name)
	}
	if website.Spec.DefaultStyleBundle != "default" {
		t.Fatalf("unexpected style bundle: %q", website.Spec.DefaultStyleBundle)
	}
	if website.Spec.BaseTemplate != "default" {
		t.Fatalf("unexpected base template: %q", website.Spec.BaseTemplate)
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
    canonicalURL: https://futurelab.studio/product
    meta:
      robots: index,follow
      keywords: Product, Futurelab
    openGraph:
      type: website
      url: https://futurelab.studio/product
      siteName: Futurelab
      locale: en_US
      title: Product
      description: Product page
      image: /assets/product/og.jpg
    twitter:
      card: summary_large_image
      url: https://futurelab.studio/product
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
	if page.Spec.Head.CanonicalURL != "https://futurelab.studio/product" {
		t.Fatalf("unexpected canonicalURL: %q", page.Spec.Head.CanonicalURL)
	}
	if page.Spec.Head.OpenGraph == nil || page.Spec.Head.OpenGraph.SiteName != "Futurelab" {
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
		CanonicalURL: "https://futurelab.studio/ora",
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
		`"canonicalURL":"https://futurelab.studio/ora"`,
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

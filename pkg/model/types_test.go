package model

import (
	"encoding/json"
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

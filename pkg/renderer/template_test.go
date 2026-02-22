package renderer

import (
	"html/template"
	"strings"
	"testing"
)

func TestRenderDefaultTemplateStructureAndOrder(t *testing.T) {
	output, err := renderDefaultTemplate(pageTemplateData{
		Title:       "Product",
		Description: "Product page",
		StyleHrefs:  []string{"/styles/tokens-abc.css", "/styles/default-def.css"},
		ContentHTML: template.HTML("<section id=\"hero\"></section>\n"),
		ScriptSrc:   "/scripts/site-123.js",
	})
	if err != nil {
		t.Fatalf("renderDefaultTemplate() error = %v", err)
	}

	html := string(output)
	for _, needle := range []string{"<!DOCTYPE html>", "<html", "<head>", "<body>", "<main>", "<title>Product</title>", "meta name=\"description\" content=\"Product page\"", "<script src=\"/scripts/site-123.js\"></script>"} {
		if !strings.Contains(html, needle) {
			t.Fatalf("expected rendered html to contain %q", needle)
		}
	}

	tokensIdx := strings.Index(html, "/styles/tokens-abc.css")
	defaultIdx := strings.Index(html, "/styles/default-def.css")
	if tokensIdx == -1 || defaultIdx == -1 || tokensIdx > defaultIdx {
		t.Fatalf("expected styles in stable order, got html: %s", html)
	}
}

func TestRenderDefaultTemplateNoScriptWhenEmpty(t *testing.T) {
	output, err := renderDefaultTemplate(pageTemplateData{Title: "A", Description: "B"})
	if err != nil {
		t.Fatalf("renderDefaultTemplate() error = %v", err)
	}
	if strings.Contains(string(output), "<script") {
		t.Fatalf("unexpected script tag when ScriptSrc is empty")
	}
}

func TestRenderDefaultTemplateEscapesMetadataFields(t *testing.T) {
	output, err := renderDefaultTemplate(pageTemplateData{
		Title:       `</title><script>alert(1)</script>`,
		Description: `" onload="evil()"`,
	})
	if err != nil {
		t.Fatalf("renderDefaultTemplate() error = %v", err)
	}

	html := string(output)
	if strings.Contains(html, `<script>alert(1)</script>`) {
		t.Fatalf("expected title script payload to be escaped, got: %s", html)
	}
	if !strings.Contains(html, `<title>&lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;</title>`) {
		t.Fatalf("expected escaped title payload, got: %s", html)
	}
	if strings.Contains(html, `content="" onload="evil()"`) {
		t.Fatalf("expected description payload to remain escaped in content attr, got: %s", html)
	}
	if !strings.Contains(html, `meta name="description" content="&#34; onload=&#34;evil()&#34;"`) {
		t.Fatalf("expected escaped description payload, got: %s", html)
	}
}

func TestRenderDefaultTemplatePreservesTrustedContentHTML(t *testing.T) {
	output, err := renderDefaultTemplate(pageTemplateData{
		Title:       "A",
		Description: "B",
		ContentHTML: template.HTML("<section id=\"pricing\"><h2>Pricing</h2></section>\n"),
	})
	if err != nil {
		t.Fatalf("renderDefaultTemplate() error = %v", err)
	}

	html := string(output)
	if !strings.Contains(html, "<section id=\"pricing\"><h2>Pricing</h2></section>") {
		t.Fatalf("expected component html to render unescaped, got: %s", html)
	}
	if strings.Contains(html, "&lt;section") {
		t.Fatalf("unexpected escaping of trusted component html, got: %s", html)
	}
}

package renderer

import (
	"strings"
	"testing"
)

func TestRenderDefaultTemplateStructureAndOrder(t *testing.T) {
	output, err := renderDefaultTemplate(pageTemplateData{
		Title:       "Product",
		Description: "Product page",
		StyleHrefs:  []string{"/styles/tokens-abc.css", "/styles/default-def.css"},
		ContentHTML: "<section id=\"hero\"></section>\n",
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

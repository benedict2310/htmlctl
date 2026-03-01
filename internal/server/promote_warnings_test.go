package server

import (
	"strings"
	"testing"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestPromoteMetadataHostWarningsIgnoresRelativeURLs(t *testing.T) {
	warnings, err := promoteMetadataHostWarnings(`{
  "resources": {
    "pages": [
      {
        "name": "index",
        "head": {
          "canonicalURL": "/",
          "openGraph": {
            "image": "/og/index.png"
          },
          "twitter": {
            "url": "/index"
          }
        }
      }
    ]
  }
}`, nil, []dbpkg.DomainBindingResolvedRow{{Domain: "example.com"}}, "prod")
	if err != nil {
		t.Fatalf("promoteMetadataHostWarnings() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for relative URLs, got %#v", warnings)
	}
}

func TestPromoteMetadataHostWarningsMatchesSourceHostAndStagingFallback(t *testing.T) {
	sourceBindings := []dbpkg.DomainBindingResolvedRow{{Domain: "staging.example.com"}}
	targetBindings := []dbpkg.DomainBindingResolvedRow{{Domain: "example.com"}}
	warnings, err := promoteMetadataHostWarnings(`{
  "resources": {
    "pages": [
      {
        "name": "index",
        "head": {
          "canonicalURL": "https://staging.example.com/",
          "openGraph": {
            "image": "https://cdn-staging.example.net/og.png"
          }
        }
      }
    ]
  }
}`, sourceBindings, targetBindings, "prod")
	if err != nil {
		t.Fatalf("promoteMetadataHostWarnings() error = %v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %#v", warnings)
	}
	if !strings.Contains(warnings[0], "page=index field=canonicalURL host=staging.example.com") {
		t.Fatalf("unexpected canonical warning: %q", warnings[0])
	}
	if !strings.Contains(warnings[1], "page=index field=openGraph.image host=cdn-staging.example.net") {
		t.Fatalf("unexpected open graph warning: %q", warnings[1])
	}
}

func TestPromoteMetadataHostWarningsStableOrderAndCap(t *testing.T) {
	pages := make([]string, 0, maxPromoteMetadataWarnings+5)
	for i := 0; i < maxPromoteMetadataWarnings+5; i++ {
		pages = append(pages, `{"name":"page-`+string(rune('a'+i))+`","head":{"canonicalURL":"https://staging.example.com/`+string(rune('a'+i))+`"}}`)
	}
	manifest := `{"resources":{"pages":[` + strings.Join(pages, ",") + `]}}`
	warnings, err := promoteMetadataHostWarnings(
		manifest,
		[]dbpkg.DomainBindingResolvedRow{{Domain: "staging.example.com"}},
		[]dbpkg.DomainBindingResolvedRow{{Domain: "example.com"}},
		"prod",
	)
	if err != nil {
		t.Fatalf("promoteMetadataHostWarnings() error = %v", err)
	}
	if len(warnings) != maxPromoteMetadataWarnings {
		t.Fatalf("expected capped warning count %d, got %d", maxPromoteMetadataWarnings, len(warnings))
	}
	if warnings[len(warnings)-1] != "additional metadata host warnings omitted: 6" {
		t.Fatalf("unexpected truncation warning: %#v", warnings[len(warnings)-1])
	}
	if !strings.Contains(warnings[0], "page=page-a field=canonicalURL") {
		t.Fatalf("unexpected first warning ordering: %#v", warnings[0])
	}
}

func TestPromoteMetadataHostWarningsRejectsMalformedHead(t *testing.T) {
	_, err := promoteMetadataHostWarnings(`{
  "resources": {
    "pages": [
      {
        "name": "index",
        "head": "bad"
      }
    ]
  }
}`, nil, nil, "prod")
	if err == nil || !strings.Contains(err.Error(), `parse head metadata for page "index"`) {
		t.Fatalf("expected head parse error, got %v", err)
	}
}

package renderer

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/loader"
)

func TestRenderStaticsWritesContentAddressedAssets(t *testing.T) {
	site, err := loader.LoadSite(filepath.Join("..", "..", "testdata", "site-with-assets"))
	if err != nil {
		t.Fatalf("LoadSite() error = %v", err)
	}

	outDir := t.TempDir()
	statics, err := renderStatics(site, outDir)
	if err != nil {
		t.Fatalf("renderStatics() error = %v", err)
	}

	if statics.TokensHref == "" || statics.DefaultHref == "" {
		t.Fatalf("expected style hrefs to be set")
	}
	if len(statics.AssetMap) != 2 {
		t.Fatalf("expected 2 assets to be copied, got %d", len(statics.AssetMap))
	}

	re := regexp.MustCompile(`^/assets(?:/[a-zA-Z0-9._-]+)*/[a-zA-Z0-9._-]+-[a-f0-9]{12}\.[a-zA-Z0-9]+$`)
	for original, rewritten := range statics.AssetMap {
		if !re.MatchString(rewritten) {
			t.Fatalf("asset %q was not rewritten to hashed filename: %q", original, rewritten)
		}
		if _, err := os.Stat(filepath.Join(outDir, rewritten[1:])); err != nil {
			t.Fatalf("rewritten asset file missing: %s (%v)", rewritten, err)
		}
	}
}

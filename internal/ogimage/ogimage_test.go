package ogimage

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"testing"
)

func TestGenerateDimensions(t *testing.T) {
	pngBytes, err := Generate(Card{Title: "Title", Description: "Description", SiteName: "Site"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if cfg.Width != Width || cfg.Height != Height {
		t.Fatalf("unexpected dimensions: got %dx%d want %dx%d", cfg.Width, cfg.Height, Width, Height)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	card := Card{
		Title:       "Ora for macOS",
		Description: "Local-first voice assistant",
		SiteName:    "sample",
	}
	first, err := Generate(card)
	if err != nil {
		t.Fatalf("Generate(first) error = %v", err)
	}
	second, err := Generate(card)
	if err != nil {
		t.Fatalf("Generate(second) error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("expected deterministic PNG bytes")
	}
}

func TestGenerateCacheKey(t *testing.T) {
	card := Card{
		Title:       "A",
		Description: "B",
		SiteName:    "C",
	}
	got := CacheKey(card)
	want := sha256.Sum256([]byte(templateVersion + "A" + "\x00" + "B" + "\x00" + "C" + "\x00" + ""))
	if got != want {
		t.Fatalf("CacheKey() mismatch: got %x want %x", got, want)
	}
}

func TestGenerateCacheKeyAccentIncluded(t *testing.T) {
	base := Card{Title: "T", Description: "D", SiteName: "S"}
	withAccent := Card{Title: "T", Description: "D", SiteName: "S", AccentColor: "#6d9ea3"}
	if CacheKey(base) == CacheKey(withAccent) {
		t.Fatal("expected different cache keys for cards with and without AccentColor")
	}
}

func TestGenerateUsesAccentColor(t *testing.T) {
	neutral, err := Generate(Card{Title: "T", Description: "D", SiteName: "S"})
	if err != nil {
		t.Fatalf("Generate(neutral) error = %v", err)
	}
	teal, err := Generate(Card{Title: "T", Description: "D", SiteName: "S", AccentColor: "#6d9ea3"})
	if err != nil {
		t.Fatalf("Generate(teal) error = %v", err)
	}
	if bytes.Equal(neutral, teal) {
		t.Fatal("expected different PNG output for different AccentColor values")
	}
}

func TestGenerateRendersText(t *testing.T) {
	withText, err := Generate(Card{Title: "Alpha", Description: "Beta", SiteName: "Gamma"})
	if err != nil {
		t.Fatalf("Generate(with text) error = %v", err)
	}
	withoutText, err := Generate(Card{})
	if err != nil {
		t.Fatalf("Generate(without text) error = %v", err)
	}
	if bytes.Equal(withText, withoutText) {
		t.Fatalf("expected output with text to differ from blank card output")
	}
}

func TestEmbeddedFontSubsetSizeGuard(t *testing.T) {
	const maxFontBytes = 50 * 1024
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "Inter-Regular.ttf", data: interRegularTTF},
		{name: "Inter-SemiBold.ttf", data: interSemiBoldTTF},
	} {
		if len(tc.data) > maxFontBytes {
			t.Fatalf("font %s too large: got %d bytes, max %d", tc.name, len(tc.data), maxFontBytes)
		}
	}
}

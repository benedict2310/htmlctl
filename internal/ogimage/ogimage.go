package ogimage

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	Width  = 1200
	Height = 630

	// templateVersion is included in every cache key.
	// Bump this whenever Generate's visual output changes (layout, fonts, colors, or canvas size).
	templateVersion     = "og-v1:"
	titleMaxLines       = 3
	descriptionMaxLines = 3
)

type Card struct {
	Title       string
	Description string
	SiteName    string
}

var (
	//go:embed fonts/JetBrainsMono-Bold.ttf
	jetBrainsMonoBoldTTF []byte
	//go:embed fonts/Inter-Regular.ttf
	interRegularTTF []byte

	fontLoadOnce sync.Once
	fontLoadErr  error

	jetBrainsMonoBoldFont *opentype.Font
	interRegularFont      *opentype.Font
)

func Generate(c Card) ([]byte, error) {
	if err := ensureFontsLoaded(); err != nil {
		return nil, err
	}
	card := normalizeCard(c)

	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	fillRect(img, img.Bounds(), color.RGBA{R: 13, G: 20, B: 33, A: 255})
	fillRect(img, image.Rect(0, 0, 24, Height), color.RGBA{R: 39, G: 94, B: 254, A: 255})
	fillRect(img, image.Rect(0, Height-70, Width, Height), color.RGBA{R: 18, G: 30, B: 48, A: 255})

	siteFace, err := newFace(interRegularFont, 32)
	if err != nil {
		return nil, fmt.Errorf("create site name font face: %w", err)
	}
	defer closeFace(siteFace)
	titleFace, err := newFace(jetBrainsMonoBoldFont, 66)
	if err != nil {
		return nil, fmt.Errorf("create title font face: %w", err)
	}
	defer closeFace(titleFace)
	descriptionFace, err := newFace(interRegularFont, 34)
	if err != nil {
		return nil, fmt.Errorf("create description font face: %w", err)
	}
	defer closeFace(descriptionFace)

	_ = drawWrappedText(img, siteFace, card.SiteName, 78, 94, Width-156, 40, 1, color.RGBA{R: 148, G: 173, B: 255, A: 255})
	nextY := drawWrappedText(img, titleFace, card.Title, 78, 188, Width-156, 76, titleMaxLines, color.RGBA{R: 245, G: 249, B: 255, A: 255})
	nextY += 34
	_ = drawWrappedText(img, descriptionFace, card.Description, 78, nextY, Width-156, 46, descriptionMaxLines, color.RGBA{R: 200, G: 212, B: 236, A: 255})

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return out.Bytes(), nil
}

func CacheKey(c Card) [32]byte {
	card := normalizeCard(c)
	payload := templateVersion + card.Title + "\x00" + card.Description + "\x00" + card.SiteName
	return sha256.Sum256([]byte(payload))
}

func ensureFontsLoaded() error {
	fontLoadOnce.Do(func() {
		jetBrainsMonoBoldFont, fontLoadErr = opentype.Parse(jetBrainsMonoBoldTTF)
		if fontLoadErr != nil {
			fontLoadErr = fmt.Errorf("parse JetBrainsMono-Bold.ttf: %w", fontLoadErr)
			return
		}
		interRegularFont, fontLoadErr = opentype.Parse(interRegularTTF)
		if fontLoadErr != nil {
			fontLoadErr = fmt.Errorf("parse Inter-Regular.ttf: %w", fontLoadErr)
		}
	})
	return fontLoadErr
}

func newFace(parsed *opentype.Font, size float64) (font.Face, error) {
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, err
	}
	return face, nil
}

func closeFace(face font.Face) {
	if closer, ok := face.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func normalizeCard(c Card) Card {
	return Card{
		Title:       normalizeWhitespace(c.Title),
		Description: normalizeWhitespace(c.Description),
		SiteName:    normalizeWhitespace(c.SiteName),
	}
}

func normalizeWhitespace(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func fillRect(img draw.Image, rect image.Rectangle, c color.Color) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func drawWrappedText(img draw.Image, face font.Face, text string, x, y, maxWidth, lineHeight, maxLines int, c color.Color) int {
	lines := wrapLines(face, text, maxWidth, maxLines)
	for _, line := range lines {
		drawer := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: face,
			Dot:  fixed.P(x, y),
		}
		drawer.DrawString(line)
		y += lineHeight
	}
	return y
}

func wrapLines(face font.Face, text string, maxWidth, maxLines int) []string {
	text = normalizeWhitespace(text)
	if text == "" || maxLines <= 0 {
		return nil
	}

	words := strings.Split(text, " ")
	lines := make([]string, 0, maxLines)
	wordIndex := 0
	for wordIndex < len(words) && len(lines) < maxLines {
		line := words[wordIndex]
		wordIndex++
		if textWidth(face, line) > maxWidth {
			lines = append(lines, fitWithEllipsis(face, line, maxWidth))
			continue
		}
		for wordIndex < len(words) {
			candidate := line + " " + words[wordIndex]
			if textWidth(face, candidate) > maxWidth {
				break
			}
			line = candidate
			wordIndex++
		}
		lines = append(lines, line)
	}

	if wordIndex < len(words) && len(lines) > 0 {
		lines[len(lines)-1] = fitWithEllipsis(face, lines[len(lines)-1]+" ...", maxWidth)
	}
	return lines
}

func fitWithEllipsis(face font.Face, text string, maxWidth int) string {
	const ellipsis = "..."
	if textWidth(face, text) <= maxWidth {
		return text
	}
	if textWidth(face, ellipsis) > maxWidth {
		return ""
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimRight(string(runes), " ") + ellipsis
		if textWidth(face, candidate) <= maxWidth {
			return candidate
		}
	}
	return ellipsis
}

func textWidth(face font.Face, text string) int {
	return font.MeasureString(face, text).Ceil()
}

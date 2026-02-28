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
	templateVersion     = "og-v3:"
	titleMaxLines       = 3
	descriptionMaxLines = 3

	// defaultAccentColor is the neutral accent used when Card.AccentColor is absent or invalid.
	// Sites override this by defining --og-accent in their tokens.css.
	defaultAccentColor = "#64748b" // slate-500
)

// Card holds the text and branding inputs for OG image generation.
// AccentColor is an optional CSS hex color (#rgb or #rrggbb) used for the
// left stripe and site-name text. When empty or invalid, a neutral slate
// accent is used so the card is presentable for any generic website.
type Card struct {
	Title       string
	Description string
	SiteName    string
	AccentColor string // optional: #rgb or #rrggbb hex; read from --og-accent in tokens.css
}

var (
	//go:embed fonts/Inter-SemiBold.ttf
	interSemiBoldTTF []byte
	//go:embed fonts/Inter-Regular.ttf
	interRegularTTF []byte

	fontLoadOnce sync.Once
	fontLoadErr  error

	interSemiBoldFont *opentype.Font
	interRegularFont  *opentype.Font
)

func Generate(c Card) ([]byte, error) {
	if err := ensureFontsLoaded(); err != nil {
		return nil, err
	}
	card := normalizeCard(c)

	// Resolve accent: use Card.AccentColor if valid, otherwise the neutral default.
	accent, ok := parseHexColor(card.AccentColor)
	if !ok {
		accent, _ = parseHexColor(defaultAccentColor)
	}

	// Neutral dark palette — background and text are generic slate tones.
	// Only the accent stripe and site-name colour come from the website's design.
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	fillRect(img, img.Bounds(), color.RGBA{R: 15, G: 23, B: 42, A: 255})                    // slate-950 background
	fillRect(img, image.Rect(0, 0, 24, Height), accent)                                      // brand accent stripe
	fillRect(img, image.Rect(0, Height-70, Width, Height), color.RGBA{R: 30, G: 41, B: 59, A: 255}) // slate-800 footer

	siteFace, err := newFace(interRegularFont, 32)
	if err != nil {
		return nil, fmt.Errorf("create site name font face: %w", err)
	}
	defer closeFace(siteFace)
	titleFace, err := newFace(interSemiBoldFont, 66)
	if err != nil {
		return nil, fmt.Errorf("create title font face: %w", err)
	}
	defer closeFace(titleFace)
	descriptionFace, err := newFace(interRegularFont, 34)
	if err != nil {
		return nil, fmt.Errorf("create description font face: %w", err)
	}
	defer closeFace(descriptionFace)

	_ = drawWrappedText(img, siteFace, card.SiteName, 78, 94, Width-156, 40, 1, accent)
	nextY := drawWrappedText(img, titleFace, card.Title, 78, 188, Width-156, 80, titleMaxLines, color.RGBA{R: 241, G: 245, B: 249, A: 255}) // slate-100
	nextY += 32
	_ = drawWrappedText(img, descriptionFace, card.Description, 78, nextY, Width-156, 46, descriptionMaxLines, color.RGBA{R: 148, G: 163, B: 184, A: 255}) // slate-400

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return out.Bytes(), nil
}

func CacheKey(c Card) [32]byte {
	card := normalizeCard(c)
	payload := templateVersion + card.Title + "\x00" + card.Description + "\x00" + card.SiteName + "\x00" + card.AccentColor
	return sha256.Sum256([]byte(payload))
}

func ensureFontsLoaded() error {
	fontLoadOnce.Do(func() {
		interSemiBoldFont, fontLoadErr = opentype.Parse(interSemiBoldTTF)
		if fontLoadErr != nil {
			fontLoadErr = fmt.Errorf("parse Inter-SemiBold.ttf: %w", fontLoadErr)
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
		AccentColor: strings.ToLower(strings.TrimSpace(c.AccentColor)),
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

// parseHexColor parses a CSS hex color (#rgb or #rrggbb) into color.RGBA.
// The input may or may not include the leading '#'. Case-insensitive.
// Returns (zero, false) if the value is not a valid hex color.
func parseHexColor(s string) (color.RGBA, bool) {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#")))
	switch len(s) {
	case 3:
		r, ok1 := hexByte(s[0:1] + s[0:1])
		g, ok2 := hexByte(s[1:2] + s[1:2])
		b, ok3 := hexByte(s[2:3] + s[2:3])
		if ok1 && ok2 && ok3 {
			return color.RGBA{R: r, G: g, B: b, A: 255}, true
		}
	case 6:
		r, ok1 := hexByte(s[0:2])
		g, ok2 := hexByte(s[2:4])
		b, ok3 := hexByte(s[4:6])
		if ok1 && ok2 && ok3 {
			return color.RGBA{R: r, G: g, B: b, A: 255}, true
		}
	}
	return color.RGBA{}, false
}

func hexByte(s string) (byte, bool) {
	if len(s) != 2 {
		return 0, false
	}
	hi := hexNibble(s[0])
	lo := hexNibble(s[1])
	if hi < 0 || lo < 0 {
		return 0, false
	}
	return byte(hi<<4 | lo), true
}

func hexNibble(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	default:
		return -1
	}
}

package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/benedict2310/htmlctl/pkg/model"
)

type renderedStatics struct {
	TokensHref  string
	DefaultHref string
	ScriptSrc   string
	AssetMap    map[string]string
}

func renderStatics(site *model.Site, outputDir string) (renderedStatics, error) {
	statics := renderedStatics{AssetMap: map[string]string{}}

	tokensRel, err := writeContentAddressedTextFile(outputDir, "styles", "tokens.css", site.Styles.TokensCSS)
	if err != nil {
		return statics, err
	}
	defaultRel, err := writeContentAddressedTextFile(outputDir, "styles", "default.css", site.Styles.DefaultCSS)
	if err != nil {
		return statics, err
	}
	statics.TokensHref = "/" + filepath.ToSlash(tokensRel)
	statics.DefaultHref = "/" + filepath.ToSlash(defaultRel)

	if site.ScriptPath != "" {
		scriptAbs := filepath.Join(site.RootDir, filepath.FromSlash(site.ScriptPath))
		scriptBytes, err := os.ReadFile(scriptAbs)
		if err != nil {
			return statics, fmt.Errorf("read script file %s: %w", scriptAbs, err)
		}

		scriptRel, err := writeContentAddressedBytesFile(outputDir, "scripts", "site.js", normalizeLFBytes(scriptBytes))
		if err != nil {
			return statics, err
		}
		statics.ScriptSrc = "/" + filepath.ToSlash(scriptRel)
	}

	assets := append([]model.Asset(nil), site.Assets...)
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	for _, asset := range assets {
		src := filepath.Join(site.RootDir, "assets", filepath.FromSlash(asset.Path))
		content, err := os.ReadFile(src)
		if err != nil {
			return statics, fmt.Errorf("read asset file %s: %w", src, err)
		}

		dir := filepath.Dir(asset.Path)
		if dir == "." {
			dir = ""
		}
		assetRelDir := filepath.ToSlash(filepath.Join("assets", filepath.FromSlash(dir)))
		assetRel, err := writeContentAddressedBytesFile(outputDir, assetRelDir, filepath.Base(asset.Path), content)
		if err != nil {
			return statics, err
		}

		originalKey := filepath.ToSlash(filepath.Join("assets", asset.Path))
		statics.AssetMap[originalKey] = "/" + filepath.ToSlash(assetRel)
	}

	return statics, nil
}

func writeContentAddressedTextFile(outputDir, relDir, canonicalName, content string) (string, error) {
	normalized := []byte(normalizeLFString(content))
	return writeContentAddressedBytesFile(outputDir, relDir, canonicalName, normalized)
}

func writeContentAddressedBytesFile(outputDir, relDir, canonicalName string, content []byte) (string, error) {
	targetName := hashedFilename(canonicalName, content)
	relPath := filepath.ToSlash(filepath.Join(filepath.FromSlash(relDir), targetName))
	path := filepath.Join(outputDir, filepath.FromSlash(relPath))
	if err := writeFileAtomic(path, content); err != nil {
		return "", fmt.Errorf("write file %s: %w", path, err)
	}
	return relPath, nil
}

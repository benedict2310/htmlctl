package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
)

type renderedStatics struct {
	TokensHref          string
	DefaultHref         string
	ScriptSrc           string
	AssetMap            map[string]string
	ComponentStyleHrefs map[string]string
	ComponentScriptSrcs map[string]string
}

func renderStatics(site *model.Site, outputDir string) (renderedStatics, error) {
	statics := renderedStatics{
		AssetMap:            map[string]string{},
		ComponentStyleHrefs: map[string]string{},
		ComponentScriptSrcs: map[string]string{},
	}

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

	for _, name := range sortedComponentNames(site.Components) {
		component := site.Components[name]
		if strings.TrimSpace(component.CSS) != "" {
			rel, err := writeContentAddressedTextFile(outputDir, "components", component.Name+".css", component.CSS)
			if err != nil {
				return statics, err
			}
			statics.ComponentStyleHrefs[component.Name] = "/" + filepath.ToSlash(rel)
		}
		if strings.TrimSpace(component.JS) != "" {
			rel, err := writeContentAddressedTextFile(outputDir, "components", component.Name+".js", component.JS)
			if err != nil {
				return statics, err
			}
			statics.ComponentScriptSrcs[component.Name] = "/" + filepath.ToSlash(rel)
		}
	}

	if err := renderBranding(site, outputDir); err != nil {
		return statics, err
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
		originalRel := filepath.ToSlash(filepath.Join("assets", asset.Path))
		if err := writeOriginalBytesFile(outputDir, originalRel, content); err != nil {
			return statics, err
		}

		statics.AssetMap[originalRel] = "/" + filepath.ToSlash(assetRel)
	}

	return statics, nil
}

func renderBranding(site *model.Site, outputDir string) error {
	if site == nil || len(site.Branding) == 0 {
		return nil
	}
	for _, slot := range sortedBrandingSlots(site.Branding) {
		targetName, ok := brandingPublicFilename(slot)
		if !ok {
			continue
		}
		src := filepath.Join(site.RootDir, filepath.FromSlash(site.Branding[slot].SourcePath))
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read branding file %s: %w", src, err)
		}
		if err := writeOriginalBytesFile(outputDir, targetName, content); err != nil {
			return err
		}
	}
	return nil
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

func writeOriginalBytesFile(outputDir, relPath string, content []byte) error {
	path := filepath.Join(outputDir, filepath.FromSlash(relPath))
	if err := writeFileAtomic(path, content); err != nil {
		return fmt.Errorf("write original file %s: %w", path, err)
	}
	return nil
}

func brandingPublicFilename(slot string) (string, bool) {
	switch slot {
	case "svg":
		return "favicon.svg", true
	case "ico":
		return "favicon.ico", true
	case "apple_touch":
		return "apple-touch-icon.png", true
	default:
		return "", false
	}
}

func sortedBrandingSlots(v map[string]model.BrandingAsset) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedComponentNames(v map[string]model.Component) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

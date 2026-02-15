package loader

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
	"gopkg.in/yaml.v3"
)

// LoadSite parses a site directory into a strongly typed aggregate model.
func LoadSite(dirPath string) (*model.Site, error) {
	root, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("resolve site path: %w", err)
	}

	website, err := loadWebsite(root)
	if err != nil {
		return nil, err
	}

	pages, err := loadPages(root)
	if err != nil {
		return nil, err
	}

	components, err := loadComponents(root)
	if err != nil {
		return nil, err
	}

	styles, err := loadStyles(root, website.Spec.DefaultStyleBundle)
	if err != nil {
		return nil, err
	}

	scriptPath, err := loadScriptPath(root)
	if err != nil {
		return nil, err
	}

	assets, err := loadAssets(root)
	if err != nil {
		return nil, err
	}

	site := &model.Site{
		RootDir:    root,
		Website:    website,
		Pages:      pages,
		Components: components,
		Styles:     styles,
		ScriptPath: scriptPath,
		Assets:     assets,
	}

	if err := ValidateSite(site); err != nil {
		return nil, err
	}

	return site, nil
}

func loadWebsite(root string) (model.Website, error) {
	var website model.Website

	path := filepath.Join(root, "website.yaml")
	if err := mustFile(path); err != nil {
		return website, err
	}

	if err := decodeYAMLFile(path, &website); err != nil {
		return website, err
	}

	return website, nil
}

func loadPages(root string) (map[string]model.Page, error) {
	pattern := filepath.Join(root, "pages", "*.page.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob page files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("missing page files in %s", filepath.Join(root, "pages"))
	}

	sort.Strings(files)

	pages := make(map[string]model.Page, len(files))
	for _, path := range files {
		var page model.Page
		if err := decodeYAMLFile(path, &page); err != nil {
			return nil, err
		}

		name := page.Metadata.Name
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(path), ".page.yaml")
			page.Metadata.Name = name
		}
		pages[name] = page
	}

	return pages, nil
}

func loadComponents(root string) (map[string]model.Component, error) {
	dir := filepath.Join(root, "components")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]model.Component{}, nil
		}
		return nil, fmt.Errorf("read components directory %s: %w", dir, err)
	}

	components := make(map[string]model.Component)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".html" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read component file %s: %w", path, err)
		}

		name := strings.TrimSuffix(entry.Name(), ".html")
		components[name] = model.Component{
			Name:  name,
			Scope: model.ComponentScopeGlobal,
			HTML:  normalizeLineEndings(string(content)),
		}
	}

	return components, nil
}

func loadStyles(root, bundleName string) (model.StyleBundle, error) {
	if bundleName == "" {
		bundleName = "default"
	}

	stylesDir := filepath.Join(root, "styles")
	tokensPath := filepath.Join(stylesDir, "tokens.css")
	defaultPath := filepath.Join(stylesDir, "default.css")

	if err := mustFile(tokensPath); err != nil {
		return model.StyleBundle{}, err
	}
	if err := mustFile(defaultPath); err != nil {
		return model.StyleBundle{}, err
	}

	tokensCSS, err := os.ReadFile(tokensPath)
	if err != nil {
		return model.StyleBundle{}, fmt.Errorf("read style file %s: %w", tokensPath, err)
	}
	defaultCSS, err := os.ReadFile(defaultPath)
	if err != nil {
		return model.StyleBundle{}, fmt.Errorf("read style file %s: %w", defaultPath, err)
	}

	return model.StyleBundle{
		Name:       bundleName,
		TokensCSS:  normalizeLineEndings(string(tokensCSS)),
		DefaultCSS: normalizeLineEndings(string(defaultCSS)),
	}, nil
}

func loadScriptPath(root string) (string, error) {
	scriptPath := filepath.Join(root, "scripts", "site.js")
	if _, err := os.Stat(scriptPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("check script file %s: %w", scriptPath, err)
	}

	return filepath.ToSlash(filepath.Join("scripts", "site.js")), nil
}

func loadAssets(root string) ([]model.Asset, error) {
	assetsDir := filepath.Join(root, "assets")
	if _, err := os.Stat(assetsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []model.Asset{}, nil
		}
		return nil, fmt.Errorf("check assets directory %s: %w", assetsDir, err)
	}

	assets := []model.Asset{}
	err := filepath.WalkDir(assetsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(assetsDir, path)
		if err != nil {
			return err
		}

		assets = append(assets, model.Asset{
			Name: filepath.Base(path),
			Path: filepath.ToSlash(relPath),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk assets directory %s: %w", assetsDir, err)
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	return assets, nil
}

func decodeYAMLFile(path string, out any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read yaml file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(content, out); err != nil {
		return fmt.Errorf("parse yaml file %s: %w", path, err)
	}

	return nil
}

func mustFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("required file missing: %s", path)
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("required file is a directory: %s", path)
	}
	return nil
}

func normalizeLineEndings(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

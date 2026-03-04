package bundle

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/model"
	"gopkg.in/yaml.v3"
)

const maxBundleSizeBytes = 100 * 1024 * 1024

type BuildOptions struct {
	Source *Source
}

// BuildTarFromPath validates a site directory or supported site file path and produces a bundle.
func BuildTarFromPath(sourcePath string, website string) ([]byte, Manifest, error) {
	return BuildTarFromPathWithOptions(sourcePath, website, BuildOptions{})
}

// BuildTarFromPathWithOptions validates a site directory or supported site file path and produces a bundle.
func BuildTarFromPathWithOptions(sourcePath string, website string, opts BuildOptions) ([]byte, Manifest, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("stat source path %s: %w", sourcePath, err)
	}
	if info.IsDir() {
		return BuildTarFromDirWithOptions(sourcePath, website, opts)
	}
	return BuildTarFromFileWithOptions(sourcePath, website, opts)
}

// BuildTarFromDir validates a site directory and produces a full-apply tar bundle.
func BuildTarFromDir(dir string, website string) ([]byte, Manifest, error) {
	return BuildTarFromDirWithOptions(dir, website, BuildOptions{})
}

// BuildTarFromDirWithOptions validates a site directory and produces a full-apply tar bundle.
func BuildTarFromDirWithOptions(dir string, website string, opts BuildOptions) ([]byte, Manifest, error) {
	website = strings.TrimSpace(website)
	if website == "" {
		return nil, Manifest{}, fmt.Errorf("website is required")
	}

	site, err := loader.LoadSite(dir)
	if err != nil {
		return nil, Manifest{}, err
	}
	if name := strings.TrimSpace(site.Website.Metadata.Name); name != "" && name != website {
		return nil, Manifest{}, fmt.Errorf("website.yaml metadata.name %q does not match target website %q", name, website)
	}

	files := make(map[string]int64)
	totalBytes := int64(0)
	resources := make([]Resource, 0, len(site.Components)+len(site.Pages)+len(site.Assets)+len(site.Branding)+3)

	websiteRel := "website.yaml"
	websiteHash, websiteSize, err := hashFile(site.RootDir, websiteRel)
	if err != nil {
		return nil, Manifest{}, err
	}
	files[websiteRel] = websiteSize
	totalBytes += websiteSize
	if totalBytes > maxBundleSizeBytes {
		return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
	}
	resources = append(resources, Resource{
		Kind: "Website",
		Name: website,
		File: websiteRel,
		Hash: websiteHash,
	})

	for _, slot := range sortedBrandingSlots(site.Branding) {
		asset := site.Branding[slot]
		rel := filepath.ToSlash(asset.SourcePath)
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, Manifest{}, err
		}
		files[rel] = size
		totalBytes += size
		if totalBytes > maxBundleSizeBytes {
			return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		resources = append(resources, Resource{
			Kind:        "WebsiteIcon",
			Name:        websiteIconResourceName(slot),
			File:        rel,
			Hash:        hash,
			ContentType: contentTypeForPath(rel),
			Size:        size,
		})
	}

	componentNames := sortedComponentNames(site.Components)
	for _, name := range componentNames {
		refs, err := componentFileRefs(site.RootDir, site.Components[name])
		if err != nil {
			return nil, Manifest{}, err
		}
		for _, ref := range refs {
			size, err := addBundleFile(site.RootDir, ref.File, files, &totalBytes)
			if err != nil {
				return nil, Manifest{}, err
			}
			_ = size
		}
		resources = append(resources, Resource{
			Kind:  "Component",
			Name:  name,
			Files: refs,
		})
	}

	pageFiles, err := pageFiles(site.RootDir)
	if err != nil {
		return nil, Manifest{}, err
	}
	for _, rel := range pageFiles {
		content, err := readSiteFile(site.RootDir, rel)
		if err != nil {
			return nil, Manifest{}, err
		}
		name, err := pageNameFromYAML(content, rel)
		if err != nil {
			return nil, Manifest{}, err
		}
		size := int64(len(content))
		files[rel] = size
		totalBytes += size
		if totalBytes > maxBundleSizeBytes {
			return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		resources = append(resources, Resource{
			Kind: "Page",
			Name: name,
			File: rel,
			Hash: contentHash(content),
		})
	}

	tokensRel := filepath.ToSlash(filepath.Join("styles", "tokens.css"))
	tokensHash, tokensSize, err := hashFile(site.RootDir, tokensRel)
	if err != nil {
		return nil, Manifest{}, err
	}
	files[tokensRel] = tokensSize
	totalBytes += tokensSize
	if totalBytes > maxBundleSizeBytes {
		return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
	}

	defaultRel := filepath.ToSlash(filepath.Join("styles", "default.css"))
	defaultHash, defaultSize, err := hashFile(site.RootDir, defaultRel)
	if err != nil {
		return nil, Manifest{}, err
	}
	files[defaultRel] = defaultSize
	totalBytes += defaultSize
	if totalBytes > maxBundleSizeBytes {
		return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
	}

	styleBundleName := strings.TrimSpace(site.Styles.Name)
	if styleBundleName == "" {
		styleBundleName = "default"
	}
	resources = append(resources, Resource{
		Kind: "StyleBundle",
		Name: styleBundleName,
		Files: []FileRef{
			{File: tokensRel, Hash: tokensHash},
			{File: defaultRel, Hash: defaultHash},
		},
	})

	if strings.TrimSpace(site.ScriptPath) != "" {
		scriptRel := filepath.ToSlash(site.ScriptPath)
		hash, size, err := hashFile(site.RootDir, scriptRel)
		if err != nil {
			return nil, Manifest{}, err
		}
		files[scriptRel] = size
		totalBytes += size
		if totalBytes > maxBundleSizeBytes {
			return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		resources = append(resources, Resource{
			Kind:        "Script",
			Name:        scriptRel,
			File:        scriptRel,
			Hash:        hash,
			ContentType: contentTypeForPath(scriptRel),
			Size:        size,
		})
	}

	for _, asset := range site.Assets {
		rel := filepath.ToSlash(filepath.Join("assets", asset.Path))
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, Manifest{}, err
		}
		files[rel] = size
		totalBytes += size
		if totalBytes > maxBundleSizeBytes {
			return nil, Manifest{}, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		resources = append(resources, Resource{
			Kind:        "Asset",
			Name:        rel,
			File:        rel,
			Hash:        hash,
			ContentType: contentTypeForPath(rel),
			Size:        size,
		})
	}

	manifest := Manifest{
		APIVersion: "htmlctl.dev/v1",
		Kind:       "Bundle",
		Mode:       ApplyModeFull,
		Website:    website,
		Source:     opts.Source,
		Resources:  resources,
	}
	if err := manifest.Validate(); err != nil {
		return nil, Manifest{}, err
	}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("marshal manifest: %w", err)
	}

	archive, err := buildTarArchive(site.RootDir, files, manifestBytes)
	if err != nil {
		return nil, Manifest{}, err
	}
	return archive, manifest, nil
}

// BuildTarFromFileWithOptions validates a supported site file path and produces a partial-apply tar bundle.
func BuildTarFromFileWithOptions(filePath string, website string, opts BuildOptions) ([]byte, Manifest, error) {
	website = strings.TrimSpace(website)
	if website == "" {
		return nil, Manifest{}, fmt.Errorf("website is required")
	}
	root, rel, err := resolveSiteRootAndRel(filePath)
	if err != nil {
		return nil, Manifest{}, err
	}
	site, err := loader.LoadSite(root)
	if err != nil {
		return nil, Manifest{}, err
	}
	if name := strings.TrimSpace(site.Website.Metadata.Name); name != "" && name != website {
		return nil, Manifest{}, fmt.Errorf("website.yaml metadata.name %q does not match target website %q", name, website)
	}

	files := make(map[string]int64)
	totalBytes := int64(0)
	resources, err := partialResourcesForPath(site, rel, files, &totalBytes)
	if err != nil {
		return nil, Manifest{}, err
	}
	manifest := Manifest{
		APIVersion: "htmlctl.dev/v1",
		Kind:       "Bundle",
		Mode:       ApplyModePartial,
		Website:    website,
		Source:     opts.Source,
		Resources:  resources,
	}
	if err := manifest.Validate(); err != nil {
		return nil, Manifest{}, err
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("marshal manifest: %w", err)
	}
	archive, err := buildTarArchive(site.RootDir, files, manifestBytes)
	if err != nil {
		return nil, Manifest{}, err
	}
	return archive, manifest, nil
}

func readSiteFile(root, rel string) ([]byte, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	content, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read site file %s: %w", abs, err)
	}
	return content, nil
}

func pageFiles(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "pages", "*.page.yaml"))
	if err != nil {
		return nil, fmt.Errorf("glob page files: %w", err)
	}
	sort.Strings(matches)
	out := make([]string, 0, len(matches))
	for _, abs := range matches {
		out = append(out, filepath.ToSlash(filepath.Join("pages", filepath.Base(abs))))
	}
	return out, nil
}

func pageNameFromYAML(content []byte, rel string) (string, error) {
	var page model.Page
	if err := yaml.Unmarshal(content, &page); err != nil {
		return "", fmt.Errorf("parse page file %s: %w", rel, err)
	}
	name := strings.TrimSpace(page.Metadata.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(rel), ".page.yaml")
	}
	if name == "" {
		return "", fmt.Errorf("page file %s has empty name", rel)
	}
	return name, nil
}

func sortedComponentNames(components map[string]model.Component) []string {
	out := make([]string, 0, len(components))
	for name := range components {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func componentFileRefs(root string, component model.Component) ([]FileRef, error) {
	refs := make([]FileRef, 0, 3)
	for _, rel := range []string{
		filepath.ToSlash(filepath.Join("components", component.Name+".html")),
	} {
		hash, _, err := hashFile(root, rel)
		if err != nil {
			return nil, err
		}
		refs = append(refs, FileRef{File: rel, Hash: hash})
	}
	if strings.TrimSpace(component.CSS) != "" {
		rel := filepath.ToSlash(filepath.Join("components", component.Name+".css"))
		hash, _, err := hashFile(root, rel)
		if err != nil {
			return nil, err
		}
		refs = append(refs, FileRef{File: rel, Hash: hash})
	}
	if strings.TrimSpace(component.JS) != "" {
		rel := filepath.ToSlash(filepath.Join("components", component.Name+".js"))
		hash, _, err := hashFile(root, rel)
		if err != nil {
			return nil, err
		}
		refs = append(refs, FileRef{File: rel, Hash: hash})
	}
	return refs, nil
}

func sortedMapKeys(v map[string]int64) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedBrandingSlots(v map[string]model.BrandingAsset) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func websiteIconResourceName(slot string) string {
	switch slot {
	case "svg":
		return "website-icon-svg"
	case "ico":
		return "website-icon-ico"
	case "apple_touch":
		return "website-icon-apple-touch"
	default:
		return "website-icon-" + strings.ReplaceAll(slot, "_", "-")
	}
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func contentTypeForPath(rel string) string {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(rel))) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	}
	typ := strings.TrimSpace(mime.TypeByExtension(filepath.Ext(rel)))
	if typ == "" {
		return "application/octet-stream"
	}
	return typ
}

func writeTarEntry(tw *tar.Writer, rel string, content []byte) error {
	hdr := &tar.Header{
		Name: rel,
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header %s: %w", rel, err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write tar content %s: %w", rel, err)
	}
	return nil
}

func writeTarFileFromDisk(tw *tar.Writer, root, rel string, size int64) error {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	f, err := os.Open(abs)
	if err != nil {
		return fmt.Errorf("open site file %s: %w", abs, err)
	}
	defer f.Close()

	hdr := &tar.Header{
		Name: rel,
		Mode: 0o644,
		Size: size,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header %s: %w", rel, err)
	}
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("write tar content %s: %w", rel, err)
	}
	return nil
}

func buildTarArchive(root string, files map[string]int64, manifestBytes []byte) ([]byte, error) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if err := writeTarEntry(tw, "manifest.json", manifestBytes); err != nil {
		return nil, err
	}
	for _, rel := range sortedMapKeys(files) {
		if err := writeTarFileFromDisk(tw, root, rel, files[rel]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar archive: %w", err)
	}
	return archive.Bytes(), nil
}

func hashFile(root, rel string) (string, int64, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	f, err := os.Open(abs)
	if err != nil {
		return "", 0, fmt.Errorf("open site file %s: %w", abs, err)
	}
	defer f.Close()

	hasher := sha256.New()
	n, err := io.Copy(hasher, f)
	if err != nil {
		return "", 0, fmt.Errorf("hash site file %s: %w", abs, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), n, nil
}

func addBundleFile(root, rel string, files map[string]int64, totalBytes *int64) (int64, error) {
	hash, size, err := hashFile(root, rel)
	if err != nil {
		return 0, err
	}
	_ = hash
	files[rel] = size
	*totalBytes += size
	if *totalBytes > maxBundleSizeBytes {
		return 0, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
	}
	return size, nil
}

func resolveSiteRootAndRel(filePath string) (string, string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve source path: %w", err)
	}
	current := filepath.Dir(abs)
	for {
		if _, err := os.Stat(filepath.Join(current, "website.yaml")); err == nil {
			rel, err := filepath.Rel(current, abs)
			if err != nil {
				return "", "", fmt.Errorf("resolve relative source path: %w", err)
			}
			return current, filepath.ToSlash(rel), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", "", fmt.Errorf("source file %s is not inside a site root containing website.yaml", filePath)
}

func partialResourcesForPath(site *model.Site, rel string, files map[string]int64, totalBytes *int64) ([]Resource, error) {
	switch rel {
	case "website.yaml":
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, err
		}
		files[rel] = size
		*totalBytes += size
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		return []Resource{{Kind: "Website", Name: site.Website.Metadata.Name, File: rel, Hash: hash}}, nil
	case filepath.ToSlash(filepath.Join("styles", "tokens.css")), filepath.ToSlash(filepath.Join("styles", "default.css")):
		tokensRel := filepath.ToSlash(filepath.Join("styles", "tokens.css"))
		defaultRel := filepath.ToSlash(filepath.Join("styles", "default.css"))
		tokensHash, tokensSize, err := hashFile(site.RootDir, tokensRel)
		if err != nil {
			return nil, err
		}
		files[tokensRel] = tokensSize
		*totalBytes += tokensSize
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		defaultHash, defaultSize, err := hashFile(site.RootDir, defaultRel)
		if err != nil {
			return nil, err
		}
		files[defaultRel] = defaultSize
		*totalBytes += defaultSize
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		styleBundleName := strings.TrimSpace(site.Styles.Name)
		if styleBundleName == "" {
			styleBundleName = "default"
		}
		return []Resource{{
			Kind: "StyleBundle",
			Name: styleBundleName,
			Files: []FileRef{
				{File: tokensRel, Hash: tokensHash},
				{File: defaultRel, Hash: defaultHash},
			},
		}}, nil
	case filepath.ToSlash(filepath.Join("scripts", "site.js")):
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, err
		}
		files[rel] = size
		*totalBytes += size
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		return []Resource{{Kind: "Script", Name: rel, File: rel, Hash: hash, ContentType: contentTypeForPath(rel), Size: size}}, nil
	}

	if strings.HasPrefix(rel, "components/") {
		name, ok := componentNameFromRel(rel)
		if !ok {
			return nil, fmt.Errorf("unsupported component path %q; use components/<name>.html, .css, or .js", rel)
		}
		component, ok := site.Components[name]
		if !ok {
			return nil, fmt.Errorf("component %q not found", name)
		}
		refs, err := componentFileRefs(site.RootDir, component)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			size, err := addBundleFile(site.RootDir, ref.File, files, totalBytes)
			if err != nil {
				return nil, err
			}
			_ = size
		}
		return []Resource{{Kind: "Component", Name: name, Files: refs}}, nil
	}
	if strings.HasPrefix(rel, "pages/") && strings.HasSuffix(rel, ".page.yaml") {
		content, err := readSiteFile(site.RootDir, rel)
		if err != nil {
			return nil, err
		}
		name, err := pageNameFromYAML(content, rel)
		if err != nil {
			return nil, err
		}
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, err
		}
		files[rel] = size
		*totalBytes += size
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		resources := []Resource{{Kind: "Page", Name: name, File: rel, Hash: hash}}
		page, ok := site.Pages[name]
		if !ok {
			return nil, fmt.Errorf("page %q not found in loaded site", name)
		}
		seenComponents := map[string]struct{}{}
		for _, item := range page.Spec.Layout {
			componentName := strings.TrimSpace(item.Include)
			if componentName == "" {
				continue
			}
			if _, ok := seenComponents[componentName]; ok {
				continue
			}
			component, ok := site.Components[componentName]
			if !ok {
				return nil, fmt.Errorf("page %q references missing component %q", name, componentName)
			}
			refs, err := componentFileRefs(site.RootDir, component)
			if err != nil {
				return nil, err
			}
			for _, ref := range refs {
				size, err := addBundleFile(site.RootDir, ref.File, files, totalBytes)
				if err != nil {
					return nil, err
				}
				_ = size
			}
			resources = append(resources, Resource{Kind: "Component", Name: componentName, Files: refs})
			seenComponents[componentName] = struct{}{}
		}
		return resources, nil
	}
	if strings.HasPrefix(rel, "assets/") {
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, err
		}
		files[rel] = size
		*totalBytes += size
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		return []Resource{{Kind: "Asset", Name: rel, File: rel, Hash: hash, ContentType: contentTypeForPath(rel), Size: size}}, nil
	}
	for slot, asset := range site.Branding {
		if asset.SourcePath != rel {
			continue
		}
		hash, size, err := hashFile(site.RootDir, rel)
		if err != nil {
			return nil, err
		}
		files[rel] = size
		*totalBytes += size
		if *totalBytes > maxBundleSizeBytes {
			return nil, fmt.Errorf("bundle exceeds %d bytes; reduce site size or split large assets", maxBundleSizeBytes)
		}
		return []Resource{{Kind: "WebsiteIcon", Name: websiteIconResourceName(slot), File: rel, Hash: hash, ContentType: contentTypeForPath(rel), Size: size}}, nil
	}
	return nil, fmt.Errorf("unsupported apply path %q", rel)
}

func componentNameFromRel(rel string) (string, bool) {
	if path.Dir(rel) != "components" {
		return "", false
	}
	base := path.Base(rel)
	ext := strings.ToLower(path.Ext(base))
	switch ext {
	case ".html", ".css", ".js":
	default:
		return "", false
	}
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		return "", false
	}
	return name, true
}

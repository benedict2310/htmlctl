package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
)

// Render stitches a parsed site into deterministic static output files.
func Render(site *model.Site, outputDir string) error {
	if site == nil {
		return fmt.Errorf("site is nil")
	}
	if outputDir == "" {
		return fmt.Errorf("output directory is required")
	}

	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("clean output directory %s: %w", outputDir, err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %s: %w", outputDir, err)
	}

	statics, err := renderStatics(site, outputDir)
	if err != nil {
		return err
	}

	pages := sortedPages(site.Pages)
	for _, page := range pages {
		contentHTML, err := stitchComponents(page.Spec.Layout, site.Components)
		if err != nil {
			return fmt.Errorf("stitch page %q: %w", page.Metadata.Name, err)
		}

		htmlBytes, err := renderDefaultTemplate(pageTemplateData{
			Title:       page.Spec.Title,
			Description: page.Spec.Description,
			StyleHrefs:  []string{statics.TokensHref, statics.DefaultHref},
			ContentHTML: contentHTML,
			ScriptSrc:   statics.ScriptSrc,
		})
		if err != nil {
			return fmt.Errorf("render page %q: %w", page.Metadata.Name, err)
		}

		outputRel, err := routeToOutputPath(page.Spec.Route)
		if err != nil {
			return fmt.Errorf("resolve output path for page %q: %w", page.Metadata.Name, err)
		}
		if err := writeFileAtomic(filepath.Join(outputDir, filepath.FromSlash(outputRel)), htmlBytes); err != nil {
			return fmt.Errorf("write page %q output: %w", page.Metadata.Name, err)
		}
	}

	return nil
}

func sortedPages(pages map[string]model.Page) []model.Page {
	out := make([]model.Page, 0, len(pages))
	for _, page := range pages {
		out = append(out, page)
	}
	sort.Slice(out, func(i, j int) bool {
		ri := normalizeRoute(out[i].Spec.Route)
		rj := normalizeRoute(out[j].Spec.Route)
		if ri == rj {
			return out[i].Metadata.Name < out[j].Metadata.Name
		}
		return ri < rj
	})
	return out
}

func stitchComponents(layout []model.PageLayoutItem, components map[string]model.Component) (string, error) {
	chunks := make([]string, 0, len(layout))
	for _, item := range layout {
		include := strings.TrimSpace(item.Include)
		component, ok := components[include]
		if !ok {
			return "", fmt.Errorf("missing component %q", include)
		}
		chunks = append(chunks, strings.TrimRight(normalizeLFString(component.HTML), "\n"))
	}
	if len(chunks) == 0 {
		return "", nil
	}
	return strings.Join(chunks, "\n") + "\n", nil
}

func routeToOutputPath(route string) (string, error) {
	route = normalizeRoute(route)
	if route == "" {
		return "", fmt.Errorf("route is empty")
	}
	if route == "/" {
		return "index.html", nil
	}
	return strings.TrimPrefix(route, "/") + "/index.html", nil
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return ""
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	if len(route) > 1 {
		route = strings.TrimRight(route, "/")
	}
	return route
}

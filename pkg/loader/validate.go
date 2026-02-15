package loader

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
)

// ValidateSite validates cross-resource relationships required for safe parsing.
func ValidateSite(site *model.Site) error {
	if site == nil {
		return fmt.Errorf("site is nil")
	}
	if site.Website.Metadata.Name == "" {
		return fmt.Errorf("website metadata.name is required")
	}
	if len(site.Pages) == 0 {
		return fmt.Errorf("at least one page is required")
	}

	routes := make(map[string]string, len(site.Pages))
	for pageName, page := range site.Pages {
		route := NormalizeRoute(page.Spec.Route)
		if route == "" {
			return fmt.Errorf("page %q has empty route", pageName)
		}

		if existingPage, exists := routes[route]; exists {
			return fmt.Errorf("duplicate route %q in pages %q and %q", route, existingPage, pageName)
		}
		routes[route] = pageName

		page.Spec.Route = route
		site.Pages[pageName] = page

		for _, item := range page.Spec.Layout {
			include := strings.TrimSpace(item.Include)
			if include == "" {
				return fmt.Errorf("page %q has an empty include", pageName)
			}
			if _, exists := site.Components[include]; !exists {
				return fmt.Errorf("page %q references missing component %q", pageName, include)
			}
		}
	}

	return nil
}

// NormalizeRoute normalizes routes to a deterministic representation.
func NormalizeRoute(route string) string {
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

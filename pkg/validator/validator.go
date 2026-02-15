package validator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
)

const (
	SeverityError = "error"
)

type ValidationError struct {
	Component string
	Rule      string
	Severity  string
	Message   string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] component %q (%s): %s", e.Severity, e.Component, e.Rule, e.Message)
}

// ValidateComponent validates a single component with the default rule set.
func ValidateComponent(component *model.Component) []ValidationError {
	return ValidateComponentWithConfig(component, DefaultConfig())
}

// ValidateComponentWithConfig validates a single component with configurable rules.
func ValidateComponentWithConfig(component *model.Component, cfg Config) []ValidationError {
	if component == nil {
		return []ValidationError{{
			Component: "",
			Rule:      "component-present",
			Severity:  SeverityError,
			Message:   "component is nil",
		}}
	}

	allowlist := normalizedAllowlist(cfg.AllowedRootTags)
	result := validateParsedFragment(*component, allowlist, cfg.RequireAnchorID, cfg.ExpectedAnchorID)
	return result
}

// ValidateAllComponents validates all site components and returns all diagnostics.
func ValidateAllComponents(site *model.Site) []ValidationError {
	if site == nil {
		return []ValidationError{{
			Component: "",
			Rule:      "site-present",
			Severity:  SeverityError,
			Message:   "site is nil",
		}}
	}

	used := usedComponents(site)
	names := make([]string, 0, len(site.Components))
	for name := range site.Components {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []ValidationError
	for _, name := range names {
		component := site.Components[name]
		cfg := DefaultConfig()
		cfg.RequireAnchorID = used[name]
		cfg.ExpectedAnchorID = name
		out = append(out, ValidateComponentWithConfig(&component, cfg)...)
	}

	return out
}

func FormatErrors(errs []ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(errs))
	for _, err := range errs {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n")
}

func usedComponents(site *model.Site) map[string]bool {
	used := make(map[string]bool)
	pageNames := make([]string, 0, len(site.Pages))
	for pageName := range site.Pages {
		pageNames = append(pageNames, pageName)
	}
	sort.Strings(pageNames)

	for _, pageName := range pageNames {
		page := site.Pages[pageName]
		for _, item := range page.Spec.Layout {
			include := strings.TrimSpace(item.Include)
			if include != "" {
				used[include] = true
			}
		}
	}
	return used
}

func normalizedAllowlist(tags []string) map[string]struct{} {
	if len(tags) == 0 {
		tags = DefaultConfig().AllowedRootTags
	}
	out := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			out[tag] = struct{}{}
		}
	}
	if len(out) == 0 {
		for _, tag := range DefaultConfig().AllowedRootTags {
			out[tag] = struct{}{}
		}
	}
	return out
}

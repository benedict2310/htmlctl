package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/client"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

type websiteInspectReport struct {
	Website        string                 `json:"website" yaml:"website"`
	Environment    string                 `json:"environment" yaml:"environment"`
	Site           client.WebsiteResource `json:"site" yaml:"site"`
	ResourceCounts client.ResourceCounts  `json:"resourceCounts" yaml:"resourceCounts"`
	Warnings       []string               `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type pageInspectReport struct {
	Website              string              `json:"website" yaml:"website"`
	Environment          string              `json:"environment" yaml:"environment"`
	Page                 client.PageResource `json:"page" yaml:"page"`
	ReferencedComponents []string            `json:"referencedComponents" yaml:"referencedComponents"`
	MissingComponents    []string            `json:"missingComponents,omitempty" yaml:"missingComponents,omitempty"`
}

type componentInspectReport struct {
	Website          string                   `json:"website" yaml:"website"`
	Environment      string                   `json:"environment" yaml:"environment"`
	Component        client.ComponentResource `json:"component" yaml:"component"`
	ReferencingPages []string                 `json:"referencingPages" yaml:"referencingPages"`
}

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect remote site resources in detail",
	}

	cmd.AddCommand(newInspectWebsiteCmd())
	cmd.AddCommand(newInspectPageCmd())
	cmd.AddCommand(newInspectComponentCmd())
	return cmd
}

func newInspectWebsiteCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "website",
		Short: "Inspect website-level metadata and resource summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			resources, err := loadResourcesForContext(cmd)
			if err != nil {
				return err
			}
			report := websiteInspectReport{
				Website:        resources.Website,
				Environment:    resources.Environment,
				Site:           resources.Site,
				ResourceCounts: resources.ResourceCounts,
			}
			if resources.Site.SEO != nil && resources.Site.SEO.StructuredData != nil && resources.Site.SEO.StructuredData.Enabled && strings.TrimSpace(resources.Site.SEO.PublicBaseURL) == "" {
				report.Warnings = append(report.Warnings, "structuredData is enabled but publicBaseURL is empty")
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, report)
			}
			if err := writeWebsiteTable(cmd, resources.Site, resources.ResourceCounts); err != nil {
				return err
			}
			if len(report.Warnings) > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
				for _, warning := range report.Warnings {
					fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning)
				}
			}
			return nil
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newInspectPageCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "page <name>",
		Short: "Inspect one page including route, layout, and head metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resources, err := loadResourcesForContext(cmd)
			if err != nil {
				return err
			}
			page, ok := findPage(resources.Pages, args[0])
			if !ok {
				return fmt.Errorf("page %q not found. Next: run 'htmlctl get pages --context %s' to list available pages.", args[0], activeContextName(cmd))
			}
			componentNames := make([]string, 0, len(page.Layout))
			missing := []string{}
			componentSet := make(map[string]bool, len(resources.Components))
			for _, component := range resources.Components {
				componentSet[component.Name] = true
			}
			for _, item := range page.Layout {
				componentNames = append(componentNames, item.Include)
				if !componentSet[item.Include] {
					missing = append(missing, item.Include)
				}
			}
			report := pageInspectReport{
				Website:              resources.Website,
				Environment:          resources.Environment,
				Page:                 page,
				ReferencedComponents: componentNames,
				MissingComponents:    missing,
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, report)
			}
			rows := [][]string{
				{"name", page.Name},
				{"route", page.Route},
				{"title", page.Title},
				{"description", page.Description},
				{"components", strings.Join(componentNames, ", ")},
				{"canonical_url", pageCanonicalURL(page)},
				{"og_title", pageOpenGraphTitle(page)},
				{"twitter_card", pageTwitterCard(page)},
				{"content_hash", page.ContentHash},
			}
			if err := output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows); err != nil {
				return err
			}
			if len(missing) > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: missing components referenced by layout: %s\n", strings.Join(missing, ", "))
			}
			return nil
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newInspectComponentCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "component <name>",
		Short: "Inspect one component including sidecars and referencing pages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resources, err := loadResourcesForContext(cmd)
			if err != nil {
				return err
			}
			component, ok := findComponent(resources.Components, args[0])
			if !ok {
				return fmt.Errorf("component %q not found. Next: run 'htmlctl get components --context %s' to list available components.", args[0], activeContextName(cmd))
			}
			referencingPages := referencingPages(resources.Pages, component.Name)
			report := componentInspectReport{
				Website:          resources.Website,
				Environment:      resources.Environment,
				Component:        component,
				ReferencingPages: referencingPages,
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, report)
			}
			rows := [][]string{
				{"name", component.Name},
				{"scope", component.Scope},
				{"has_css", boolWord(component.HasCSS)},
				{"has_js", boolWord(component.HasJS)},
				{"referencing_pages", strings.Join(referencingPages, ", ")},
				{"content_hash", component.ContentHash},
				{"css_hash", stringOrNone(component.CSSHash)},
				{"js_hash", stringOrNone(component.JSHash)},
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func loadResourcesForContext(cmd *cobra.Command) (client.ResourcesResponse, error) {
	rt, api, err := runtimeAndClientFromCommand(cmd)
	if err != nil {
		return client.ResourcesResponse{}, err
	}
	website, err := requireContextWebsite(rt)
	if err != nil {
		return client.ResourcesResponse{}, err
	}
	environment, err := requireContextEnvironment(rt)
	if err != nil {
		return client.ResourcesResponse{}, err
	}
	return api.GetResources(cmd.Context(), website, environment)
}

func findPage(pages []client.PageResource, name string) (client.PageResource, bool) {
	for _, page := range pages {
		if page.Name == strings.TrimSpace(name) {
			return page, true
		}
	}
	return client.PageResource{}, false
}

func findComponent(components []client.ComponentResource, name string) (client.ComponentResource, bool) {
	for _, component := range components {
		if component.Name == strings.TrimSpace(name) {
			return component, true
		}
	}
	return client.ComponentResource{}, false
}

func referencingPages(pages []client.PageResource, componentName string) []string {
	out := []string{}
	for _, page := range pages {
		for _, item := range page.Layout {
			if item.Include == componentName {
				out = append(out, page.Name)
				break
			}
		}
	}
	return out
}

func pageCanonicalURL(page client.PageResource) string {
	if page.Head == nil {
		return "<none>"
	}
	return stringOrNone(page.Head.CanonicalURL)
}

func pageOpenGraphTitle(page client.PageResource) string {
	if page.Head == nil || page.Head.OpenGraph == nil {
		return "<none>"
	}
	return stringOrNone(page.Head.OpenGraph.Title)
}

func pageTwitterCard(page client.PageResource) string {
	if page.Head == nil || page.Head.Twitter == nil {
		return "<none>"
	}
	return stringOrNone(page.Head.Twitter.Card)
}

func activeContextName(cmd *cobra.Command) string {
	rt := runtimeFromCommandIfExists(cmd)
	if rt == nil {
		return "<context>"
	}
	return rt.ResolvedContext.Name
}

package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/client"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var outputMode string

	cmd := &cobra.Command{
		Use:   "get <resource-type>",
		Short: "List remote resources",
		Long:  "List remote resources. Use `get` for inventory and `inspect` for deeper page/component/website details.\n\nSupported resource types: websites, website, environments, releases, pages, components, styles, assets, branding, domains, backends.",
		Example: "  htmlctl get websites\n" +
			"  htmlctl get pages --context staging\n" +
			"  htmlctl get assets --output json\n" +
			"  htmlctl inspect page index --context staging",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			resourceType, err := normalizeResourceType(args[0])
			if err != nil {
				return err
			}

			switch resourceType {
			case "websites":
				resp, err := api.ListWebsites(cmd.Context())
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resp)
				}
				if len(resp.Websites) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No websites found.")
					return nil
				}
				rows := make([][]string, 0, len(resp.Websites))
				for _, w := range resp.Websites {
					rows = append(rows, []string{
						w.Name,
						w.DefaultStyleBundle,
						w.BaseTemplate,
						w.UpdatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"NAME", "DEFAULT_STYLE", "BASE_TEMPLATE", "UPDATED_AT"}, rows)

			case "website":
				resources, err := loadResourcesForContext(cmd)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resources.Site)
				}
				return writeWebsiteTable(cmd, resources.Site, resources.ResourceCounts)

			case "environments":
				website, err := requireContextWebsite(rt)
				if err != nil {
					return err
				}
				resp, err := api.ListEnvironments(cmd.Context(), website)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resp)
				}
				if len(resp.Environments) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No environments found.")
					return nil
				}
				rows := make([][]string, 0, len(resp.Environments))
				for _, env := range resp.Environments {
					rows = append(rows, []string{
						env.Name,
						output.OrNone(env.ActiveReleaseID),
						env.UpdatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"NAME", "ACTIVE_RELEASE", "UPDATED_AT"}, rows)

			case "releases":
				website, err := requireContextWebsite(rt)
				if err != nil {
					return err
				}
				environment, err := requireContextEnvironment(rt)
				if err != nil {
					return err
				}
				resp, err := api.ListReleases(cmd.Context(), website, environment)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resp)
				}
				if len(resp.Releases) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No releases found.")
					return nil
				}
				rows := make([][]string, 0, len(resp.Releases))
				for _, rel := range resp.Releases {
					active := "false"
					if rel.Active {
						active = "true"
					}
					rows = append(rows, []string{
						rel.ReleaseID,
						rel.Status,
						active,
						rel.CreatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"RELEASE_ID", "STATUS", "ACTIVE", "CREATED_AT"}, rows)

			case "pages":
				resources, err := loadResourcesForContext(cmd)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resources.Pages)
				}
				if len(resources.Pages) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No pages found.")
					return nil
				}
				rows := make([][]string, 0, len(resources.Pages))
				for _, page := range resources.Pages {
					rows = append(rows, []string{
						page.Name,
						page.Route,
						itoa(len(page.Layout)),
						page.UpdatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"NAME", "ROUTE", "LAYOUT_ITEMS", "UPDATED_AT"}, rows)

			case "components":
				resources, err := loadResourcesForContext(cmd)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resources.Components)
				}
				if len(resources.Components) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No components found.")
					return nil
				}
				rows := make([][]string, 0, len(resources.Components))
				for _, component := range resources.Components {
					rows = append(rows, []string{
						component.Name,
						component.Scope,
						boolWord(component.HasCSS),
						boolWord(component.HasJS),
						component.UpdatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"NAME", "SCOPE", "CSS", "JS", "UPDATED_AT"}, rows)

			case "styles":
				resources, err := loadResourcesForContext(cmd)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resources.Styles)
				}
				if len(resources.Styles) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No styles found.")
					return nil
				}
				rows := make([][]string, 0, len(resources.Styles))
				for _, style := range resources.Styles {
					rows = append(rows, []string{
						style.Name,
						itoa(len(style.Files)),
						style.UpdatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"NAME", "FILES", "UPDATED_AT"}, rows)

			case "assets":
				resources, err := loadResourcesForContext(cmd)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resources.Assets)
				}
				if len(resources.Assets) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No assets found.")
					return nil
				}
				rows := make([][]string, 0, len(resources.Assets))
				for _, asset := range resources.Assets {
					rows = append(rows, []string{
						asset.Path,
						asset.ContentType,
						itoa64(asset.SizeBytes),
						asset.ContentHash,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"PATH", "TYPE", "SIZE_BYTES", "HASH"}, rows)

			case "branding":
				resources, err := loadResourcesForContext(cmd)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resources.Branding)
				}
				if len(resources.Branding) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No branding assets found.")
					return nil
				}
				rows := make([][]string, 0, len(resources.Branding))
				for _, asset := range resources.Branding {
					rows = append(rows, []string{
						asset.Slot,
						asset.SourcePath,
						asset.ContentType,
						itoa64(asset.SizeBytes),
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"SLOT", "SOURCE_PATH", "TYPE", "SIZE_BYTES"}, rows)

			case "domains":
				website, err := requireContextWebsite(rt)
				if err != nil {
					return err
				}
				environment, err := requireContextEnvironment(rt)
				if err != nil {
					return err
				}
				resp, err := api.ListDomainBindings(cmd.Context(), website, environment)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resp)
				}
				if len(resp.Domains) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No domains found.")
					return nil
				}
				rows := make([][]string, 0, len(resp.Domains))
				for _, binding := range resp.Domains {
					rows = append(rows, []string{
						binding.Domain,
						binding.Website,
						binding.Environment,
						binding.CreatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"DOMAIN", "WEBSITE", "ENVIRONMENT", "CREATED"}, rows)

			case "backends":
				website, err := requireContextWebsite(rt)
				if err != nil {
					return err
				}
				environment, err := requireContextEnvironment(rt)
				if err != nil {
					return err
				}
				resp, err := api.ListBackends(cmd.Context(), website, environment)
				if err != nil {
					return err
				}
				if format != output.FormatTable {
					return output.WriteStructured(cmd.OutOrStdout(), format, resp)
				}
				if len(resp.Backends) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No backends found.")
					return nil
				}
				rows := make([][]string, 0, len(resp.Backends))
				for _, backend := range resp.Backends {
					rows = append(rows, []string{
						backend.PathPrefix,
						backend.Upstream,
						backend.CreatedAt,
					})
				}
				return output.WriteTable(cmd.OutOrStdout(), []string{"PATH PREFIX", "UPSTREAM", "CREATED"}, rows)

			}
			return fmt.Errorf("internal: unhandled get resource type %q", resourceType)
		},
	}

	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")

	return cmd
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func itoa64(v int64) string {
	return fmt.Sprintf("%d", v)
}

func writeWebsiteTable(cmd *cobra.Command, site client.WebsiteResource, counts client.ResourceCounts) error {
	seo := site.SEO
	head := site.Head
	rows := [][]string{
		{"name", site.Name},
		{"default_style_bundle", site.DefaultStyleBundle},
		{"base_template", site.BaseTemplate},
		{"content_hash", stringOrNone(site.ContentHash)},
		{"icons_configured", boolWord(head != nil && head.Icons != nil)},
		{"public_base_url", stringOrNone(optionalPublicBaseURL(site))},
		{"robots_enabled", boolWord(seo != nil && seo.Robots != nil && seo.Robots.Enabled)},
		{"sitemap_enabled", boolWord(seo != nil && seo.Sitemap != nil && seo.Sitemap.Enabled)},
		{"llms_txt_enabled", boolWord(seo != nil && seo.LLMsTxt != nil && seo.LLMsTxt.Enabled)},
		{"structured_data_enabled", boolWord(seo != nil && seo.StructuredData != nil && seo.StructuredData.Enabled)},
		{"pages", itoa(counts.Pages)},
		{"components", itoa(counts.Components)},
		{"styles", itoa(counts.Styles)},
		{"assets", itoa(counts.Assets)},
		{"scripts", itoa(counts.Scripts)},
	}
	return output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func optionalPublicBaseURL(site client.WebsiteResource) string {
	if site.SEO == nil {
		return ""
	}
	return strings.TrimSpace(site.SEO.PublicBaseURL)
}

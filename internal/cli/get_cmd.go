package cli

import (
	"fmt"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var outputMode string

	cmd := &cobra.Command{
		Use:   "get <resource-type>",
		Short: "List remote resources",
		Args:  cobra.ExactArgs(1),
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

package cli

import (
	"fmt"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newPreviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Manage expiring preview URLs for pinned releases",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	markRequiresTransport(cmd)
	cmd.AddCommand(newPreviewCreateCmd())
	cmd.AddCommand(newPreviewListCmd())
	cmd.AddCommand(newPreviewRemoveCmd())
	return cmd
}

func newPreviewCreateCmd() *cobra.Command {
	var envName string
	var releaseID string
	var ttl string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "create [website/<name>]",
		Short: "Create an expiring preview URL for a release",
		Long:  "Create an expiring preview URL for a release. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl preview create website/sample --env staging --release 01ARZ3NDEKTSV4RRFFQ69G5FAV --ttl 72h\n" +
			"  htmlctl preview create --release 01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			website, err := resolveRemoteWebsite(rt, args)
			if err != nil {
				return err
			}
			envName, err := resolveRemoteEnvironment(rt, envName)
			if err != nil {
				return err
			}

			resp, err := api.CreatePreview(cmd.Context(), website, envName, releaseID, ttl)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "preview %d created for release %s on %s/%s\n", resp.ID, resp.ReleaseID, website, envName)
			fmt.Fprintf(cmd.OutOrStdout(), "Hostname: %s\n", resp.Hostname)
			fmt.Fprintf(cmd.OutOrStdout(), "Expires:  %s\n", resp.ExpiresAt)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: verify the pinned release on %s before promotion.\n", resp.Hostname)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: run 'htmlctl preview list website/%s --env %s --context %s' to inspect active previews.\n", website, envName, rt.ResolvedContext.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().StringVar(&releaseID, "release", "", "Release ID to pin to the preview hostname")
	cmd.Flags().StringVar(&ttl, "ttl", "", "Preview TTL as a whole-hour duration (for example 72h)")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("release")
	return cmd
}

func newPreviewListCmd() *cobra.Command {
	var envName string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "list [website/<name>]",
		Short: "List preview URLs for an environment",
		Long:  "List preview URLs for an environment. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl preview list website/sample --env staging\n" +
			"  htmlctl preview list",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			website, err := resolveRemoteWebsite(rt, args)
			if err != nil {
				return err
			}
			envName, err := resolveRemoteEnvironment(rt, envName)
			if err != nil {
				return err
			}

			resp, err := api.ListPreviews(cmd.Context(), website, envName)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			if len(resp.Previews) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No previews configured.")
				return nil
			}
			rows := make([][]string, 0, len(resp.Previews))
			for _, preview := range resp.Previews {
				rows = append(rows, []string{
					fmt.Sprintf("%d", preview.ID),
					preview.ReleaseID,
					preview.Hostname,
					preview.ExpiresAt,
					preview.CreatedAt,
				})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"ID", "RELEASE", "HOSTNAME", "EXPIRES", "CREATED"}, rows)
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newPreviewRemoveCmd() *cobra.Command {
	var envName string
	var previewID int64
	var outputMode string
	cmd := &cobra.Command{
		Use:   "remove [website/<name>]",
		Short: "Remove a preview URL",
		Long:  "Remove a preview URL. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl preview remove website/sample --env staging --id 42\n" +
			"  htmlctl preview remove --id 42",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			website, err := resolveRemoteWebsite(rt, args)
			if err != nil {
				return err
			}
			envName, err := resolveRemoteEnvironment(rt, envName)
			if err != nil {
				return err
			}
			if err := api.RemovePreview(cmd.Context(), website, envName, previewID); err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, map[string]any{
					"id":          previewID,
					"website":     website,
					"environment": envName,
					"removed":     true,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "preview %d removed from %s/%s\n", previewID, website, envName)
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().Int64Var(&previewID, "id", 0, "Preview ID")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

package cli

import (
	"fmt"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newBackendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backend",
		Short: "Manage environment backends",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	markRequiresTransport(cmd)
	cmd.AddCommand(newBackendAddCmd())
	cmd.AddCommand(newBackendListCmd())
	cmd.AddCommand(newBackendRemoveCmd())
	return cmd
}

func newBackendAddCmd() *cobra.Command {
	var envName string
	var pathPrefix string
	var upstream string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "add <website-ref>",
		Short: "Add or update a backend for an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			website, err := parseWebsiteRef(args[0])
			if err != nil {
				return err
			}

			resp, err := api.AddBackend(cmd.Context(), website, envName, pathPrefix, upstream)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backend %s -> %s added to %s/%s\n", resp.PathPrefix, resp.Upstream, website, envName)
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "Backend path prefix (for example /api/*)")
	cmd.Flags().StringVar(&upstream, "upstream", "", "Backend upstream URL")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("path")
	_ = cmd.MarkFlagRequired("upstream")
	return cmd
}

func newBackendListCmd() *cobra.Command {
	var envName string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "list <website-ref>",
		Short: "List backends for an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			website, err := parseWebsiteRef(args[0])
			if err != nil {
				return err
			}

			resp, err := api.ListBackends(cmd.Context(), website, envName)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			if len(resp.Backends) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No backends configured.")
				return nil
			}
			rows := make([][]string, 0, len(resp.Backends))
			for _, backend := range resp.Backends {
				rows = append(rows, []string{backend.PathPrefix, backend.Upstream, backend.CreatedAt})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"PATH PREFIX", "UPSTREAM", "CREATED"}, rows)
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("env")
	return cmd
}

func newBackendRemoveCmd() *cobra.Command {
	var envName string
	var pathPrefix string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "remove <website-ref>",
		Short: "Remove a backend from an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			website, err := parseWebsiteRef(args[0])
			if err != nil {
				return err
			}
			if err := api.RemoveBackend(cmd.Context(), website, envName, pathPrefix); err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, map[string]any{
					"pathPrefix":  pathPrefix,
					"website":     website,
					"environment": envName,
					"removed":     true,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backend %s removed from %s/%s\n", pathPrefix, website, envName)
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "Backend path prefix")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("path")
	return cmd
}

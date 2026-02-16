package cli

import (
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var outputMode string

	cmd := &cobra.Command{
		Use:   "status website/<name>",
		Short: "Show environment status for a website",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			website, err := parseWebsiteRef(args[0])
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}

			status, err := api.GetStatus(cmd.Context(), website, rt.ResolvedContext.Environment)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, status)
			}

			rows := [][]string{
				{"website", status.Website},
				{"environment", status.Environment},
				{"active_release", output.OrNone(status.ActiveReleaseID)},
				{"release_timestamp", output.OrNone(status.ActiveReleaseTimestamp)},
				{"pages", itoa(status.ResourceCounts.Pages)},
				{"components", itoa(status.ResourceCounts.Components)},
				{"styles", itoa(status.ResourceCounts.Styles)},
				{"assets", itoa(status.ResourceCounts.Assets)},
				{"scripts", itoa(status.ResourceCounts.Scripts)},
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

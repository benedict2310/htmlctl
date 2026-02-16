package cli

import (
	"fmt"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var outputMode string
	var limit int

	cmd := &cobra.Command{
		Use:   "logs website/<name>",
		Short: "Show audit log entries for a website",
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

			resp, err := api.GetLogs(cmd.Context(), website, rt.ResolvedContext.Environment, limit)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				payload := map[string]any{
					"website":     website,
					"environment": rt.ResolvedContext.Environment,
					"total":       resp.Total,
					"entries":     resp.Entries,
				}
				return output.WriteStructured(cmd.OutOrStdout(), format, payload)
			}
			if len(resp.Entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No audit log entries found.")
				return nil
			}

			rows := make([][]string, 0, len(resp.Entries))
			for _, entry := range resp.Entries {
				rows = append(rows, []string{
					entry.Timestamp,
					entry.Operation,
					entry.Actor,
					output.OrNone(entry.ReleaseID),
					output.Truncate(entry.ResourceSummary, 72),
				})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"TIMESTAMP", "ACTION", "ACTOR", "RELEASE", "SUMMARY"}, rows)
		},
	}

	markRequiresTransport(cmd)
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of log entries")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

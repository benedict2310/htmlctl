package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newRolloutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollout",
		Short: "Inspect and manage release rollout state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	markRequiresTransport(cmd)
	cmd.AddCommand(newRolloutHistoryCmd())
	cmd.AddCommand(newRolloutUndoCmd())
	return cmd
}

func newRolloutHistoryCmd() *cobra.Command {
	var outputMode string
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "history website/<name>",
		Short: "Show release history for a website environment",
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
			if limit < 0 {
				return fmt.Errorf("limit must be >= 0")
			}
			if offset < 0 {
				return fmt.Errorf("offset must be >= 0")
			}

			resp, err := api.ListReleasesPage(cmd.Context(), website, rt.ResolvedContext.Environment, limit, offset)
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
				actor := strings.TrimSpace(rel.Actor)
				if actor == "" {
					actor = "<unknown>"
				}
				rows = append(rows, []string{
					rel.ReleaseID,
					rel.CreatedAt,
					actor,
					rel.Status,
				})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"ID", "TIMESTAMP", "ACTOR", "STATUS"}, rows)
		},
	}

	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of releases to list")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of releases to skip before listing")
	return cmd
}

func newRolloutUndoCmd() *cobra.Command {
	var outputMode string

	cmd := &cobra.Command{
		Use:   "undo website/<name>",
		Short: "Roll back to the previous active release",
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

			resp, err := api.Rollback(cmd.Context(), website, rt.ResolvedContext.Environment)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Rolled back %s/%s: %s -> %s\n",
				resp.Website,
				resp.Environment,
				resp.FromReleaseID,
				resp.ToReleaseID,
			)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

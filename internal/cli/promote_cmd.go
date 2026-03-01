package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newPromoteCmd() *cobra.Command {
	var fromEnv string
	var toEnv string
	var outputMode string

	cmd := &cobra.Command{
		Use:   "promote website/<name> --from <source-env> --to <target-env>",
		Short: "Promote an active release artifact from one environment to another",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, api, err := runtimeAndClientFromCommand(cmd)
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

			from := strings.TrimSpace(fromEnv)
			to := strings.TrimSpace(toEnv)
			if from == "" || to == "" {
				return fmt.Errorf("both --from and --to are required")
			}
			if from == to {
				return fmt.Errorf("--from and --to must be different")
			}

			resp, err := api.Promote(cmd.Context(), website, from, to)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Promoted %s: %s -> %s, release %s, source %s, files %d, hash %s (%s)\n",
				resp.Website,
				resp.FromEnvironment,
				resp.ToEnvironment,
				resp.ReleaseID,
				resp.SourceReleaseID,
				resp.FileCount,
				resp.Hash,
				resp.Strategy,
			)
			for _, warning := range resp.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning)
			}
			return nil
		},
	}

	markRequiresTransport(cmd)
	cmd.Flags().StringVar(&fromEnv, "from", "", "Source environment name")
	cmd.Flags().StringVar(&toEnv, "to", "", "Target environment name")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

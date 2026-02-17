package cli

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/bundle"
	"github.com/benedict2310/htmlctl/internal/client"
	diffpkg "github.com/benedict2310/htmlctl/internal/diff"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

type applyDryRunResponse struct {
	Website     string         `json:"website" yaml:"website"`
	Environment string         `json:"environment" yaml:"environment"`
	DryRun      bool           `json:"dryRun" yaml:"dryRun"`
	Message     string         `json:"message" yaml:"message"`
	Result      diffpkg.Result `json:"result" yaml:"result"`
}

func newApplyCmd() *cobra.Command {
	var from string
	var outputMode string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply -f <site-dir>",
		Short: "Apply local site resources to a remote environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(from) == "" {
				fmt.Fprint(cmd.ErrOrStderr(), cmd.UsageString())
				return fmt.Errorf("required flag(s) \"from\" not set")
			}

			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}

			if dryRun {
				report, err := computeDesiredStateDiff(cmd.Context(), api, rt.ResolvedContext.Website, rt.ResolvedContext.Environment, from)
				if err != nil {
					return err
				}
				if format == output.FormatTable {
					if err := diffpkg.WriteTable(cmd.OutOrStdout(), report.Result, diffpkg.DisplayOptions{
						Color: diffpkg.AutoColor(cmd.OutOrStdout()),
					}); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), "Dry run: no changes applied")
					return nil
				}
				return output.WriteStructured(cmd.OutOrStdout(), format, applyDryRunResponse{
					Website:     report.Website,
					Environment: report.Environment,
					DryRun:      true,
					Message:     "Dry run: no changes applied",
					Result:      report.Result,
				})
			}

			if format == output.FormatTable {
				fmt.Fprintln(cmd.OutOrStdout(), "Bundling...")
			}
			archive, _, err := bundle.BuildTarFromDir(from, rt.ResolvedContext.Website)
			if err != nil {
				return fmt.Errorf("local validation failed: %w", err)
			}

			if format == output.FormatTable {
				fmt.Fprintln(cmd.OutOrStdout(), "Uploading...")
				fmt.Fprintln(cmd.OutOrStdout(), "Validating...")
			}
			uploadResp, err := api.ApplyBundle(
				cmd.Context(),
				rt.ResolvedContext.Website,
				rt.ResolvedContext.Environment,
				bytes.NewReader(archive),
				false,
			)
			if err != nil {
				return err
			}

			if format == output.FormatTable {
				fmt.Fprintln(cmd.OutOrStdout(), "Rendering...")
				fmt.Fprintln(cmd.OutOrStdout(), "Activating...")
			}
			release, err := api.CreateRelease(cmd.Context(), rt.ResolvedContext.Website, rt.ResolvedContext.Environment)
			if err != nil {
				return err
			}
			releaseResp := &release

			out := client.ApplyCommandResponse{
				Website:     rt.ResolvedContext.Website,
				Environment: rt.ResolvedContext.Environment,
				DryRun:      false,
				Upload:      uploadResp,
				Release:     releaseResp,
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, out)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Done. Release %s active.\n", releaseResp.ReleaseID)
			if releaseResp.PreviousReleaseID == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "First deploy for %s/%s complete.\n", rt.ResolvedContext.Website, rt.ResolvedContext.Environment)
				fmt.Fprintf(cmd.OutOrStdout(), "Next: run 'htmlctl domain add <domain> --context %s' to publish it.\n", rt.ResolvedContext.Name)
			}
			return nil
		},
	}

	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&from, "from", "f", "", "Source site directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show diff only; do not upload or create release")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

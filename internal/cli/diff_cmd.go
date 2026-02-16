package cli

import (
	"errors"
	"fmt"
	"strings"

	diffpkg "github.com/benedict2310/htmlctl/internal/diff"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

var errDiffHasChanges = errors.New("diff detected changes")

func newDiffCmd() *cobra.Command {
	var from string
	var outputMode string

	cmd := &cobra.Command{
		Use:   "diff -f <site-dir>",
		Short: "Show file-level desired-state diff against a remote environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(from) == "" {
				fmt.Fprint(cmd.ErrOrStderr(), cmd.UsageString())
				return exitCodeError(2, fmt.Errorf("required flag(s) \"from\" not set"))
			}

			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return exitCodeError(2, err)
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return exitCodeError(2, err)
			}

			report, err := computeDesiredStateDiff(cmd.Context(), api, rt.ResolvedContext.Website, rt.ResolvedContext.Environment, from)
			if err != nil {
				return exitCodeError(2, err)
			}

			if format == output.FormatTable {
				if err := diffpkg.WriteTable(cmd.OutOrStdout(), report.Result, diffpkg.DisplayOptions{
					Color: diffpkg.AutoColor(cmd.OutOrStdout()),
				}); err != nil {
					return exitCodeError(2, err)
				}
			} else {
				if err := output.WriteStructured(cmd.OutOrStdout(), format, report); err != nil {
					return exitCodeError(2, err)
				}
			}

			if report.Result.HasChanges() {
				return exitCodeError(1, errDiffHasChanges)
			}
			return nil
		},
	}

	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&from, "from", "f", "", "Source site directory")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

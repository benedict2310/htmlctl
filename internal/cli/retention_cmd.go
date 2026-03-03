package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

func newRetentionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retention",
		Short: "Run release retention and optional blob garbage collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	markRequiresTransport(cmd)
	cmd.AddCommand(newRetentionRunCmd())
	return cmd
}

func newRetentionRunCmd() *cobra.Command {
	var envName string
	var keep int
	var dryRun bool
	var blobGC bool
	var outputMode string
	cmd := &cobra.Command{
		Use:   "run [website/<name>]",
		Short: "Run retention for one environment",
		Long:  "Run retention for one environment. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl retention run website/sample --env staging --keep 20 --dry-run\n" +
			"  htmlctl retention run --keep 10 --blob-gc",
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
			envName, err = resolveRemoteEnvironment(rt, envName)
			if err != nil {
				return err
			}

			resp, err := api.RunRetention(cmd.Context(), website, envName, keep, dryRun, blobGC)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "retention run complete for %s/%s\n", website, envName)
			fmt.Fprintf(cmd.OutOrStdout(), "Keep:      %d\n", resp.Keep)
			fmt.Fprintf(cmd.OutOrStdout(), "Dry run:   %t\n", resp.DryRun)
			fmt.Fprintf(cmd.OutOrStdout(), "Blob GC:   %t\n", resp.BlobGC)
			if resp.ActiveReleaseID != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Active:    %s\n", *resp.ActiveReleaseID)
			}
			if resp.RollbackReleaseID != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Rollback:  %s\n", *resp.RollbackReleaseID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Retained:  %d\n", len(resp.RetainedReleaseIDs))
			fmt.Fprintf(cmd.OutOrStdout(), "Prunable:  %d\n", len(resp.PrunableReleaseIDs))
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned:    %d\n", len(resp.PrunedReleaseIDs))
			if len(resp.PreviewPinnedReleaseIDs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Preview pins: %s\n", strings.Join(resp.PreviewPinnedReleaseIDs, ", "))
			}
			if len(resp.PrunableReleaseIDs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Prunable IDs: %s\n", strings.Join(resp.PrunableReleaseIDs, ", "))
			}
			if resp.BlobGC {
				fmt.Fprintf(cmd.OutOrStdout(), "Marked blobs:     %d\n", resp.MarkedBlobCount)
				fmt.Fprintf(cmd.OutOrStdout(), "Blob candidates: %d\n", len(resp.BlobDeleteCandidates))
				fmt.Fprintf(cmd.OutOrStdout(), "Blob deleted:    %d\n", len(resp.DeletedBlobHashes))
			}
			if resp.DryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run only; no data was deleted.")
			}
			for _, warning := range resp.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().IntVar(&keep, "keep", 0, "Number of newest releases to keep before applying safety pins")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what retention would delete without deleting it")
	cmd.Flags().BoolVar(&blobGC, "blob-gc", false, "Delete orphaned hash-named blobs after release pruning")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("keep")
	return cmd
}

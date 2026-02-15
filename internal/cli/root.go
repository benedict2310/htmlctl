package cli

import "github.com/spf13/cobra"

// NewRootCmd builds the htmlctl root command tree.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "htmlctl",
		Short: "CLI control plane for static HTML websites",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRenderCmd())
	cmd.AddCommand(newServeCmd())
	cmd.AddCommand(newVersionCmd(version))

	return cmd
}

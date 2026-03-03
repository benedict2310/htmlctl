package cli

import (
	"fmt"

	"github.com/benedict2310/htmlctl/internal/client"
	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/benedict2310/htmlctl/internal/transport"
	"github.com/spf13/cobra"
)

func newVersionCmd(version string) *cobra.Command {
	if version == "" {
		version = "dev"
	}

	var remote bool
	var outputMode string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print htmlctl version",
		Long:  "Print the local htmlctl version. Pass --remote to also query the selected htmlservd server version.",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			if !remote {
				if format == output.FormatTable {
					fmt.Fprintln(cmd.OutOrStdout(), version)
					return nil
				}
				return output.WriteStructured(cmd.OutOrStdout(), format, map[string]string{"localVersion": version})
			}

			contextOverride, err := cmd.Flags().GetString("context")
			if err != nil {
				return err
			}
			cfg, _, err := config.Load("")
			if err != nil {
				return err
			}
			resolved, err := config.ResolveContext(cfg, contextOverride)
			if err != nil {
				return err
			}
			tr, err := buildTransportForContext(cmd.Context(), resolved, transport.SSHConfig{})
			if err != nil {
				return err
			}
			defer tr.Close()

			api := client.NewWithAuth(tr, resolved.Name, resolved.Token)
			remoteVersion, err := api.GetVersion(cmd.Context())
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, map[string]string{
					"localVersion":  version,
					"remoteVersion": remoteVersion.Version,
				})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, [][]string{
				{"local_version", version},
				{"remote_version", remoteVersion.Version},
			})
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "Also query the selected remote htmlservd version")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

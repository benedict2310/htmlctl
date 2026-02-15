package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/renderer"
	"github.com/benedict2310/htmlctl/pkg/validator"
	"github.com/spf13/cobra"
)

func newRenderCmd() *cobra.Command {
	var from string
	var output string

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render a site directory into static output",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(from) == "" {
				fmt.Fprint(cmd.ErrOrStderr(), cmd.UsageString())
				return fmt.Errorf("required flag(s) \"from\" not set")
			}

			site, err := loader.LoadSite(from)
			if err != nil {
				return err
			}

			errs := validator.ValidateAllComponents(site)
			if len(errs) > 0 {
				return fmt.Errorf("component validation failed:\n%s", validator.FormatErrors(errs))
			}

			if err := renderer.Render(site, output); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rendered %d page(s) to %s\n", len(site.Pages), output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&from, "from", "f", "", "Source site directory")
	cmd.Flags().StringVarP(&output, "output", "o", "./dist", "Output directory")

	return cmd
}

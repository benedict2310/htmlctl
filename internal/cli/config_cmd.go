package cli

import (
	"fmt"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect htmlctl config and active context",
		Long:  "Inspect htmlctl config and active context. Use the 'context' command group to create, update, list, and switch contexts.",
	}
	markRequiresConfig(cmd)

	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigCurrentContextCmd())
	cmd.AddCommand(newConfigUseContextCmd())

	return cmd
}

func newConfigViewCmd() *cobra.Command {
	var showSecrets bool

	cmd := &cobra.Command{
		Use:   "view",
		Short: "Print the loaded config with tokens redacted by default",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}
			cfg := rt.Config
			if !showSecrets {
				cfg = cfg.RedactedCopy()
			}
			out, err := yaml.Marshal(&cfg)
			if err != nil {
				return fmt.Errorf("marshal config output: %w", err)
			}
			if len(out) == 0 || out[len(out)-1] != '\n' {
				out = append(out, '\n')
			}
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	}
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Print secret values such as context tokens")
	return cmd
}

func newConfigCurrentContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Print the active context name",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}
			ctx, err := config.ResolveContext(rt.Config, rt.ContextOverride)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ctx.Name)
			return nil
		},
	}
}

func newConfigUseContextCmd() *cobra.Command {
	cmd := newContextUseCmd()
	cmd.Use = "use-context <name>"
	cmd.Short = "Switch current-context (preferred: htmlctl context use <name>)"
	return cmd
}

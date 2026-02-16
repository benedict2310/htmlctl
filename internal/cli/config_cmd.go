package cli

import (
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage htmlctl contexts",
	}
	markRequiresConfig(cmd)

	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigCurrentContextCmd())
	cmd.AddCommand(newConfigUseContextCmd())

	return cmd
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print the loaded config",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(&rt.Config)
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
	return &cobra.Command{
		Use:   "use-context <name>",
		Short: "Switch current-context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}

			ctx, err := config.ResolveContext(rt.Config, args[0])
			if err != nil {
				return err
			}

			rt.Config.CurrentContext = strings.TrimSpace(ctx.Name)
			if err := config.Save(rt.ConfigPath, rt.Config); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q\n", ctx.Name)
			return nil
		},
	}
}

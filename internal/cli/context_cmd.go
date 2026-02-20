package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/spf13/cobra"
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage context values",
	}

	cmd.AddCommand(newContextSetCmd())
	cmd.AddCommand(newContextTokenCmd())
	return cmd
}

func newContextSetCmd() *cobra.Command {
	var server string
	var website string
	var environment string
	var token string
	var port int

	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Update a context entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}

			name := strings.TrimSpace(args[0])
			if _, err := config.ResolveContext(rt.Config, name); err != nil {
				return err
			}
			index := -1
			for i := range rt.Config.Contexts {
				if strings.TrimSpace(rt.Config.Contexts[i].Name) == name {
					index = i
					break
				}
			}
			if index < 0 {
				return fmt.Errorf("context %q not found", name)
			}

			changed := false
			if cmd.Flags().Changed("server") {
				rt.Config.Contexts[index].Server = strings.TrimSpace(server)
				changed = true
			}
			if cmd.Flags().Changed("website") {
				rt.Config.Contexts[index].Website = strings.TrimSpace(website)
				changed = true
			}
			if cmd.Flags().Changed("environment") {
				rt.Config.Contexts[index].Environment = strings.TrimSpace(environment)
				changed = true
			}
			if cmd.Flags().Changed("port") {
				rt.Config.Contexts[index].Port = port
				changed = true
			}
			if cmd.Flags().Changed("token") {
				rt.Config.Contexts[index].Token = strings.TrimSpace(token)
				changed = true
			}
			if !changed {
				return fmt.Errorf("at least one context field must be set")
			}

			if err := config.Save(rt.ConfigPath, rt.Config); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated context %q\n", name)
			return nil
		},
	}
	markRequiresConfig(cmd)
	cmd.Flags().StringVar(&server, "server", "", "SSH server URL (ssh://user@host)")
	cmd.Flags().StringVar(&website, "website", "", "Website name")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")
	cmd.Flags().IntVar(&port, "port", 0, "Remote htmlservd port (0 uses default)")
	cmd.Flags().StringVar(&token, "token", "", "API bearer token")
	return cmd
}

func newContextTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Context token utilities",
	}
	cmd.AddCommand(newContextTokenGenerateCmd())
	return cmd
}

func newContextTokenGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate a random API token (use with: htmlctl context set <name> --token <value>)",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := generateTokenHex(32)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), token)
			return nil
		},
	}
}

func generateTokenHex(size int) (string, error) {
	if size <= 0 {
		return "", fmt.Errorf("token size must be greater than zero")
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

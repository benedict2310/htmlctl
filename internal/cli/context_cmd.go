package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/names"
	"github.com/benedict2310/htmlctl/internal/transport"
	"github.com/spf13/cobra"
)

var contextNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]*$`)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Create, update, list, and switch contexts",
	}

	cmd.AddCommand(newContextListCmd())
	cmd.AddCommand(newContextUseCmd())
	cmd.AddCommand(newContextCreateCmd())
	cmd.AddCommand(newContextSetCmd())
	cmd.AddCommand(newContextTokenCmd())
	return cmd
}

func newContextListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}

			if len(rt.Config.Contexts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No contexts configured.")
				fmt.Fprintln(cmd.OutOrStdout(), "Next: run 'htmlctl context create <name> --server <ssh://user@host> --website <website> --environment <env>'.")
				return nil
			}

			for _, ctx := range rt.Config.Contexts {
				marker := " "
				if strings.TrimSpace(rt.Config.CurrentContext) == strings.TrimSpace(ctx.Name) {
					marker = "*"
				}

				port := "default"
				if ctx.Port > 0 {
					port = strconv.Itoa(ctx.Port)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %q\tserver=%s\twebsite=%s\tenvironment=%s\tport=%s\n", marker, strings.TrimSpace(ctx.Name), config.RedactServerURL(ctx.Server), strings.TrimSpace(ctx.Website), strings.TrimSpace(ctx.Environment), port)
			}
			return nil
		},
	}
	markRequiresConfig(cmd)
	return cmd
}

func newContextUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name>",
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
	markRequiresConfig(cmd)
	return cmd
}

func newContextCreateCmd() *cobra.Command {
	var server string
	var website string
	var environment string
	var token string
	var port int

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new context entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("context name is required")
			}
			if err := validateContextName(name); err != nil {
				return err
			}

			cfg, path, err := loadConfigForContextCreate()
			if err != nil {
				return err
			}
			for _, ctx := range cfg.Contexts {
				if strings.TrimSpace(ctx.Name) == name {
					return fmt.Errorf("context %q already exists. Next: run 'htmlctl context use <name>' or 'htmlctl context set <name> --server <ssh://user@host> --website <website> --environment <env>'.", name)
				}
			}

			server = strings.TrimSpace(server)
			if _, err := transport.ParseServerURL(server); err != nil {
				return fmt.Errorf("context %q: %w", name, err)
			}
			website = strings.TrimSpace(website)
			if err := names.ValidateResourceName(website); err != nil {
				return fmt.Errorf("context %q: invalid website name %q: %w", name, website, err)
			}
			environment = strings.TrimSpace(environment)
			if err := names.ValidateResourceName(environment); err != nil {
				return fmt.Errorf("context %q: invalid environment name %q: %w", name, environment, err)
			}
			token = strings.TrimSpace(token)
			selectCreatedContext := shouldSelectCreatedContext(cfg, name)

			cfg.Contexts = append(cfg.Contexts, config.Context{
				Name:        name,
				Server:      server,
				Website:     website,
				Environment: environment,
				Port:        port,
				Token:       token,
			})
			if selectCreatedContext {
				cfg.CurrentContext = name
			}

			if err := config.Save(path, cfg); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created context %q\n", name)
			if cfg.CurrentContext == name {
				fmt.Fprintf(cmd.OutOrStdout(), "Current context is now %q\n", name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "SSH server URL (ssh://user@host)")
	cmd.Flags().StringVar(&website, "website", "", "Website name")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")
	cmd.Flags().IntVar(&port, "port", 0, "Remote htmlservd port (0 uses default)")
	cmd.Flags().StringVar(&token, "token", "", "API bearer token")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("website")
	_ = cmd.MarkFlagRequired("environment")
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
				server = strings.TrimSpace(server)
				if _, err := transport.ParseServerURL(server); err != nil {
					return fmt.Errorf("context %q: %w", name, err)
				}
				rt.Config.Contexts[index].Server = server
				changed = true
			}
			if cmd.Flags().Changed("website") {
				website = strings.TrimSpace(website)
				if err := names.ValidateResourceName(website); err != nil {
					return fmt.Errorf("context %q: invalid website name %q: %w", name, website, err)
				}
				rt.Config.Contexts[index].Website = website
				changed = true
			}
			if cmd.Flags().Changed("environment") {
				environment = strings.TrimSpace(environment)
				if err := names.ValidateResourceName(environment); err != nil {
					return fmt.Errorf("context %q: invalid environment name %q: %w", name, environment, err)
				}
				rt.Config.Contexts[index].Environment = environment
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

func loadConfigForContextCreate() (config.Config, string, error) {
	path, err := config.ResolvePath("")
	if err != nil {
		return config.Config{}, "", err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.Config{}, path, nil
		}
		return config.Config{}, "", fmt.Errorf("stat config file %s: %w", path, err)
	}

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}

func shouldSelectCreatedContext(cfg config.Config, createdName string) bool {
	current := strings.TrimSpace(cfg.CurrentContext)
	if current == "" {
		return true
	}
	for _, ctx := range cfg.Contexts {
		if strings.TrimSpace(ctx.Name) == current {
			return false
		}
	}
	return true
}

func validateContextName(name string) error {
	if name == "" {
		return fmt.Errorf("context name is required")
	}
	if !contextNamePattern.MatchString(name) {
		return fmt.Errorf("invalid context name %q: must match %q", name, contextNamePattern.String())
	}
	return nil
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

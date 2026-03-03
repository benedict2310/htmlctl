package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/transport"
	"github.com/spf13/cobra"
)

const requiresConfigAnnotation = "htmlctl.dev/requires-config"
const requiresTransportAnnotation = "htmlctl.dev/requires-transport"

var buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
	return transport.NewSSHTransportFromContext(ctx, info, cfg)
}

type rootOptions struct {
	contextOverride string
}

type runtimeContextKey struct{}

type commandRuntime struct {
	ContextOverride string
	ResolvedContext config.ContextInfo
	Config          config.Config
	ConfigPath      string
	Transport       transport.Transport
}

// NewRootCmd builds the htmlctl root command tree.
func NewRootCmd(version string) *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "htmlctl",
		Short: "CLI control plane for static HTML websites",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			needsConfig := commandRequiresConfig(cmd) || commandRequiresTransport(cmd)
			if !needsConfig {
				return nil
			}

			cfg, path, err := config.Load("")
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rt := &commandRuntime{
				ContextOverride: strings.TrimSpace(opts.contextOverride),
				Config:          cfg,
				ConfigPath:      path,
			}
			if commandRequiresTransport(cmd) {
				resolved, err := config.ResolveContext(cfg, rt.ContextOverride)
				if err != nil {
					return err
				}
				if err := validateTransportContextRequirements(cmd, args, resolved); err != nil {
					return err
				}
				tr, err := buildTransportForContext(ctx, resolved, transport.SSHConfig{})
				if err != nil {
					return err
				}
				rt.ResolvedContext = resolved
				rt.Transport = tr
			}

			ctx = context.WithValue(ctx, runtimeContextKey{}, rt)
			cmd.SetContext(ctx)
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			rt := runtimeFromCommandIfExists(cmd)
			if rt == nil || rt.Transport == nil {
				return nil
			}
			return rt.Transport.Close()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.contextOverride, "context", "", "Context name from htmlctl config")

	cmd.AddCommand(newContextCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newRenderCmd())
	cmd.AddCommand(newServeCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newApplyCmd())
	cmd.AddCommand(newRolloutCmd())
	cmd.AddCommand(newPromoteCmd())
	cmd.AddCommand(newDomainCmd())
	cmd.AddCommand(newBackendCmd())
	cmd.AddCommand(newAuthPolicyCmd())
	cmd.AddCommand(newPreviewCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newVersionCmd(version))
	cmd.AddCommand(newDoctorCmd(version))

	return cmd
}

func markRequiresConfig(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[requiresConfigAnnotation] = "true"
}

func markRequiresTransport(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[requiresTransportAnnotation] = "true"
}

func commandRequiresConfig(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations != nil && c.Annotations[requiresConfigAnnotation] == "true" {
			return true
		}
	}
	return false
}

func commandRequiresTransport(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations != nil && c.Annotations[requiresTransportAnnotation] == "true" {
			return true
		}
	}
	return false
}

func runtimeFromCommand(cmd *cobra.Command) (*commandRuntime, error) {
	rt := runtimeFromCommandIfExists(cmd)
	if rt == nil {
		return nil, fmt.Errorf("internal: command runtime is not initialized")
	}
	return rt, nil
}

func runtimeFromCommandIfExists(cmd *cobra.Command) *commandRuntime {
	if cmd == nil || cmd.Context() == nil {
		return nil
	}
	rt, ok := cmd.Context().Value(runtimeContextKey{}).(*commandRuntime)
	if !ok {
		return nil
	}
	return rt
}

func runtimeTransportFromCommand(cmd *cobra.Command) (transport.Transport, error) {
	rt, err := runtimeFromCommand(cmd)
	if err != nil {
		return nil, err
	}
	if rt.Transport == nil {
		return nil, fmt.Errorf("internal: transport is not initialized")
	}
	return rt.Transport, nil
}

func validateTransportContextRequirements(cmd *cobra.Command, args []string, resolved config.ContextInfo) error {
	switch cmd.CommandPath() {
	case "htmlctl apply", "htmlctl diff", "htmlctl domain add":
		if _, err := requireContextWebsiteValue(resolved.Website); err != nil {
			return err
		}
		if _, err := requireContextEnvironmentValue(resolved.Environment); err != nil {
			return err
		}
	case "htmlctl domain list":
		if _, err := requireContextWebsiteValue(resolved.Website); err != nil {
			return err
		}
	case "htmlctl get":
		if len(args) == 0 {
			return nil
		}
		resourceType, err := normalizeResourceType(args[0])
		if err != nil {
			return err
		}
		switch resourceType {
		case "environments":
			if _, err := requireContextWebsiteValue(resolved.Website); err != nil {
				return err
			}
		case "releases":
			if _, err := requireContextWebsiteValue(resolved.Website); err != nil {
				return err
			}
			if _, err := requireContextEnvironmentValue(resolved.Environment); err != nil {
				return err
			}
		}
	case "htmlctl status", "htmlctl logs", "htmlctl rollout history", "htmlctl rollout undo":
		if _, err := resolveRemoteWebsiteValue(args, resolved.Website); err != nil {
			return err
		}
		if _, err := requireContextEnvironmentValue(resolved.Environment); err != nil {
			return err
		}
	case "htmlctl backend add", "htmlctl backend list", "htmlctl backend remove":
		if _, err := resolveRemoteWebsiteValue(args, resolved.Website); err != nil {
			return err
		}
		if strings.TrimSpace(cmd.Flag("env").Value.String()) == "" {
			if _, err := requireContextEnvironmentValue(resolved.Environment); err != nil {
				return fmt.Errorf("no environment selected: pass --env <environment> or run 'htmlctl context set <name> --environment <environment>'")
			}
		}
	case "htmlctl authpolicy add", "htmlctl authpolicy list", "htmlctl authpolicy remove":
		if _, err := resolveRemoteWebsiteValue(args, resolved.Website); err != nil {
			return err
		}
		if strings.TrimSpace(cmd.Flag("env").Value.String()) == "" {
			if _, err := requireContextEnvironmentValue(resolved.Environment); err != nil {
				return fmt.Errorf("no environment selected: pass --env <environment> or run 'htmlctl context set <name> --environment <environment>'")
			}
		}
	case "htmlctl preview create", "htmlctl preview list", "htmlctl preview remove":
		if _, err := resolveRemoteWebsiteValue(args, resolved.Website); err != nil {
			return err
		}
		if strings.TrimSpace(cmd.Flag("env").Value.String()) == "" {
			if _, err := requireContextEnvironmentValue(resolved.Environment); err != nil {
				return fmt.Errorf("no environment selected: pass --env <environment> or run 'htmlctl context set <name> --environment <environment>'")
			}
		}
	}
	return nil
}

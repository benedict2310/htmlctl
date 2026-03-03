package cli

import (
	"fmt"
	"io"
	"strings"

	authpolicypkg "github.com/benedict2310/htmlctl/internal/authpolicy"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

func newAuthPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "authpolicy",
		Short: "Manage environment auth policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	markRequiresTransport(cmd)
	cmd.AddCommand(newAuthPolicyAddCmd())
	cmd.AddCommand(newAuthPolicyListCmd())
	cmd.AddCommand(newAuthPolicyRemoveCmd())
	return cmd
}

func newAuthPolicyAddCmd() *cobra.Command {
	var envName string
	var pathPrefix string
	var username string
	var passwordStdin bool
	var outputMode string
	cmd := &cobra.Command{
		Use:   "add [website/<name>]",
		Short: "Add or update a Basic Auth policy for an environment path prefix",
		Long:  "Add or update a Basic Auth policy for an environment path prefix. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl authpolicy add website/sample --env staging --path /docs/* --username reviewer --password-stdin\n" +
			"  printf 'secret\\n' | htmlctl authpolicy add --path /docs/* --username reviewer --password-stdin",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !passwordStdin {
				return fmt.Errorf("--password-stdin is required")
			}
			password, err := readPasswordFromStdin(cmd.InOrStdin())
			if err != nil {
				return err
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), authpolicypkg.MinBcryptCost)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}

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

			resp, err := api.AddAuthPolicy(cmd.Context(), website, envName, pathPrefix, username, string(hash))
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "auth policy %s added to %s/%s for %s\n", resp.PathPrefix, website, envName, resp.Username)
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "Protected path prefix (for example /docs/*)")
	cmd.Flags().StringVar(&username, "username", "", "Basic Auth username")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the password from stdin and hash it locally with bcrypt")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("path")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

func newAuthPolicyListCmd() *cobra.Command {
	var envName string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "list [website/<name>]",
		Short: "List auth policies for an environment",
		Long:  "List auth policies for an environment. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl authpolicy list website/sample --env staging\n" +
			"  htmlctl authpolicy list",
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

			resp, err := api.ListAuthPolicies(cmd.Context(), website, envName)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			if len(resp.AuthPolicies) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No auth policies configured.")
				return nil
			}
			rows := make([][]string, 0, len(resp.AuthPolicies))
			for _, policy := range resp.AuthPolicies {
				rows = append(rows, []string{policy.PathPrefix, policy.Username, policy.CreatedAt})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"PATH PREFIX", "USERNAME", "CREATED"}, rows)
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newAuthPolicyRemoveCmd() *cobra.Command {
	var envName string
	var pathPrefix string
	var outputMode string
	cmd := &cobra.Command{
		Use:   "remove [website/<name>]",
		Short: "Remove an auth policy from an environment",
		Long:  "Remove an auth policy from an environment. Omit website/<name> to use the active context website. Omit --env to use the active context environment.",
		Example: "  htmlctl authpolicy remove website/sample --env staging --path /docs/*\n" +
			"  htmlctl authpolicy remove --path /docs/*",
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
			if err := api.RemoveAuthPolicy(cmd.Context(), website, envName, pathPrefix); err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, map[string]any{
					"pathPrefix":  pathPrefix,
					"website":     website,
					"environment": envName,
					"removed":     true,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "auth policy %s removed from %s/%s\n", pathPrefix, website, envName)
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "Environment name (defaults to context environment)")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "Protected path prefix")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	_ = cmd.MarkFlagRequired("path")
	return cmd
}

func readPasswordFromStdin(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	password := strings.TrimRight(string(b), "\r\n")
	if password == "" {
		return "", fmt.Errorf("password from stdin is empty")
	}
	return password, nil
}

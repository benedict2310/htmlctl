package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/domain"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

var lookupDomainHost = func(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

type tlsVerifyResult struct {
	Issuer    string `json:"issuer" yaml:"issuer"`
	ExpiresAt string `json:"expiresAt" yaml:"expiresAt"`
}

var verifyDomainTLS = func(ctx context.Context, host string) (tlsVerifyResult, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, "443"), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return tlsVerifyResult{}, err
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return tlsVerifyResult{}, fmt.Errorf("no peer certificate received")
	}
	cert := state.PeerCertificates[0]
	issuer := strings.TrimSpace(cert.Issuer.CommonName)
	if issuer == "" {
		issuer = cert.Issuer.String()
	}
	return tlsVerifyResult{
		Issuer:    issuer,
		ExpiresAt: cert.NotAfter.UTC().Format(time.RFC3339),
	}, nil
}

type domainVerifyResponse struct {
	Domain      string `json:"domain" yaml:"domain"`
	Context     string `json:"context" yaml:"context"`
	Website     string `json:"website" yaml:"website"`
	Environment string `json:"environment" yaml:"environment"`
	DNS         struct {
		Pass      bool     `json:"pass" yaml:"pass"`
		Addresses []string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
		Error     string   `json:"error,omitempty" yaml:"error,omitempty"`
	} `json:"dns" yaml:"dns"`
	TLS struct {
		Pass      bool   `json:"pass" yaml:"pass"`
		Skipped   bool   `json:"skipped,omitempty" yaml:"skipped,omitempty"`
		Issuer    string `json:"issuer,omitempty" yaml:"issuer,omitempty"`
		ExpiresAt string `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
		Error     string `json:"error,omitempty" yaml:"error,omitempty"`
	} `json:"tls" yaml:"tls"`
}

func newDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Manage custom domain bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	markRequiresConfig(cmd)
	cmd.AddCommand(newDomainAddCmd())
	cmd.AddCommand(newDomainListCmd())
	cmd.AddCommand(newDomainRemoveCmd())
	cmd.AddCommand(newDomainVerifyCmd())
	return cmd
}

func newDomainAddCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "add <domain>",
		Short: "Bind a domain to the current context environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			domainName, err := domain.Normalize(args[0])
			if err != nil {
				return err
			}

			resp, err := api.CreateDomainBinding(cmd.Context(), domainName, rt.ResolvedContext.Website, rt.ResolvedContext.Environment)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Domain binding created:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Domain:      %s\n", resp.Domain)
			fmt.Fprintf(cmd.OutOrStdout(), "  Website:     %s\n", resp.Website)
			fmt.Fprintf(cmd.OutOrStdout(), "  Environment: %s\n\n", resp.Environment)
			fmt.Fprintf(cmd.OutOrStdout(), "Caddy configuration updated and reloaded.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Next: run 'htmlctl domain verify %s --context %s' to check DNS and TLS.\n", resp.Domain, rt.ResolvedContext.Name)
			return nil
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newDomainListCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List domains for the current context website",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}

			resp, err := api.ListDomainBindings(cmd.Context(), rt.ResolvedContext.Website, "")
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, resp)
			}
			if len(resp.Domains) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No domains configured.")
				return nil
			}
			rows := make([][]string, 0, len(resp.Domains))
			for _, binding := range resp.Domains {
				rows = append(rows, []string{
					binding.Domain,
					binding.Environment,
					binding.CreatedAt,
				})
			}
			return output.WriteTable(cmd.OutOrStdout(), []string{"DOMAIN", "ENVIRONMENT", "CREATED"}, rows)
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newDomainRemoveCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "remove <domain>",
		Short: "Remove a domain binding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			domainName, err := domain.Normalize(args[0])
			if err != nil {
				return err
			}
			if err := api.DeleteDomainBinding(cmd.Context(), domainName); err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, map[string]any{
					"domain":  domainName,
					"removed": true,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Domain binding removed: %s\n", domainName)
			fmt.Fprintln(cmd.OutOrStdout(), "Caddy configuration updated and reloaded.")
			return nil
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newDomainVerifyCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "verify <domain>",
		Short: "Verify DNS and TLS for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeFromCommand(cmd)
			if err != nil {
				return err
			}
			ctxInfo, err := config.ResolveContext(rt.Config, rt.ContextOverride)
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			domainName, err := domain.Normalize(args[0])
			if err != nil {
				return err
			}

			result := domainVerifyResponse{
				Domain:      domainName,
				Context:     ctxInfo.Name,
				Website:     ctxInfo.Website,
				Environment: ctxInfo.Environment,
			}

			dnsCtx, cancelDNS := context.WithTimeout(cmd.Context(), 5*time.Second)
			addresses, dnsErr := lookupDomainHost(dnsCtx, domainName)
			cancelDNS()
			if dnsErr != nil {
				result.DNS.Error = dnsErr.Error()
			} else {
				result.DNS.Pass = true
				result.DNS.Addresses = addresses
			}

			if result.DNS.Pass {
				tlsCtx, cancelTLS := context.WithTimeout(cmd.Context(), 10*time.Second)
				tlsInfo, tlsErr := verifyDomainTLS(tlsCtx, domainName)
				cancelTLS()
				if tlsErr != nil {
					result.TLS.Error = tlsErr.Error()
				} else {
					result.TLS.Pass = true
					result.TLS.Issuer = tlsInfo.Issuer
					result.TLS.ExpiresAt = tlsInfo.ExpiresAt
				}
			} else {
				result.TLS.Skipped = true
				result.TLS.Error = "cannot check TLS without DNS resolution"
			}

			if format != output.FormatTable {
				if err := output.WriteStructured(cmd.OutOrStdout(), format, result); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Verifying %s for context %s...\n\n", domainName, ctxInfo.Name)
				if result.DNS.Pass {
					fmt.Fprintf(cmd.OutOrStdout(), "DNS Resolution:    PASS  (resolves to %s)\n", strings.Join(result.DNS.Addresses, ", "))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "DNS Resolution:    FAIL  (%s)\n", result.DNS.Error)
					fmt.Fprintln(cmd.OutOrStdout(), "  -> Add an A/AAAA record for the domain pointing to your server.")
				}
				switch {
				case result.TLS.Pass:
					fmt.Fprintf(cmd.OutOrStdout(), "TLS Certificate:   PASS  (valid, issued by %s, expires %s)\n", result.TLS.Issuer, result.TLS.ExpiresAt)
				case result.TLS.Skipped:
					fmt.Fprintf(cmd.OutOrStdout(), "TLS Certificate:   SKIP  (%s)\n", result.TLS.Error)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "TLS Certificate:   FAIL  (%s)\n", result.TLS.Error)
					fmt.Fprintln(cmd.OutOrStdout(), "  -> Ensure Caddy is serving the domain and certificate issuance has completed.")
				}
			}

			if !result.DNS.Pass || !result.TLS.Pass {
				return fmt.Errorf("domain verification failed")
			}
			return nil
		},
	}
	markRequiresConfig(cmd)
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

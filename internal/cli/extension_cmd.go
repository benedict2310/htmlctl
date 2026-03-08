package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benedict2310/htmlctl/internal/client"
	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/extensionspec"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/benedict2310/htmlctl/internal/transport"
	"github.com/spf13/cobra"
)

type extensionValidateCheck struct {
	Target string `json:"target" yaml:"target"`
	OK     bool   `json:"ok" yaml:"ok"`
	Detail string `json:"detail" yaml:"detail"`
}

type extensionValidateReport struct {
	ManifestPath  string                   `json:"manifestPath" yaml:"manifestPath"`
	Extension     string                   `json:"extension" yaml:"extension"`
	ExtensionVer  string                   `json:"extensionVersion" yaml:"extensionVersion"`
	LocalVersion  string                   `json:"localVersion" yaml:"localVersion"`
	MinHTMLCTL    string                   `json:"minHTMLCTL" yaml:"minHTMLCTL"`
	RemoteVersion string                   `json:"remoteVersion,omitempty" yaml:"remoteVersion,omitempty"`
	MinHTMLSERVD  string                   `json:"minHTMLSERVD" yaml:"minHTMLSERVD"`
	Context       string                   `json:"context,omitempty" yaml:"context,omitempty"`
	Checks        []extensionValidateCheck `json:"checks" yaml:"checks"`
}

func newExtensionCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Validate and inspect htmlctl extensions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newExtensionValidateCmd(version))
	return cmd
}

func newExtensionValidateCmd(localVersion string) *cobra.Command {
	if strings.TrimSpace(localVersion) == "" {
		localVersion = "dev"
	}

	var remote bool
	var outputMode string

	cmd := &cobra.Command{
		Use:   "validate <extension-dir-or-manifest>",
		Short: "Validate an extension manifest and compatibility requirements",
		Long:  "Validate an extension manifest, enforce its schema rules, and check local htmlctl compatibility. Pass --remote to also verify the selected htmlservd version against minHTMLSERVD.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}

			manifestPath, err := resolveExtensionManifestPath(args[0])
			if err != nil {
				return err
			}
			manifest, err := extensionspec.LoadManifest(manifestPath)
			if err != nil {
				return err
			}

			report := extensionValidateReport{
				ManifestPath: manifestPath,
				Extension:    manifest.Metadata.Name,
				ExtensionVer: manifest.Metadata.Version,
				LocalVersion: localVersion,
				MinHTMLCTL:   manifest.Spec.Compatibility.MinHTMLCTL,
				MinHTMLSERVD: manifest.Spec.Compatibility.MinHTMLSERVD,
			}

			failed := false
			if err := extensionspec.VersionSatisfiesMinimum(localVersion, manifest.Spec.Compatibility.MinHTMLCTL); err != nil {
				report.Checks = append(report.Checks, extensionValidateCheck{
					Target: "htmlctl",
					OK:     false,
					Detail: err.Error(),
				})
				failed = true
			} else {
				report.Checks = append(report.Checks, extensionValidateCheck{
					Target: "htmlctl",
					OK:     true,
					Detail: fmt.Sprintf("local htmlctl %s satisfies minHTMLCTL %s", localVersion, manifest.Spec.Compatibility.MinHTMLCTL),
				})
			}

			if remote {
				remoteVersion, contextName, err := loadRemoteExtensionVersion(cmd.Context(), cmd, manifest.Spec.Compatibility.MinHTMLSERVD)
				if err != nil {
					return err
				}
				report.Context = contextName
				report.RemoteVersion = remoteVersion
				if err := extensionspec.VersionSatisfiesMinimum(remoteVersion, manifest.Spec.Compatibility.MinHTMLSERVD); err != nil {
					report.Checks = append(report.Checks, extensionValidateCheck{
						Target: "htmlservd",
						OK:     false,
						Detail: err.Error(),
					})
					failed = true
				} else {
					report.Checks = append(report.Checks, extensionValidateCheck{
						Target: "htmlservd",
						OK:     true,
						Detail: fmt.Sprintf("remote htmlservd %s satisfies minHTMLSERVD %s", remoteVersion, manifest.Spec.Compatibility.MinHTMLSERVD),
					})
				}
			}

			if err := writeExtensionValidateReport(cmd, format, report); err != nil {
				return err
			}
			if failed {
				return exitCodeError(1, errors.New("extension validation failed"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "Also query the selected remote htmlservd version")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func resolveExtensionManifestPath(raw string) (string, error) {
	pathValue := strings.TrimSpace(raw)
	if pathValue == "" {
		return "", fmt.Errorf("extension path is required")
	}
	info, err := os.Stat(pathValue)
	if err != nil {
		return "", fmt.Errorf("stat extension path %q: %w", pathValue, err)
	}
	if info.IsDir() {
		pathValue = filepath.Join(pathValue, "extension.yaml")
	}
	absPath, err := filepath.Abs(pathValue)
	if err != nil {
		return "", fmt.Errorf("resolve extension manifest path %q: %w", pathValue, err)
	}
	return absPath, nil
}

func loadRemoteExtensionVersion(ctx context.Context, cmd *cobra.Command, minimum string) (string, string, error) {
	contextOverride, err := cmd.Flags().GetString("context")
	if err != nil {
		return "", "", err
	}
	cfg, _, err := config.Load("")
	if err != nil {
		return "", "", err
	}
	resolved, err := config.ResolveContext(cfg, contextOverride)
	if err != nil {
		return "", "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tr, err := buildTransportForContext(ctx, resolved, transport.SSHConfig{})
	if err != nil {
		return "", "", err
	}
	defer tr.Close()

	api := client.NewWithAuth(tr, resolved.Name, resolved.Token)
	remoteVersion, err := api.GetVersion(ctx)
	if err != nil {
		return "", "", fmt.Errorf("query remote htmlservd version for minHTMLSERVD %s: %w", minimum, err)
	}
	return remoteVersion.Version, resolved.Name, nil
}

func writeExtensionValidateReport(cmd *cobra.Command, format output.Format, report extensionValidateReport) error {
	if format != output.FormatTable {
		return output.WriteStructured(cmd.OutOrStdout(), format, report)
	}

	rows := [][]string{
		{"manifest_path", report.ManifestPath},
		{"extension", report.Extension},
		{"extension_version", report.ExtensionVer},
		{"local_version", report.LocalVersion},
		{"min_htmlctl", report.MinHTMLCTL},
		{"remote_version", stringOrNone(report.RemoteVersion)},
		{"min_htmlservd", report.MinHTMLSERVD},
		{"context", stringOrNone(report.Context)},
	}
	if err := output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout())

	checkRows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		status := "FAIL"
		if check.OK {
			status = "PASS"
		}
		checkRows = append(checkRows, []string{check.Target, status, check.Detail})
	}
	return output.WriteTable(cmd.OutOrStdout(), []string{"TARGET", "STATUS", "DETAIL"}, checkRows)
}

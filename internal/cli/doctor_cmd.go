package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/benedict2310/htmlctl/internal/client"
	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/benedict2310/htmlctl/internal/transport"
	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Layer  string `json:"layer" yaml:"layer"`
	OK     bool   `json:"ok" yaml:"ok"`
	Detail string `json:"detail" yaml:"detail"`
	Hint   string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

type doctorReport struct {
	Context       string        `json:"context,omitempty" yaml:"context,omitempty"`
	ConfigPath    string        `json:"configPath,omitempty" yaml:"configPath,omitempty"`
	Server        string        `json:"server,omitempty" yaml:"server,omitempty"`
	Website       string        `json:"website,omitempty" yaml:"website,omitempty"`
	Environment   string        `json:"environment,omitempty" yaml:"environment,omitempty"`
	LocalVersion  string        `json:"localVersion" yaml:"localVersion"`
	RemoteVersion string        `json:"remoteVersion,omitempty" yaml:"remoteVersion,omitempty"`
	Checks        []doctorCheck `json:"checks" yaml:"checks"`
	NextSteps     []string      `json:"nextSteps,omitempty" yaml:"nextSteps,omitempty"`
}

func newDoctorCmd(version string) *cobra.Command {
	if version == "" {
		version = "dev"
	}

	var outputMode string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run remote diagnostics for the selected context",
		Long:  "Run remote diagnostics for the selected context, covering config resolution, SSH transport, server health, readiness, and version awareness.",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}

			report, failed := runDoctorChecks(cmd, version)
			if err := writeDoctorReport(cmd, format, report); err != nil {
				return err
			}
			if failed {
				return exitCodeError(1, errors.New("doctor checks failed"))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func runDoctorChecks(cmd *cobra.Command, localVersion string) (doctorReport, bool) {
	report := doctorReport{
		LocalVersion: localVersion,
	}
	failed := false
	contextOverride, _ := cmd.Flags().GetString("context")

	cfg, path, err := config.Load("")
	report.ConfigPath = path
	if err != nil {
		addDoctorCheck(&report, "config", false, err.Error(), "Create or fix your config, then rerun 'htmlctl doctor'.")
		return report, true
	}

	resolved, err := config.ResolveContext(cfg, contextOverride)
	if err != nil {
		addDoctorCheck(&report, "context", false, err.Error(), "Select a valid context with 'htmlctl context use <name>' or pass --context.")
		return report, true
	}
	report.Context = resolved.Name
	report.Server = resolved.Server
	report.Website = resolved.Website
	report.Environment = resolved.Environment
	addDoctorCheck(&report, "config", true, fmt.Sprintf("loaded %s", path), "")
	addDoctorCheck(&report, "context", true, fmt.Sprintf("resolved context %q", resolved.Name), "")

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	tr, err := buildTransportForContext(ctx, resolved, transport.SSHConfig{})
	if err != nil {
		addDoctorCheck(&report, "transport", false, err.Error(), doctorHintForTransportError(err))
		return report, true
	}
	defer tr.Close()
	addDoctorCheck(&report, "transport", true, "ssh tunnel established", "")

	api := client.NewWithAuth(tr, resolved.Name, resolved.Token)

	health, err := api.GetHealth(ctx)
	if err != nil {
		addDoctorCheck(&report, "health", false, err.Error(), "Verify htmlservd is running and reachable through the SSH tunnel.")
		failed = true
	} else {
		addDoctorCheck(&report, "health", true, fmt.Sprintf("/healthz status=%s", strings.TrimSpace(health.Status)), "")
	}

	ready, err := api.GetReady(ctx)
	if err != nil {
		addDoctorCheck(&report, "readiness", false, err.Error(), "Wait for htmlservd to finish startup, then rerun 'htmlctl doctor'.")
		failed = true
	} else {
		addDoctorCheck(&report, "readiness", true, fmt.Sprintf("/readyz status=%s", strings.TrimSpace(ready.Status)), "")
	}

	websites, err := api.ListWebsites(ctx)
	if err != nil {
		addDoctorCheck(&report, "auth", false, err.Error(), doctorHintForAuthError(err))
		failed = true
	} else {
		addDoctorCheck(&report, "auth", true, fmt.Sprintf("authenticated api request succeeded (/api/v1/websites, websites=%d)", len(websites.Websites)), "")
	}

	remoteVersion, err := api.GetVersion(ctx)
	if err != nil {
		addDoctorCheck(&report, "version", false, err.Error(), "Check the remote server health and verify the tunnel target is an htmlservd instance.")
		failed = true
		return report, failed
	}
	report.RemoteVersion = remoteVersion.Version
	if strings.TrimSpace(remoteVersion.Version) != strings.TrimSpace(localVersion) {
		addDoctorCheck(&report, "version", false, fmt.Sprintf("local=%s remote=%s", localVersion, remoteVersion.Version), "Update htmlctl or htmlservd so the versions match before using newer CLI features.")
		failed = true
		return report, failed
	}
	addDoctorCheck(&report, "version", true, fmt.Sprintf("local=%s remote=%s", localVersion, remoteVersion.Version), "")
	return report, failed
}

func addDoctorCheck(report *doctorReport, layer string, ok bool, detail, hint string) {
	report.Checks = append(report.Checks, doctorCheck{
		Layer:  layer,
		OK:     ok,
		Detail: strings.TrimSpace(detail),
		Hint:   strings.TrimSpace(hint),
	})
	if !ok && strings.TrimSpace(hint) != "" {
		for _, existing := range report.NextSteps {
			if existing == hint {
				return
			}
		}
		report.NextSteps = append(report.NextSteps, hint)
	}
}

func doctorHintForTransportError(err error) string {
	switch {
	case errors.Is(err, transport.ErrSSHHostKey):
		return "Refresh known_hosts with ssh-keyscan or fix the host key mismatch, then rerun 'htmlctl doctor'."
	case errors.Is(err, transport.ErrSSHAuth), errors.Is(err, transport.ErrSSHAgentUnavailable):
		return "Load a valid SSH key into your agent or set HTMLCTL_SSH_KEY_PATH, then rerun 'htmlctl doctor'."
	case errors.Is(err, transport.ErrSSHUnreachable):
		return "Verify the SSH host, port, and network reachability for the selected context, then rerun 'htmlctl doctor'."
	default:
		return "Check the SSH server, key, and tunnel configuration for the selected context, then rerun 'htmlctl doctor'."
	}
}

func doctorHintForAuthError(err error) string {
	if client.IsUnauthorized(err) {
		return "Check the selected context token or rotate the htmlservd API token, then rerun 'htmlctl doctor'."
	}
	return "Verify the selected context token has access to the htmlservd API, then rerun 'htmlctl doctor'."
}

func writeDoctorReport(cmd *cobra.Command, format output.Format, report doctorReport) error {
	if format != output.FormatTable {
		return output.WriteStructured(cmd.OutOrStdout(), format, report)
	}

	targetRows := [][]string{
		{"context", stringOrNone(report.Context)},
		{"config_path", stringOrNone(report.ConfigPath)},
		{"server", stringOrNone(report.Server)},
		{"website", stringOrNone(report.Website)},
		{"environment", stringOrNone(report.Environment)},
		{"local_version", stringOrNone(report.LocalVersion)},
		{"remote_version", stringOrNone(report.RemoteVersion)},
	}
	if err := output.WriteTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, targetRows); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout())

	checkRows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		status := "FAIL"
		if check.OK {
			status = "PASS"
		}
		checkRows = append(checkRows, []string{check.Layer, status, check.Detail})
	}
	if err := output.WriteTable(cmd.OutOrStdout(), []string{"LAYER", "STATUS", "DETAIL"}, checkRows); err != nil {
		return err
	}

	if len(report.NextSteps) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		for _, step := range report.NextSteps {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", step)
		}
	}
	return nil
}

func stringOrNone(v string) string {
	if strings.TrimSpace(v) == "" {
		return "<none>"
	}
	return v
}

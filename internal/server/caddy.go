package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/caddy"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

type caddyReloader interface {
	Reload(ctx context.Context, reason string) error
}

func (s *Server) generateCaddyConfig(ctx context.Context) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database is not initialized")
	}
	q := dbpkg.NewQueries(s.db)
	rows, err := q.ListDomainBindings(ctx, "", "")
	if err != nil {
		return "", err
	}
	backendsByEnvironment := map[int64][]caddy.Backend{}
	authPoliciesByEnvironment := map[int64][]caddy.AuthPolicy{}
	backendRows, err := q.ListBackends(ctx)
	if err != nil {
		return "", err
	}
	for _, backendRow := range backendRows {
		backendsByEnvironment[backendRow.EnvironmentID] = append(backendsByEnvironment[backendRow.EnvironmentID], caddy.Backend{
			PathPrefix: backendRow.PathPrefix,
			Upstream:   backendRow.Upstream,
		})
	}
	authPolicyRows, err := q.ListAuthPolicies(ctx)
	if err != nil {
		return "", err
	}
	for _, row := range authPolicyRows {
		authPoliciesByEnvironment[row.EnvironmentID] = append(authPoliciesByEnvironment[row.EnvironmentID], caddy.AuthPolicy{
			PathPrefix:   row.PathPrefix,
			Username:     row.Username,
			PasswordHash: row.PasswordHash,
		})
	}
	previewRows := []dbpkg.ReleasePreviewResolvedRow{}
	if s.cfg.Preview.Enabled {
		previewRows, err = q.ListReleasePreviews(ctx, formatPreviewTimestamp(time.Now().UTC()))
		if err != nil {
			return "", err
		}
	}
	sites := make([]caddy.Site, 0, len(rows))
	for _, row := range rows {
		root := filepath.Join(s.dataPaths.WebsitesRoot, row.WebsiteName, "envs", row.EnvironmentName, "current")
		sites = append(sites, caddy.Site{
			Domain:       row.Domain,
			Root:         filepath.ToSlash(root),
			Backends:     append([]caddy.Backend(nil), backendsByEnvironment[row.EnvironmentID]...),
			AuthPolicies: append([]caddy.AuthPolicy(nil), authPoliciesByEnvironment[row.EnvironmentID]...),
		})
	}
	for _, row := range previewRows {
		root := filepath.Join(s.dataPaths.WebsitesRoot, row.WebsiteName, "envs", row.EnvironmentName, "releases", row.ReleaseID)
		sites = append(sites, caddy.Site{
			Domain:       row.Hostname,
			Root:         filepath.ToSlash(root),
			Backends:     append([]caddy.Backend(nil), backendsByEnvironment[row.EnvironmentID]...),
			AuthPolicies: append([]caddy.AuthPolicy(nil), authPoliciesByEnvironment[row.EnvironmentID]...),
			Headers: map[string]string{
				"X-Robots-Tag": "noindex, nofollow, noarchive",
			},
		})
	}
	if s.cfg.Preview.Enabled && strings.TrimSpace(s.previewBaseDomain()) != "" {
		sites = append(sites, caddy.Site{
			Domain:        "*." + s.previewBaseDomain(),
			RespondStatus: http.StatusNotFound,
		})
	}
	return caddy.GenerateConfigWithOptions(sites, caddy.ConfigOptions{
		DisableAutoHTTPS: !s.cfg.CaddyAutoHTTPS,
		TelemetryPort:    s.telemetryProxyPort(),
	})
}

func (s *Server) telemetryProxyPort() int {
	if !s.cfg.Telemetry.Enabled {
		return 0
	}
	if s.cfg.Port > 0 {
		return s.cfg.Port
	}
	listenAddr := strings.TrimSpace(s.Addr())
	if listenAddr == "" {
		return 0
	}
	_, portRaw, err := net.SplitHostPort(listenAddr)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("telemetry proxy port unresolved; set an explicit server port when telemetry is enabled")
		}
		return 0
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 || port > 65535 {
		if s.logger != nil {
			s.logger.Warn("telemetry proxy port unresolved; set an explicit server port when telemetry is enabled")
		}
		return 0
	}
	return port
}

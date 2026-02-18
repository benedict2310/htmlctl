package server

import (
	"context"
	"fmt"
	"path/filepath"

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
	sites := make([]caddy.Site, 0, len(rows))
	for _, row := range rows {
		root := filepath.Join(s.dataPaths.WebsitesRoot, row.WebsiteName, "envs", row.EnvironmentName, "current")
		sites = append(sites, caddy.Site{
			Domain: row.Domain,
			Root:   filepath.ToSlash(root),
		})
	}
	return caddy.GenerateConfigWithOptions(sites, caddy.ConfigOptions{
		DisableAutoHTTPS: !s.cfg.CaddyAutoHTTPS,
	})
}

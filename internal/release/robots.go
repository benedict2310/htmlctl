package release

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func GenerateRobotsText(robots *model.WebsiteRobots, sitemapURL string) string {
	if robots == nil || !robots.Enabled {
		return ""
	}

	groups := robots.Groups
	if len(groups) == 0 {
		groups = []model.RobotsGroup{{
			UserAgents: []string{"*"},
			Allow:      []string{"/"},
		}}
	}

	lines := make([]string, 0, 8)
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		for _, userAgent := range group.UserAgents {
			lines = append(lines, "User-agent: "+userAgent)
		}
		for _, allow := range group.Allow {
			lines = append(lines, "Allow: "+allow)
		}
		for _, disallow := range group.Disallow {
			lines = append(lines, "Disallow: "+disallow)
		}
	}
	if strings.TrimSpace(sitemapURL) != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "Sitemap: "+sitemapURL)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func (b *Builder) materializeRobotsFile(ctx context.Context, seo *model.WebsiteSEO, sitemapURL, releaseDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var robots *model.WebsiteRobots
	if seo != nil {
		robots = seo.Robots
	}
	content := GenerateRobotsText(robots, sitemapURL)
	if content == "" {
		return nil
	}
	if err := writeFile(filepath.Join(releaseDir, "robots.txt"), []byte(content)); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}
	return nil
}

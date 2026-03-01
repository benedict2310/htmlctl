package release

import (
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/pkg/model"
)

func TestGenerateRobotsText(t *testing.T) {
	tests := []struct {
		name       string
		robots     *model.WebsiteRobots
		sitemapURL string
		want       string
	}{
		{
			name: "disabled",
			robots: &model.WebsiteRobots{
				Enabled: false,
			},
			want: "",
		},
		{
			name: "default allow all",
			robots: &model.WebsiteRobots{
				Enabled: true,
			},
			want: "User-agent: *\nAllow: /\n",
		},
		{
			name: "ordered groups and sitemap",
			robots: &model.WebsiteRobots{
				Enabled: true,
				Groups: []model.RobotsGroup{
					{
						UserAgents: []string{"Googlebot", "Bingbot"},
						Allow:      []string{"/"},
						Disallow:   []string{"/preview/"},
					},
					{
						UserAgents: []string{"*"},
						Disallow:   []string{"/drafts/", "/private/"},
					},
				},
			},
			sitemapURL: "https://example.com/sitemap.xml",
			want: "" +
				"User-agent: Googlebot\n" +
				"User-agent: Bingbot\n" +
				"Allow: /\n" +
				"Disallow: /preview/\n\n" +
				"User-agent: *\n" +
				"Disallow: /drafts/\n" +
				"Disallow: /private/\n\n" +
				"Sitemap: https://example.com/sitemap.xml\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GenerateRobotsText(tc.robots, tc.sitemapURL)
			if got != tc.want {
				t.Fatalf("GenerateRobotsText() = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "\r") {
				t.Fatalf("expected LF-only output, got %q", got)
			}
		})
	}
}

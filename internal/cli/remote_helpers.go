package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/benedict2310/htmlctl/internal/client"
	"github.com/spf13/cobra"
)

func runtimeAndClientFromCommand(cmd *cobra.Command) (*commandRuntime, *client.APIClient, error) {
	rt, err := runtimeFromCommand(cmd)
	if err != nil {
		return nil, nil, err
	}
	if rt.Transport == nil {
		return nil, nil, fmt.Errorf("internal: transport is not initialized")
	}
	return rt, client.NewWithAuth(rt.Transport, rt.ResolvedContext.Name, rt.ResolvedContext.Token), nil
}

func parseWebsiteRef(v string) (string, error) {
	raw := strings.TrimSpace(v)
	if strings.HasPrefix(raw, "website/") {
		name := strings.TrimSpace(strings.TrimPrefix(raw, "website/"))
		if name == "" {
			return "", fmt.Errorf("website name is required (expected website/<name>)")
		}
		return name, nil
	}
	if strings.HasPrefix(raw, "websites/") {
		name := strings.TrimSpace(strings.TrimPrefix(raw, "websites/"))
		if name == "" {
			return "", fmt.Errorf("website name is required (expected website/<name>)")
		}
		return name, nil
	}
	return "", fmt.Errorf("invalid resource %q (expected website/<name>)", v)
}

func normalizeResourceType(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "website", "websites", "ws":
		return "websites", nil
	case "environment", "environments", "env", "envs":
		return "environments", nil
	case "release", "releases":
		return "releases", nil
	default:
		return "", fmt.Errorf("unsupported resource type %q (expected websites, environments, or releases)", v)
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

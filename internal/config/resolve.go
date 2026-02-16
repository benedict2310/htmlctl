package config

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveContext resolves either an explicit context name or current-context.
func ResolveContext(cfg Config, explicitName string) (ContextInfo, error) {
	name := strings.TrimSpace(explicitName)
	if name == "" {
		name = strings.TrimSpace(cfg.CurrentContext)
		if name == "" {
			return ContextInfo{}, fmt.Errorf("no context selected: set current-context or pass --context")
		}
	}

	for _, ctx := range cfg.Contexts {
		if strings.TrimSpace(ctx.Name) != name {
			continue
		}
		return ContextInfo{
			Name:        strings.TrimSpace(ctx.Name),
			Server:      strings.TrimSpace(ctx.Server),
			Website:     strings.TrimSpace(ctx.Website),
			Environment: strings.TrimSpace(ctx.Environment),
			RemotePort:  ctx.Port,
		}, nil
	}

	available := availableContextNames(cfg.Contexts)
	if len(available) == 0 {
		return ContextInfo{}, fmt.Errorf("context %q not found: config has no contexts", name)
	}
	return ContextInfo{}, fmt.Errorf("context %q not found; available contexts: %s", name, strings.Join(available, ", "))
}

func availableContextNames(contexts []Context) []string {
	names := make([]string, 0, len(contexts))
	for _, ctx := range contexts {
		name := strings.TrimSpace(ctx.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

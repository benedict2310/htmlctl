package diff

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

func Compute(local, remote []FileRecord) (Result, error) {
	localByPath, err := indexRecords(local)
	if err != nil {
		return Result{}, fmt.Errorf("index local files: %w", err)
	}
	remoteByPath, err := indexRecords(remote)
	if err != nil {
		return Result{}, fmt.Errorf("index remote files: %w", err)
	}

	paths := make([]string, 0, len(localByPath)+len(remoteByPath))
	seen := make(map[string]struct{}, len(localByPath)+len(remoteByPath))
	for p := range localByPath {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for p := range remoteByPath {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := Result{Changes: make([]FileChange, 0, len(paths))}
	for _, p := range paths {
		newHash, hasLocal := localByPath[p]
		oldHash, hasRemote := remoteByPath[p]

		switch {
		case hasLocal && !hasRemote:
			out.Summary.Added++
			out.Changes = append(out.Changes, FileChange{
				Path:         p,
				ResourceType: resourceTypeForPath(p),
				ChangeType:   ChangeAdded,
				NewHash:      newHash,
			})
		case !hasLocal && hasRemote:
			out.Summary.Removed++
			out.Changes = append(out.Changes, FileChange{
				Path:         p,
				ResourceType: resourceTypeForPath(p),
				ChangeType:   ChangeRemoved,
				OldHash:      oldHash,
			})
		case hasLocal && hasRemote && newHash != oldHash:
			out.Summary.Modified++
			out.Changes = append(out.Changes, FileChange{
				Path:         p,
				ResourceType: resourceTypeForPath(p),
				ChangeType:   ChangeModified,
				OldHash:      oldHash,
				NewHash:      newHash,
			})
		default:
			out.Summary.Unchanged++
		}
	}

	sort.Slice(out.Changes, func(i, j int) bool {
		a, b := out.Changes[i], out.Changes[j]
		if resourceOrder(a.ResourceType) != resourceOrder(b.ResourceType) {
			return resourceOrder(a.ResourceType) < resourceOrder(b.ResourceType)
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return changeOrder(a.ChangeType) < changeOrder(b.ChangeType)
	})

	return out, nil
}

func indexRecords(records []FileRecord) (map[string]string, error) {
	out := make(map[string]string, len(records))
	for _, rec := range records {
		p := normalizePath(rec.Path)
		if p == "" {
			return nil, fmt.Errorf("path is empty")
		}
		h := normalizeHash(rec.Hash)
		if h == "" {
			return nil, fmt.Errorf("hash for %q is empty", p)
		}
		if existing, ok := out[p]; ok {
			if existing != h {
				return nil, fmt.Errorf("path %q has conflicting hashes (%s vs %s)", p, existing, h)
			}
			continue
		}
		out[p] = h
	}
	return out, nil
}

func normalizePath(raw string) string {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if p == "" {
		return ""
	}
	clean := path.Clean(p)
	if clean == "." {
		return ""
	}
	return clean
}

func normalizeHash(raw string) string {
	h := strings.TrimSpace(strings.ToLower(raw))
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "sha256:") {
		return h
	}
	return "sha256:" + h
}

func resourceTypeForPath(filePath string) string {
	p := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/")))
	switch {
	case strings.HasPrefix(p, "pages/"):
		return "pages"
	case strings.HasPrefix(p, "components/"):
		return "components"
	case strings.HasPrefix(p, "styles/"):
		return "styles"
	case strings.HasPrefix(p, "scripts/"):
		return "scripts"
	case strings.HasSuffix(p, ".js"), strings.HasSuffix(p, ".mjs"):
		return "scripts"
	default:
		return "assets"
	}
}

func resourceOrder(resourceType string) int {
	switch strings.ToLower(strings.TrimSpace(resourceType)) {
	case "pages":
		return 0
	case "components":
		return 1
	case "styles":
		return 2
	case "scripts":
		return 3
	case "assets":
		return 4
	default:
		return 5
	}
}

func changeOrder(changeType ChangeType) int {
	switch changeType {
	case ChangeAdded:
		return 0
	case ChangeModified:
		return 1
	case ChangeRemoved:
		return 2
	default:
		return 3
	}
}

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/pkg/model"
)

const maxPromoteMetadataWarnings = 20

type promoteManifestSnapshot struct {
	Resources struct {
		Pages []promoteManifestPage `json:"pages"`
	} `json:"resources"`
}

type promoteManifestPage struct {
	Name string          `json:"name"`
	Head json.RawMessage `json:"head"`
}

func collectPromoteMetadataHostWarnings(ctx context.Context, db *sql.DB, website, sourceEnv, targetEnv, sourceReleaseID string) ([]string, error) {
	q := dbpkg.NewQueries(db)
	sourceReleaseRow, err := q.GetReleaseByID(ctx, sourceReleaseID)
	if err != nil {
		return nil, fmt.Errorf("load source release %q: %w", sourceReleaseID, err)
	}

	sourceBindings, err := q.ListDomainBindings(ctx, website, sourceEnv)
	if err != nil {
		return nil, fmt.Errorf("list source domain bindings: %w", err)
	}
	targetBindings, err := q.ListDomainBindings(ctx, website, targetEnv)
	if err != nil {
		return nil, fmt.Errorf("list target domain bindings: %w", err)
	}

	return promoteMetadataHostWarnings(sourceReleaseRow.ManifestJSON, sourceBindings, targetBindings, targetEnv)
}

func promoteMetadataHostWarnings(manifestJSON string, sourceBindings, targetBindings []dbpkg.DomainBindingResolvedRow, targetEnv string) ([]string, error) {
	manifest := promoteManifestSnapshot{}
	if strings.TrimSpace(manifestJSON) != "" {
		if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
			return nil, fmt.Errorf("parse release manifest json: %w", err)
		}
	}

	sourceHosts := bindingHostSet(sourceBindings)
	targetHosts := bindingHostSet(targetBindings)
	targetIsProd := strings.EqualFold(strings.TrimSpace(targetEnv), "prod")

	warnings := make([]string, 0)
	for _, page := range manifest.Resources.Pages {
		head, err := parsePromotePageHead(page.Name, page.Head)
		if err != nil {
			return nil, err
		}
		for _, field := range promoteMetadataURLFields(head) {
			host, ok := metadataURLHost(field.Value)
			if !ok {
				continue
			}
			if _, ok := targetHosts[host]; ok {
				continue
			}
			if _, ok := sourceHosts[host]; !ok && !(targetIsProd && looksLikeStagingHost(host)) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"page=%s field=%s host=%s does not match target environment %s domains",
				page.Name,
				field.Path,
				host,
				targetEnv,
			))
		}
	}

	if len(warnings) <= maxPromoteMetadataWarnings {
		return warnings, nil
	}
	omitted := len(warnings) - (maxPromoteMetadataWarnings - 1)
	capped := append([]string{}, warnings[:maxPromoteMetadataWarnings-1]...)
	capped = append(capped, fmt.Sprintf("additional metadata host warnings omitted: %d", omitted))
	return capped, nil
}

type metadataURLField struct {
	Path  string
	Value string
}

func promoteMetadataURLFields(head *model.PageHead) []metadataURLField {
	if head == nil {
		return nil
	}
	fields := make([]metadataURLField, 0, 5)
	if v := strings.TrimSpace(head.CanonicalURL); v != "" {
		fields = append(fields, metadataURLField{Path: "canonicalURL", Value: v})
	}
	if head.OpenGraph != nil {
		if v := strings.TrimSpace(head.OpenGraph.URL); v != "" {
			fields = append(fields, metadataURLField{Path: "openGraph.url", Value: v})
		}
		if v := strings.TrimSpace(head.OpenGraph.Image); v != "" {
			fields = append(fields, metadataURLField{Path: "openGraph.image", Value: v})
		}
	}
	if head.Twitter != nil {
		if v := strings.TrimSpace(head.Twitter.URL); v != "" {
			fields = append(fields, metadataURLField{Path: "twitter.url", Value: v})
		}
		if v := strings.TrimSpace(head.Twitter.Image); v != "" {
			fields = append(fields, metadataURLField{Path: "twitter.image", Value: v})
		}
	}
	return fields
}

func parsePromotePageHead(pageName string, raw json.RawMessage) (*model.PageHead, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	head := &model.PageHead{}
	if err := json.Unmarshal(raw, head); err != nil {
		return nil, fmt.Errorf("parse head metadata for page %q: %w", pageName, err)
	}
	return head, nil
}

func metadataURLHost(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !parsed.IsAbs() {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", false
	}
	return host, true
}

func bindingHostSet(rows []dbpkg.DomainBindingResolvedRow) map[string]struct{} {
	hosts := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		host := strings.ToLower(strings.TrimSpace(row.Domain))
		if host == "" {
			continue
		}
		hosts[host] = struct{}{}
	}
	return hosts
}

func looksLikeStagingHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	parts := strings.FieldsFunc(host, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	for _, part := range parts {
		if part == "staging" {
			return true
		}
	}
	return false
}

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/internal/bundle"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/internal/release"
	"github.com/benedict2310/htmlctl/pkg/model"
)

type websitesResponse struct {
	Websites []websiteItem `json:"websites"`
}

type websiteItem struct {
	Name               string `json:"name"`
	DefaultStyleBundle string `json:"defaultStyleBundle"`
	BaseTemplate       string `json:"baseTemplate"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type environmentsResponse struct {
	Website      string            `json:"website"`
	Environments []environmentItem `json:"environments"`
}

type environmentItem struct {
	Name            string  `json:"name"`
	ActiveReleaseID *string `json:"activeReleaseId,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type statusResponse struct {
	Website                string         `json:"website"`
	Environment            string         `json:"environment"`
	ActiveReleaseID        *string        `json:"activeReleaseId,omitempty"`
	ActiveReleaseTimestamp *string        `json:"activeReleaseTimestamp,omitempty"`
	DefaultStyleBundle     string         `json:"defaultStyleBundle,omitempty"`
	BaseTemplate           string         `json:"baseTemplate,omitempty"`
	ResourceCounts         resourceCounts `json:"resourceCounts"`
}

type resourceCounts struct {
	Pages      int `json:"pages"`
	Components int `json:"components"`
	Styles     int `json:"styles"`
	Assets     int `json:"assets"`
	Scripts    int `json:"scripts"`
}

type desiredStateManifestResponse struct {
	Website     string                 `json:"website"`
	Environment string                 `json:"environment"`
	Files       []desiredManifestEntry `json:"files"`
}

type desiredManifestEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type resourcesResponse struct {
	Website        string              `json:"website"`
	Environment    string              `json:"environment"`
	Site           websiteResource     `json:"site"`
	Pages          []pageResource      `json:"pages"`
	Components     []componentResource `json:"components"`
	Styles         []styleResource     `json:"styles"`
	Assets         []assetResource     `json:"assets"`
	Branding       []brandingResource  `json:"branding"`
	ResourceCounts resourceCounts      `json:"resourceCounts"`
}

type websiteResource struct {
	Name               string             `json:"name"`
	DefaultStyleBundle string             `json:"defaultStyleBundle"`
	BaseTemplate       string             `json:"baseTemplate"`
	Head               *model.WebsiteHead `json:"head,omitempty"`
	SEO                *model.WebsiteSEO  `json:"seo,omitempty"`
	ContentHash        string             `json:"contentHash,omitempty"`
	CreatedAt          string             `json:"createdAt"`
	UpdatedAt          string             `json:"updatedAt"`
}

type pageResource struct {
	Name        string                 `json:"name"`
	Route       string                 `json:"route"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Layout      []model.PageLayoutItem `json:"layout"`
	Head        *model.PageHead        `json:"head,omitempty"`
	ContentHash string                 `json:"contentHash"`
	CreatedAt   string                 `json:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

type componentResource struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	HasCSS      bool   `json:"hasCss"`
	HasJS       bool   `json:"hasJs"`
	ContentHash string `json:"contentHash"`
	CSSHash     string `json:"cssHash,omitempty"`
	JSHash      string `json:"jsHash,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type styleFileResource struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type styleResource struct {
	Name      string              `json:"name"`
	Files     []styleFileResource `json:"files"`
	CreatedAt string              `json:"createdAt"`
	UpdatedAt string              `json:"updatedAt"`
}

type assetResource struct {
	Path        string `json:"path"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentHash string `json:"contentHash"`
	CreatedAt   string `json:"createdAt"`
}

type brandingResource struct {
	Slot        string `json:"slot"`
	SourcePath  string `json:"sourcePath"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentHash string `json:"contentHash"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type missingResourceError struct {
	kind string
	name string
}

func (e *missingResourceError) Error() string {
	return fmt.Sprintf("%s %q not found", e.kind, e.name)
}

func (s *Server) handleWebsites(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/websites" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `SELECT name, default_style_bundle, base_template, created_at, updated_at FROM websites ORDER BY name`)
	if err != nil {
		s.writeInternalAPIError(w, r, "query websites failed", err)
		return
	}
	defer rows.Close()

	out := []websiteItem{}
	for rows.Next() {
		var item websiteItem
		if err := rows.Scan(&item.Name, &item.DefaultStyleBundle, &item.BaseTemplate, &item.CreatedAt, &item.UpdatedAt); err != nil {
			s.writeInternalAPIError(w, r, "scan website row failed", err)
			return
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		s.writeInternalAPIError(w, r, "iterate website rows failed", err)
		return
	}

	writeJSON(w, http.StatusOK, websitesResponse{Websites: out})
}

func (s *Server) handleEnvironments(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, ok, err := parseEnvironmentsPath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	q := dbpkg.NewQueries(s.db)
	websiteRow, err := q.GetWebsiteByName(r.Context(), website)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup website failed", err, "website", website)
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `SELECT name, active_release_id, created_at, updated_at FROM environments WHERE website_id = ? ORDER BY name`, websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "query environments failed", err, "website", website)
		return
	}
	defer rows.Close()

	out := []environmentItem{}
	for rows.Next() {
		var item environmentItem
		if err := rows.Scan(&item.Name, &item.ActiveReleaseID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			s.writeInternalAPIError(w, r, "scan environment row failed", err, "website", website)
			return
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		s.writeInternalAPIError(w, r, "iterate environment rows failed", err, "website", website)
		return
	}

	writeJSON(w, http.StatusOK, environmentsResponse{
		Website:      websiteRow.Name,
		Environments: out,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseStatusPath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	q := dbpkg.NewQueries(s.db)
	websiteRow, err := q.GetWebsiteByName(r.Context(), website)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup website failed", err, "website", website, "environment", env)
		return
	}
	envRow, err := q.GetEnvironmentByName(r.Context(), websiteRow.ID, env)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("environment %q not found", env), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup environment failed", err, "website", website, "environment", env)
		return
	}

	pageCount, err := countByWebsiteID(r.Context(), s.db, "pages", websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "count pages failed", err, "website", website, "environment", env)
		return
	}
	componentCount, err := countByWebsiteID(r.Context(), s.db, "components", websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "count components failed", err, "website", website, "environment", env)
		return
	}
	styleCount, err := countByWebsiteID(r.Context(), s.db, "style_bundles", websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "count style bundles failed", err, "website", website, "environment", env)
		return
	}
	scriptCount, err := countScripts(r.Context(), s.db, websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "count scripts failed", err, "website", website, "environment", env)
		return
	}
	assetCount, err := countNonScriptAssets(r.Context(), s.db, websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "count assets failed", err, "website", website, "environment", env)
		return
	}

	var activeReleaseTimestamp *string
	if envRow.ActiveReleaseID != nil {
		release, err := q.GetReleaseByID(r.Context(), *envRow.ActiveReleaseID)
		if err == nil {
			activeReleaseTimestamp = &release.CreatedAt
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		Website:                websiteRow.Name,
		Environment:            envRow.Name,
		ActiveReleaseID:        envRow.ActiveReleaseID,
		ActiveReleaseTimestamp: activeReleaseTimestamp,
		DefaultStyleBundle:     websiteRow.DefaultStyleBundle,
		BaseTemplate:           websiteRow.BaseTemplate,
		ResourceCounts: resourceCounts{
			Pages:      pageCount,
			Components: componentCount,
			Styles:     styleCount,
			Assets:     assetCount,
			Scripts:    scriptCount,
		},
	})
}

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseResourcesPath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	resources, err := buildResourcesResponse(r.Context(), s.db, website, env)
	if err != nil {
		var missing *missingResourceError
		if errors.As(err, &missing) {
			writeAPIError(w, http.StatusNotFound, missing.Error(), nil)
			return
		}
		s.writeInternalAPIError(w, r, "build resources response failed", err, "website", website, "environment", env)
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseSourcePath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil || s.blobStore == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	builder, err := release.NewBuilder(s.db, s.blobStore, s.dataPaths.WebsitesRoot, s.logger)
	if err != nil {
		s.writeInternalAPIError(w, r, "initialize source exporter failed", err, "website", website, "environment", env)
		return
	}
	filename := fmt.Sprintf("%s-%s-source.tar.gz", website, env)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if err := builder.WriteSourceArchive(r.Context(), website, env, w); err != nil {
		var notFound *release.NotFoundError
		if errors.As(err, &notFound) {
			writeAPIError(w, http.StatusNotFound, notFound.Error(), nil)
			return
		}
		var unsupported *release.UnsupportedSourceExportError
		if errors.As(err, &unsupported) {
			writeAPIError(w, http.StatusConflict, unsupported.Error(), nil)
			return
		}
		s.writeInternalAPIError(w, r, "export source archive failed", err, "website", website, "environment", env)
		return
	}
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseManifestPath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	q := dbpkg.NewQueries(s.db)
	websiteRow, err := q.GetWebsiteByName(r.Context(), website)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup website failed", err, "website", website, "environment", env)
		return
	}
	envRow, err := q.GetEnvironmentByName(r.Context(), websiteRow.ID, env)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("environment %q not found", env), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup environment failed", err, "website", website, "environment", env)
		return
	}

	pages, err := q.ListPagesByWebsite(r.Context(), websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list pages failed", err, "website", website, "environment", env)
		return
	}
	components, err := q.ListComponentsByWebsite(r.Context(), websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list components failed", err, "website", website, "environment", env)
		return
	}
	styleBundles, err := q.ListStyleBundlesByWebsite(r.Context(), websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list style bundles failed", err, "website", website, "environment", env)
		return
	}
	assets, err := q.ListAssetsByWebsite(r.Context(), websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list assets failed", err, "website", website, "environment", env)
		return
	}
	websiteIcons, err := q.ListWebsiteIconsByWebsite(r.Context(), websiteRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list website icons failed", err, "website", website, "environment", env)
		return
	}

	byPath := map[string]string{}
	if strings.TrimSpace(websiteRow.ContentHash) != "" {
		if err := addManifestEntry(byPath, "website.yaml", websiteRow.ContentHash); err != nil {
			s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
			return
		}
	}
	for _, row := range pages {
		entryPath := path.Join("pages", row.Name+".page.yaml")
		if err := addManifestEntry(byPath, entryPath, row.ContentHash); err != nil {
			s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
			return
		}
	}
	for _, row := range components {
		entryPath := path.Join("components", row.Name+".html")
		if err := addManifestEntry(byPath, entryPath, row.ContentHash); err != nil {
			s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
			return
		}
		if strings.TrimSpace(row.CSSHash) != "" {
			if err := addManifestEntry(byPath, path.Join("components", row.Name+".css"), row.CSSHash); err != nil {
				s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
				return
			}
		}
		if strings.TrimSpace(row.JSHash) != "" {
			if err := addManifestEntry(byPath, path.Join("components", row.Name+".js"), row.JSHash); err != nil {
				s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
				return
			}
		}
	}
	for _, row := range styleBundles {
		refs := []bundle.FileRef{}
		if err := json.Unmarshal([]byte(row.FilesJSON), &refs); err != nil {
			s.writeInternalAPIError(w, r, "parse style bundle files failed", err, "website", website, "environment", env, "bundle", row.Name)
			return
		}
		for _, ref := range refs {
			if err := addManifestEntry(byPath, ref.File, ref.Hash); err != nil {
				s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
				return
			}
		}
	}
	for _, row := range assets {
		if err := addManifestEntry(byPath, row.Filename, row.ContentHash); err != nil {
			s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
			return
		}
	}
	for _, row := range websiteIcons {
		if err := addManifestEntry(byPath, row.SourcePath, row.ContentHash); err != nil {
			s.writeInternalAPIError(w, r, "build desired-state manifest failed", err, "website", website, "environment", env)
			return
		}
	}

	paths := make([]string, 0, len(byPath))
	for filePath := range byPath {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	files := make([]desiredManifestEntry, 0, len(paths))
	for _, filePath := range paths {
		files = append(files, desiredManifestEntry{
			Path: filePath,
			Hash: byPath[filePath],
		})
	}

	writeJSON(w, http.StatusOK, desiredStateManifestResponse{
		Website:     websiteRow.Name,
		Environment: envRow.Name,
		Files:       files,
	})
}

func parseEnvironmentsPath(pathValue string) (website string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 5 {
		return "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" {
		return "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	if strings.TrimSpace(website) == "" {
		return "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	return website, true, nil
}

func parseStatusPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "status" {
		return "", "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
		return "", "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, env, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	if err := validateResourceName(env); err != nil {
		return website, env, false, fmt.Errorf("invalid environment name %q: %w", env, err)
	}
	return website, env, true, nil
}

func parseManifestPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "manifest" {
		return "", "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
		return "", "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, env, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	if err := validateResourceName(env); err != nil {
		return website, env, false, fmt.Errorf("invalid environment name %q: %w", env, err)
	}
	return website, env, true, nil
}

func parseResourcesPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "resources" {
		return "", "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
		return "", "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, env, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	if err := validateResourceName(env); err != nil {
		return website, env, false, fmt.Errorf("invalid environment name %q: %w", env, err)
	}
	return website, env, true, nil
}

func parseSourcePath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "source" {
		return "", "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
		return "", "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, env, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	if err := validateResourceName(env); err != nil {
		return website, env, false, fmt.Errorf("invalid environment name %q: %w", env, err)
	}
	return website, env, true, nil
}

func addManifestEntry(byPath map[string]string, rawPath, rawHash string) error {
	filePath := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	hash := strings.TrimSpace(strings.ToLower(rawHash))
	if filePath == "" {
		return fmt.Errorf("manifest path is empty")
	}
	if hash == "" {
		return fmt.Errorf("manifest hash for %q is empty", filePath)
	}
	if existing, ok := byPath[filePath]; ok {
		if existing != hash {
			return fmt.Errorf("path %q has conflicting hashes (%s vs %s)", filePath, existing, hash)
		}
		return nil
	}
	byPath[filePath] = hash
	return nil
}

func countByWebsiteID(ctx context.Context, db *sql.DB, table string, websiteID int64) (int, error) {
	switch table {
	case "pages", "components", "style_bundles":
	default:
		return 0, fmt.Errorf("unsupported count table %q", table)
	}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE website_id = ?", table)
	var count int
	if err := db.QueryRowContext(ctx, query, websiteID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func countScripts(ctx context.Context, db *sql.DB, websiteID int64) (int, error) {
	query := `
SELECT COUNT(*)
FROM assets
WHERE website_id = ?
  AND filename LIKE 'scripts/%'`
	var count int
	if err := db.QueryRowContext(ctx, query, websiteID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func buildResourcesResponse(ctx context.Context, db *sql.DB, website, env string) (resourcesResponse, error) {
	q := dbpkg.NewQueries(db)
	websiteRow, err := q.GetWebsiteByName(ctx, website)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return resourcesResponse{}, &missingResourceError{kind: "website", name: website}
		}
		return resourcesResponse{}, err
	}
	envRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, env)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return resourcesResponse{}, &missingResourceError{kind: "environment", name: env}
		}
		return resourcesResponse{}, err
	}

	pages, err := q.ListPagesByWebsite(ctx, websiteRow.ID)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("list pages failed: %w", err)
	}
	components, err := q.ListComponentsByWebsite(ctx, websiteRow.ID)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("list components failed: %w", err)
	}
	styleBundles, err := q.ListStyleBundlesByWebsite(ctx, websiteRow.ID)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("list style bundles failed: %w", err)
	}
	assets, err := q.ListAssetsByWebsite(ctx, websiteRow.ID)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("list assets failed: %w", err)
	}
	websiteIcons, err := q.ListWebsiteIconsByWebsite(ctx, websiteRow.ID)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("list website icons failed: %w", err)
	}

	siteHead, err := decodeWebsiteHead(websiteRow.HeadJSON)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("parse website head json: %w", err)
	}
	siteSEO, err := decodeWebsiteSEO(websiteRow.SEOJSON)
	if err != nil {
		return resourcesResponse{}, fmt.Errorf("parse website seo json: %w", err)
	}

	pageResources := make([]pageResource, 0, len(pages))
	for _, row := range pages {
		layout, err := decodePageLayout(row.LayoutJSON)
		if err != nil {
			return resourcesResponse{}, fmt.Errorf("parse page layout json for %s: %w", row.Name, err)
		}
		head, err := decodePageHead(row.HeadJSON)
		if err != nil {
			return resourcesResponse{}, fmt.Errorf("parse page head json for %s: %w", row.Name, err)
		}
		pageResources = append(pageResources, pageResource{
			Name:        row.Name,
			Route:       row.Route,
			Title:       row.Title,
			Description: row.Description,
			Layout:      layout,
			Head:        head,
			ContentHash: row.ContentHash,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}

	componentResources := make([]componentResource, 0, len(components))
	for _, row := range components {
		componentResources = append(componentResources, componentResource{
			Name:        row.Name,
			Scope:       row.Scope,
			HasCSS:      strings.TrimSpace(row.CSSHash) != "",
			HasJS:       strings.TrimSpace(row.JSHash) != "",
			ContentHash: row.ContentHash,
			CSSHash:     row.CSSHash,
			JSHash:      row.JSHash,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}

	styleResources := make([]styleResource, 0, len(styleBundles))
	for _, row := range styleBundles {
		refs, err := decodeStyleRefs(row.FilesJSON)
		if err != nil {
			return resourcesResponse{}, fmt.Errorf("parse style bundle files for %s: %w", row.Name, err)
		}
		files := make([]styleFileResource, 0, len(refs))
		for _, ref := range refs {
			files = append(files, styleFileResource{Path: ref.File, Hash: ref.Hash})
		}
		styleResources = append(styleResources, styleResource{
			Name:      row.Name,
			Files:     files,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}

	assetResources := make([]assetResource, 0, len(assets))
	scriptCount := 0
	for _, row := range assets {
		if isScriptAssetPath(row.Filename) {
			scriptCount++
			continue
		}
		assetResources = append(assetResources, assetResource{
			Path:        row.Filename,
			ContentType: row.ContentType,
			SizeBytes:   row.SizeBytes,
			ContentHash: row.ContentHash,
			CreatedAt:   row.CreatedAt,
		})
	}

	brandingResources := make([]brandingResource, 0, len(websiteIcons))
	for _, row := range websiteIcons {
		brandingResources = append(brandingResources, brandingResource{
			Slot:        row.Slot,
			SourcePath:  row.SourcePath,
			ContentType: row.ContentType,
			SizeBytes:   row.SizeBytes,
			ContentHash: row.ContentHash,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}

	return resourcesResponse{
		Website:     websiteRow.Name,
		Environment: envRow.Name,
		Site: websiteResource{
			Name:               websiteRow.Name,
			DefaultStyleBundle: websiteRow.DefaultStyleBundle,
			BaseTemplate:       websiteRow.BaseTemplate,
			Head:               siteHead,
			SEO:                siteSEO,
			ContentHash:        websiteRow.ContentHash,
			CreatedAt:          websiteRow.CreatedAt,
			UpdatedAt:          websiteRow.UpdatedAt,
		},
		Pages:      pageResources,
		Components: componentResources,
		Styles:     styleResources,
		Assets:     assetResources,
		Branding:   brandingResources,
		ResourceCounts: resourceCounts{
			Pages:      len(pageResources),
			Components: len(componentResources),
			Styles:     len(styleResources),
			Assets:     len(assetResources),
			Scripts:    scriptCount,
		},
	}, nil
}

func decodeWebsiteHead(raw string) (*model.WebsiteHead, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return nil, nil
	}
	var out model.WebsiteHead
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == (model.WebsiteHead{}) {
		return nil, nil
	}
	return &out, nil
}

func decodeWebsiteSEO(raw string) (*model.WebsiteSEO, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return nil, nil
	}
	var out model.WebsiteSEO
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == (model.WebsiteSEO{}) {
		return nil, nil
	}
	return &out, nil
}

func decodePageHead(raw string) (*model.PageHead, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return nil, nil
	}
	var out model.PageHead
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if pageHeadIsEmpty(out) {
		return nil, nil
	}
	return &out, nil
}

func pageHeadIsEmpty(head model.PageHead) bool {
	return strings.TrimSpace(head.CanonicalURL) == "" &&
		len(head.Meta) == 0 &&
		head.OpenGraph == nil &&
		head.Twitter == nil &&
		len(head.JSONLD) == 0
}

func decodePageLayout(raw string) ([]model.PageLayoutItem, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []model.PageLayoutItem
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeStyleRefs(raw string) ([]bundle.FileRef, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []bundle.FileRef
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func isScriptAssetPath(path string) bool {
	return strings.HasPrefix(path, "scripts/")
}

func countNonScriptAssets(ctx context.Context, db *sql.DB, websiteID int64) (int, error) {
	query := `
SELECT COUNT(*)
FROM assets
WHERE website_id = ?
  AND filename NOT LIKE 'scripts/%'`
	var count int
	if err := db.QueryRowContext(ctx, query, websiteID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

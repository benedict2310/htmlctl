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
		ResourceCounts: resourceCounts{
			Pages:      pageCount,
			Components: componentCount,
			Styles:     styleCount,
			Assets:     assetCount,
			Scripts:    scriptCount,
		},
	})
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

	byPath := map[string]string{}
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
  AND (
    filename LIKE 'scripts/%'
    OR filename LIKE '%.js'
    OR filename LIKE '%.mjs'
  )`
	var count int
	if err := db.QueryRowContext(ctx, query, websiteID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func countNonScriptAssets(ctx context.Context, db *sql.DB, websiteID int64) (int, error) {
	query := `
SELECT COUNT(*)
FROM assets
WHERE website_id = ?
  AND NOT (
    filename LIKE 'scripts/%'
    OR filename LIKE '%.js'
    OR filename LIKE '%.mjs'
  )`
	var count int
	if err := db.QueryRowContext(ctx, query, websiteID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

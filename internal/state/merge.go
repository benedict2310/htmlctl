package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"path"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/internal/blob"
	"github.com/benedict2310/htmlctl/internal/bundle"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/pkg/model"
	"gopkg.in/yaml.v3"
)

type BadRequestError struct {
	msg string
}

func (e *BadRequestError) Error() string { return e.msg }

func badRequestf(format string, args ...any) error {
	return &BadRequestError{msg: fmt.Sprintf(format, args...)}
}

type AcceptedResource struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Hash string `json:"hash"`
}

type ChangeSummary struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}

type ApplyResult struct {
	Accepted []AcceptedResource `json:"acceptedResources"`
	Warnings []string           `json:"warnings,omitempty"`
	Changes  ChangeSummary      `json:"changes"`
}

type Applier struct {
	db    *sql.DB
	blobs *blob.Store
}

func NewApplier(db *sql.DB, blobs *blob.Store) (*Applier, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	if blobs == nil {
		return nil, fmt.Errorf("blob store is required")
	}
	return &Applier{db: db, blobs: blobs}, nil
}

func (a *Applier) Apply(ctx context.Context, websiteName, envName string, b bundle.Bundle, dryRun bool) (ApplyResult, error) {
	var out ApplyResult

	websiteName = strings.TrimSpace(websiteName)
	envName = strings.TrimSpace(envName)
	if websiteName == "" || envName == "" {
		return out, badRequestf("website and environment are required")
	}
	if b.Manifest.Website != websiteName {
		return out, badRequestf("manifest.website %q does not match request website %q", b.Manifest.Website, websiteName)
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("begin apply transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
	}()
	q := dbpkg.NewQueries(tx)

	website, err := ensureWebsite(ctx, q, websiteName)
	if err != nil {
		return out, err
	}
	if _, err := ensureEnvironment(ctx, q, website.ID, envName); err != nil {
		return out, err
	}

	pages, err := q.ListPagesByWebsite(ctx, website.ID)
	if err != nil {
		return out, fmt.Errorf("list current pages: %w", err)
	}
	components, err := q.ListComponentsByWebsite(ctx, website.ID)
	if err != nil {
		return out, fmt.Errorf("list current components: %w", err)
	}
	styleBundles, err := q.ListStyleBundlesByWebsite(ctx, website.ID)
	if err != nil {
		return out, fmt.Errorf("list current style bundles: %w", err)
	}
	assets, err := q.ListAssetsByWebsite(ctx, website.ID)
	if err != nil {
		return out, fmt.Errorf("list current assets: %w", err)
	}

	pageByName := map[string]dbpkg.PageRow{}
	for _, row := range pages {
		pageByName[row.Name] = row
	}
	componentByName := map[string]dbpkg.ComponentRow{}
	for _, row := range components {
		componentByName[row.Name] = row
	}
	styleBundleByName := map[string]dbpkg.StyleBundleRow{}
	for _, row := range styleBundles {
		styleBundleByName[row.Name] = row
	}
	assetByFilename := map[string]dbpkg.AssetRow{}
	for _, row := range assets {
		assetByFilename[row.Filename] = row
	}

	keepPages := map[string]struct{}{}
	keepComponents := map[string]struct{}{}
	keepStyleBundles := map[string]struct{}{}
	keepAssets := map[string]struct{}{}

	for _, res := range b.Manifest.Resources {
		kind := strings.ToLower(strings.TrimSpace(res.Kind))
		if res.Deleted {
			resourceID := strings.TrimSpace(res.Name)
			if (kind == "asset" || kind == "script") && strings.TrimSpace(res.File) != "" {
				resourceID = strings.TrimSpace(res.File)
			}
			if b.Manifest.Mode != bundle.ApplyModePartial {
				return out, badRequestf("deleted resources are only valid in partial mode")
			}
			if err := a.applyDeleted(ctx, q, website.ID, kind, resourceID, &out); err != nil {
				return out, err
			}
			continue
		}

		entries := res.FileEntries()
		if len(entries) == 0 {
			return out, badRequestf("resource %q (%s) has no files", res.Name, res.Kind)
		}

		for i, ref := range entries {
			content, ok := b.Files[ref.File]
			if !ok {
				return out, badRequestf("resource %q references missing file %q", res.Name, ref.File)
			}
			canonicalHash, err := bundle.CanonicalHash(ref.Hash)
			if err != nil {
				return out, badRequestf("resource %q has invalid hash %q: %v", res.Name, ref.Hash, err)
			}
			hashHex, _ := bundle.HashHex(canonicalHash)
			entries[i].Hash = canonicalHash
			if !dryRun {
				if _, err := a.blobs.Put(ctx, hashHex, content); err != nil {
					return out, fmt.Errorf("store blob %s for %q: %w", hashHex, ref.File, err)
				}
			}
		}

		switch kind {
		case "component":
			ref := entries[0]
			if existing, ok := componentByName[res.Name]; !ok {
				out.Changes.Created++
			} else if existing.ContentHash != ref.Hash || existing.Scope != "global" {
				out.Changes.Updated++
			}
			if !dryRun {
				if err := q.UpsertComponent(ctx, dbpkg.ComponentRow{
					WebsiteID:   website.ID,
					Name:        res.Name,
					Scope:       "global",
					ContentHash: ref.Hash,
				}); err != nil {
					return out, fmt.Errorf("upsert component %q: %w", res.Name, err)
				}
			}
			keepComponents[res.Name] = struct{}{}
			out.Accepted = append(out.Accepted, AcceptedResource{Kind: "Component", Name: res.Name, Hash: ref.Hash})

		case "page":
			ref := entries[0]
			pageDoc, err := parsePageDocument(b.Files[ref.File])
			if err != nil {
				return out, badRequestf("parse page %q: %v", ref.File, err)
			}
			layoutJSON, err := json.Marshal(pageDoc.Spec.Layout)
			if err != nil {
				return out, fmt.Errorf("marshal page layout for %q: %w", res.Name, err)
			}
			route := normalizeRoute(pageDoc.Spec.Route)
			if route == "" {
				return out, badRequestf("page %q has empty route", res.Name)
			}
			if existing, ok := pageByName[res.Name]; !ok {
				out.Changes.Created++
			} else if existing.ContentHash != ref.Hash || existing.Route != route || existing.Title != pageDoc.Spec.Title || existing.Description != pageDoc.Spec.Description || existing.LayoutJSON != string(layoutJSON) {
				out.Changes.Updated++
			}
			if !dryRun {
				if err := q.UpsertPage(ctx, dbpkg.PageRow{
					WebsiteID:   website.ID,
					Name:        res.Name,
					Route:       route,
					Title:       pageDoc.Spec.Title,
					Description: pageDoc.Spec.Description,
					LayoutJSON:  string(layoutJSON),
					ContentHash: ref.Hash,
				}); err != nil {
					return out, fmt.Errorf("upsert page %q: %w", res.Name, err)
				}
			}
			keepPages[res.Name] = struct{}{}
			out.Accepted = append(out.Accepted, AcceptedResource{Kind: "Page", Name: res.Name, Hash: ref.Hash})

		case "stylebundle":
			sort.Slice(entries, func(i, j int) bool { return entries[i].File < entries[j].File })
			filesJSON, err := json.Marshal(entries)
			if err != nil {
				return out, fmt.Errorf("marshal style bundle %q file list: %w", res.Name, err)
			}
			if existing, ok := styleBundleByName[res.Name]; !ok {
				out.Changes.Created++
			} else if existing.FilesJSON != string(filesJSON) {
				out.Changes.Updated++
			}
			if !dryRun {
				if err := q.UpsertStyleBundle(ctx, dbpkg.StyleBundleRow{
					WebsiteID: website.ID,
					Name:      res.Name,
					FilesJSON: string(filesJSON),
				}); err != nil {
					return out, fmt.Errorf("upsert style bundle %q: %w", res.Name, err)
				}
			}
			keepStyleBundles[res.Name] = struct{}{}
			out.Accepted = append(out.Accepted, AcceptedResource{Kind: "StyleBundle", Name: res.Name, Hash: styleBundleHash(entries)})

		case "asset", "script":
			ref := entries[0]
			filename := ref.File
			content := b.Files[ref.File]
			contentType := strings.TrimSpace(res.ContentType)
			if contentType == "" {
				contentType = inferContentType(filename)
			}
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			size := int64(len(content))
			if existing, ok := assetByFilename[filename]; !ok {
				out.Changes.Created++
			} else if existing.ContentHash != ref.Hash || existing.SizeBytes != size || existing.ContentType != contentType {
				out.Changes.Updated++
			}
			if !dryRun {
				if err := q.UpsertAsset(ctx, dbpkg.AssetRow{
					WebsiteID:   website.ID,
					Filename:    filename,
					ContentType: contentType,
					SizeBytes:   size,
					ContentHash: ref.Hash,
				}); err != nil {
					return out, fmt.Errorf("upsert asset %q: %w", filename, err)
				}
			}
			keepAssets[filename] = struct{}{}
			acceptedKind := "Asset"
			if kind == "script" {
				acceptedKind = "Script"
			}
			out.Accepted = append(out.Accepted, AcceptedResource{Kind: acceptedKind, Name: filename, Hash: ref.Hash})

		default:
			return out, badRequestf("unsupported resource kind %q", res.Kind)
		}
	}

	if b.Manifest.Mode == bundle.ApplyModeFull {
		pageNames := sortedKeys(keepPages)
		componentNames := sortedKeys(keepComponents)
		styleBundleNames := sortedKeys(keepStyleBundles)
		assetNames := sortedKeys(keepAssets)

		deletedPages, err := q.DeletePagesNotIn(ctx, website.ID, pageNames)
		if err != nil {
			return out, fmt.Errorf("delete stale pages: %w", err)
		}
		deletedComponents, err := q.DeleteComponentsNotIn(ctx, website.ID, componentNames)
		if err != nil {
			return out, fmt.Errorf("delete stale components: %w", err)
		}
		deletedBundles, err := q.DeleteStyleBundlesNotIn(ctx, website.ID, styleBundleNames)
		if err != nil {
			return out, fmt.Errorf("delete stale style bundles: %w", err)
		}
		deletedAssets, err := q.DeleteAssetsNotIn(ctx, website.ID, assetNames)
		if err != nil {
			return out, fmt.Errorf("delete stale assets: %w", err)
		}
		out.Changes.Deleted += int(deletedPages + deletedComponents + deletedBundles + deletedAssets)
	}

	if dryRun {
		return out, nil
	}
	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("commit apply transaction: %w", err)
	}
	committed = true
	return out, nil
}

func (a *Applier) applyDeleted(ctx context.Context, q *dbpkg.Queries, websiteID int64, kind, name string, out *ApplyResult) error {
	switch kind {
	case "component":
		n, err := q.DeleteComponentByName(ctx, websiteID, name)
		if err != nil {
			return fmt.Errorf("delete component %q: %w", name, err)
		}
		out.Changes.Deleted += int(n)
	case "page":
		n, err := q.DeletePageByName(ctx, websiteID, name)
		if err != nil {
			return fmt.Errorf("delete page %q: %w", name, err)
		}
		out.Changes.Deleted += int(n)
	case "stylebundle":
		n, err := q.DeleteStyleBundleByName(ctx, websiteID, name)
		if err != nil {
			return fmt.Errorf("delete style bundle %q: %w", name, err)
		}
		out.Changes.Deleted += int(n)
	case "asset", "script":
		n, err := q.DeleteAssetByFilename(ctx, websiteID, name)
		if err != nil {
			return fmt.Errorf("delete asset %q: %w", name, err)
		}
		out.Changes.Deleted += int(n)
	default:
		return badRequestf("unsupported deleted resource kind %q", kind)
	}
	out.Accepted = append(out.Accepted, AcceptedResource{Kind: strings.Title(kind), Name: name})
	return nil
}

func ensureWebsite(ctx context.Context, q *dbpkg.Queries, websiteName string) (dbpkg.WebsiteRow, error) {
	row, err := q.GetWebsiteByName(ctx, websiteName)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return row, fmt.Errorf("load website %q: %w", websiteName, err)
	}
	if _, err := q.InsertWebsite(ctx, dbpkg.WebsiteRow{
		Name:               websiteName,
		DefaultStyleBundle: "default",
		BaseTemplate:       "default",
	}); err != nil {
		existing, getErr := q.GetWebsiteByName(ctx, websiteName)
		if getErr == nil {
			return existing, nil
		}
		return row, fmt.Errorf("create website %q: %w", websiteName, err)
	}
	return q.GetWebsiteByName(ctx, websiteName)
}

func ensureEnvironment(ctx context.Context, q *dbpkg.Queries, websiteID int64, envName string) (dbpkg.EnvironmentRow, error) {
	row, err := q.GetEnvironmentByName(ctx, websiteID, envName)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return row, fmt.Errorf("load environment %q: %w", envName, err)
	}
	if _, err := q.InsertEnvironment(ctx, dbpkg.EnvironmentRow{
		WebsiteID: websiteID,
		Name:      envName,
	}); err != nil {
		existing, getErr := q.GetEnvironmentByName(ctx, websiteID, envName)
		if getErr == nil {
			return existing, nil
		}
		return row, fmt.Errorf("create environment %q: %w", envName, err)
	}
	return q.GetEnvironmentByName(ctx, websiteID, envName)
}

func parsePageDocument(content []byte) (model.Page, error) {
	var page model.Page
	if err := yaml.Unmarshal(content, &page); err != nil {
		return page, err
	}
	return page, nil
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return ""
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	if len(route) > 1 {
		route = strings.TrimRight(route, "/")
	}
	return route
}

func styleBundleHash(files []bundle.FileRef) string {
	if len(files) == 1 {
		return files[0].Hash
	}
	b, _ := json.Marshal(files)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func inferContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	if ext == "" {
		return ""
	}
	if contentType := mime.TypeByExtension(ext); contentType != "" {
		return contentType
	}
	switch ext {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".html":
		return "text/html; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json"
	default:
		return ""
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

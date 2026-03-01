package release

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/blob"
	"github.com/benedict2310/htmlctl/internal/bundle"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/internal/names"
	"github.com/benedict2310/htmlctl/internal/ogimage"
	"github.com/benedict2310/htmlctl/pkg/loader"
	"github.com/benedict2310/htmlctl/pkg/model"
	"github.com/benedict2310/htmlctl/pkg/renderer"
	"gopkg.in/yaml.v3"
)

type NotFoundError struct {
	msg string
}

func (e *NotFoundError) Error() string { return e.msg }

type BuildResult struct {
	ReleaseID         string
	EnvironmentID     int64
	PreviousReleaseID *string
	ManifestJSON      string
	OutputHashes      map[string]string
	BuildLog          string
}

type Builder struct {
	db           *sql.DB
	blobs        *blob.Store
	websitesRoot string
	logger       *slog.Logger
	nowFn        func() time.Time
	idFn         func(time.Time) (string, error)
	generateFn   func(ogimage.Card) ([]byte, error)
	linkFileFn   func(oldname, newname string) error
	copyFileFn   func(sourcePath, targetPath string) error
}

const (
	styleTokensFile  = "tokens.css"
	styleDefaultFile = "default.css"
)

func NewBuilder(db *sql.DB, blobs *blob.Store, websitesRoot string, logger *slog.Logger) (*Builder, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	if blobs == nil {
		return nil, fmt.Errorf("blob store is required")
	}
	if strings.TrimSpace(websitesRoot) == "" {
		return nil, fmt.Errorf("websites root is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Builder{
		db:           db,
		blobs:        blobs,
		websitesRoot: websitesRoot,
		logger:       logger,
		nowFn:        time.Now,
		idFn:         NewReleaseID,
		generateFn:   ogimage.Generate,
		linkFileFn:   os.Link,
		copyFileFn:   copyFile,
	}, nil
}

func (b *Builder) Build(ctx context.Context, websiteName, envName string) (out BuildResult, err error) {
	q := dbpkg.NewQueries(b.db)
	websiteName = strings.TrimSpace(websiteName)
	envName = strings.TrimSpace(envName)
	if websiteName == "" || envName == "" {
		return out, fmt.Errorf("website and environment are required")
	}

	readTx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, fmt.Errorf("begin release read transaction: %w", err)
	}
	readTxDone := false
	defer func() {
		if !readTxDone {
			_ = readTx.Rollback()
		}
	}()
	readQ := dbpkg.NewQueries(readTx)

	website, err := readQ.GetWebsiteByName(ctx, websiteName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("website %q not found", websiteName)}
		}
		return out, fmt.Errorf("load website %q: %w", websiteName, err)
	}
	env, err := readQ.GetEnvironmentByName(ctx, website.ID, envName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("environment %q not found", envName)}
		}
		return out, fmt.Errorf("load environment %q: %w", envName, err)
	}

	releaseID, err := b.idFn(b.nowFn())
	if err != nil {
		return out, err
	}
	out.ReleaseID = releaseID
	out.EnvironmentID = env.ID
	out.PreviousReleaseID = env.ActiveReleaseID

	log := newBuildLog()
	log.Addf("starting release build website=%s env=%s release=%s", website.Name, env.Name, releaseID)

	snapshot, err := b.loadDesiredState(ctx, readQ, website, env)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, "{}", err)
	}
	if err := readTx.Commit(); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, "{}", fmt.Errorf("commit release read transaction: %w", err))
	}
	readTxDone = true

	manifestJSONBytes, err := json.MarshalIndent(snapshot.Manifest(), "", "  ")
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, "{}", fmt.Errorf("marshal release manifest snapshot: %w", err))
	}
	manifestJSON := string(manifestJSONBytes)
	out.ManifestJSON = manifestJSON

	envDir := filepath.Join(b.websitesRoot, website.Name, "envs", env.Name)
	releasesRoot := filepath.Join(envDir, "releases")
	tmpReleaseDir := filepath.Join(releasesRoot, releaseID+".tmp")
	finalReleaseDir := filepath.Join(releasesRoot, releaseID)
	buildRoot := filepath.Join(envDir, "build")
	sourceDir := filepath.Join(buildRoot, releaseID)
	ogProbeDir := filepath.Join(buildRoot, releaseID+".ogprobe")

	if err := os.MkdirAll(releasesRoot, 0o755); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("create releases directory %s: %w", releasesRoot, err))
	}
	if err := os.MkdirAll(buildRoot, 0o755); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("create build directory %s: %w", buildRoot, err))
	}
	if _, err := os.Stat(finalReleaseDir); err == nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("release directory already exists: %s", finalReleaseDir))
	} else if err != nil && !os.IsNotExist(err) {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("stat release directory %s: %w", finalReleaseDir, err))
	}

	_ = os.RemoveAll(tmpReleaseDir)
	_ = os.RemoveAll(sourceDir)
	prevTarget, hadPrevTarget, err := ReadCurrentSymlinkTarget(envDir)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	switchedCurrent := false
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(ogProbeDir)
		if err != nil {
			_ = os.RemoveAll(tmpReleaseDir)
			if switchedCurrent {
				restoreTarget := ""
				if hadPrevTarget {
					restoreTarget = prevTarget
				}
				if restoreErr := SetCurrentSymlinkTarget(envDir, restoreTarget); restoreErr != nil {
					b.logger.Error("failed to restore current symlink after release failure", "env_dir", envDir, "error", restoreErr)
				}
			}
		}
	}()

	log.Addf("materializing source state from sqlite + blob store")
	if err := b.materializeSource(ctx, sourceDir, snapshot); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}

	site, err := loader.LoadSite(sourceDir)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("load source site: %w", err))
	}
	warnLocalhostMetadataURLs(site, log)
	log.Addf("ensuring og image blobs")
	ogPageHashes, err := b.ensureOGBlobs(ctx, site, log)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	ogReadyPageHashes, err := b.preflightOGMaterialization(ctx, ogPageHashes, ogProbeDir, log)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	b.injectOGImageMetadata(site, ogReadyPageHashes, log)

	log.Addf("rendering static output")
	if err := renderer.Render(site, tmpReleaseDir); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("render release output: %w", err))
	}

	if err := b.copyOriginalStyles(sourceDir, tmpReleaseDir); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	if err := b.copyOriginalAssets(ctx, snapshot.Assets, tmpReleaseDir); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	if err := b.materializeOGImages(ctx, tmpReleaseDir, ogReadyPageHashes, log); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}

	buildLogText := log.String()
	if err := writeFile(filepath.Join(tmpReleaseDir, ".manifest.json"), []byte(manifestJSON)); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("write release manifest snapshot: %w", err))
	}
	if err := writeFile(filepath.Join(tmpReleaseDir, ".build-log.txt"), []byte(buildLogText)); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("write release build log: %w", err))
	}

	hashes, err := computeOutputHashes(tmpReleaseDir)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	hashesJSONBytes, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("marshal output hashes: %w", err))
	}
	if err := writeFile(filepath.Join(tmpReleaseDir, ".output-hashes.json"), hashesJSONBytes); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("write output hashes file: %w", err))
	}

	log.Addf("finalizing release directory")
	if err := os.Rename(tmpReleaseDir, finalReleaseDir); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("finalize release directory: %w", err))
	}

	log.Addf("switching current symlink")
	if err := SwitchCurrentSymlink(envDir, releaseID); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, err)
	}
	switchedCurrent = true

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("begin release commit transaction: %w", err))
	}
	txDone := false
	defer func() {
		if !txDone {
			_ = tx.Rollback()
		}
	}()

	out.BuildLog = log.String()
	out.OutputHashes = hashes

	txq := dbpkg.NewQueries(tx)
	if err := txq.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            releaseID,
		EnvironmentID: env.ID,
		ManifestJSON:  manifestJSON,
		OutputHashes:  string(hashesJSONBytes),
		BuildLog:      out.BuildLog,
		Status:        "active",
	}); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("insert active release row: %w", err))
	}
	if err := txq.UpdateEnvironmentActiveRelease(ctx, env.ID, &releaseID); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("set environment active release: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return b.recordFailedAndReturn(ctx, q, out, log, manifestJSON, fmt.Errorf("commit release transaction: %w", err))
	}
	txDone = true
	switchedCurrent = false

	log.Addf("release build completed")
	out.BuildLog = log.String()
	return out, nil
}

func (b *Builder) recordFailedAndReturn(ctx context.Context, q *dbpkg.Queries, out BuildResult, log *buildLog, manifestJSON string, cause error) (BuildResult, error) {
	log.Addf("release build failed: %v", cause)
	if out.ReleaseID != "" && out.EnvironmentID != 0 {
		hashesJSON := "{}"
		if _, err := q.GetReleaseByID(ctx, out.ReleaseID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				insertErr := q.InsertRelease(ctx, dbpkg.ReleaseRow{
					ID:            out.ReleaseID,
					EnvironmentID: out.EnvironmentID,
					ManifestJSON:  manifestJSON,
					OutputHashes:  hashesJSON,
					BuildLog:      log.String(),
					Status:        "failed",
				})
				if insertErr != nil {
					b.logger.Error("failed to persist failed release row", "release_id", out.ReleaseID, "error", insertErr)
				}
			}
		}
	}
	return out, cause
}

type desiredState struct {
	Website      dbpkg.WebsiteRow
	Environment  dbpkg.EnvironmentRow
	Pages        []dbpkg.PageRow
	Components   []dbpkg.ComponentRow
	StyleBundles []dbpkg.StyleBundleRow
	Assets       []dbpkg.AssetRow
	WebsiteIcons []dbpkg.WebsiteIconRow
}

type manifestSnapshot struct {
	APIVersion  string                 `json:"apiVersion"`
	Kind        string                 `json:"kind"`
	Website     string                 `json:"website"`
	Environment string                 `json:"environment"`
	GeneratedAt string                 `json:"generatedAt"`
	Resources   map[string]interface{} `json:"resources"`
}

func (s desiredState) Manifest() manifestSnapshot {
	pages := make([]map[string]any, 0, len(s.Pages))
	for _, row := range s.Pages {
		pages = append(pages, map[string]any{
			"name":        row.Name,
			"route":       row.Route,
			"title":       row.Title,
			"description": row.Description,
			"head":        json.RawMessage(row.HeadJSONOrDefault()),
			"contentHash": row.ContentHash,
		})
	}
	components := make([]map[string]any, 0, len(s.Components))
	for _, row := range s.Components {
		components = append(components, map[string]any{
			"name":        row.Name,
			"scope":       row.Scope,
			"contentHash": row.ContentHash,
		})
	}
	styleBundles := make([]map[string]any, 0, len(s.StyleBundles))
	for _, row := range s.StyleBundles {
		styleBundles = append(styleBundles, map[string]any{
			"name":  row.Name,
			"files": json.RawMessage(row.FilesJSON),
		})
	}
	assets := make([]map[string]any, 0, len(s.Assets))
	for _, row := range s.Assets {
		assets = append(assets, map[string]any{
			"filename":    row.Filename,
			"contentType": row.ContentType,
			"sizeBytes":   row.SizeBytes,
			"contentHash": row.ContentHash,
		})
	}
	websiteIcons := make([]map[string]any, 0, len(s.WebsiteIcons))
	for _, row := range s.WebsiteIcons {
		websiteIcons = append(websiteIcons, map[string]any{
			"slot":        row.Slot,
			"sourcePath":  row.SourcePath,
			"contentType": row.ContentType,
			"sizeBytes":   row.SizeBytes,
			"contentHash": row.ContentHash,
		})
	}
	return manifestSnapshot{
		APIVersion:  "htmlctl.dev/v1",
		Kind:        "ReleaseSnapshot",
		Website:     s.Website.Name,
		Environment: s.Environment.Name,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Resources: map[string]any{
			"website": map[string]any{
				"defaultStyleBundle": s.Website.DefaultStyleBundle,
				"baseTemplate":       s.Website.BaseTemplate,
				"head":               json.RawMessage(s.Website.HeadJSONOrDefault()),
				"contentHash":        s.Website.ContentHash,
			},
			"pages":        pages,
			"components":   components,
			"styleBundles": styleBundles,
			"assets":       assets,
			"websiteIcons": websiteIcons,
		},
	}
}

func (b *Builder) loadDesiredState(ctx context.Context, q *dbpkg.Queries, website dbpkg.WebsiteRow, env dbpkg.EnvironmentRow) (desiredState, error) {
	pages, err := q.ListPagesByWebsite(ctx, website.ID)
	if err != nil {
		return desiredState{}, fmt.Errorf("load pages for release build: %w", err)
	}
	components, err := q.ListComponentsByWebsite(ctx, website.ID)
	if err != nil {
		return desiredState{}, fmt.Errorf("load components for release build: %w", err)
	}
	styleBundles, err := q.ListStyleBundlesByWebsite(ctx, website.ID)
	if err != nil {
		return desiredState{}, fmt.Errorf("load style bundles for release build: %w", err)
	}
	assets, err := q.ListAssetsByWebsite(ctx, website.ID)
	if err != nil {
		return desiredState{}, fmt.Errorf("load assets for release build: %w", err)
	}
	websiteIcons, err := q.ListWebsiteIconsByWebsite(ctx, website.ID)
	if err != nil {
		return desiredState{}, fmt.Errorf("load website icons for release build: %w", err)
	}
	if len(pages) == 0 {
		return desiredState{}, fmt.Errorf("website %q has no pages to build", website.Name)
	}
	return desiredState{
		Website:      website,
		Environment:  env,
		Pages:        pages,
		Components:   components,
		StyleBundles: styleBundles,
		Assets:       assets,
		WebsiteIcons: websiteIcons,
	}, nil
}

func (b *Builder) materializeSource(ctx context.Context, sourceDir string, state desiredState) error {
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return fmt.Errorf("create source directory %s: %w", sourceDir, err)
	}
	websiteDoc := model.Website{
		APIVersion: model.APIVersionV1,
		Kind:       model.KindWebsite,
		Metadata:   model.Metadata{Name: state.Website.Name},
		Spec: model.WebsiteSpec{
			DefaultStyleBundle: state.Website.DefaultStyleBundle,
			BaseTemplate:       state.Website.BaseTemplate,
		},
	}
	websiteHead, err := parseWebsiteHeadJSON(state.Website.HeadJSON)
	if err != nil {
		return err
	}
	websiteDoc.Spec.Head = websiteHead
	websiteBytes, err := yaml.Marshal(websiteDoc)
	if err != nil {
		return fmt.Errorf("marshal website yaml: %w", err)
	}
	if err := writeFile(filepath.Join(sourceDir, "website.yaml"), websiteBytes); err != nil {
		return fmt.Errorf("write website.yaml: %w", err)
	}

	for _, row := range state.Components {
		componentName, err := sanitizeResourceName(row.Name)
		if err != nil {
			return fmt.Errorf("invalid component name %q: %w", row.Name, err)
		}
		content, err := b.readBlob(ctx, row.ContentHash)
		if err != nil {
			return fmt.Errorf("load component %q blob: %w", row.Name, err)
		}
		if err := writeFile(filepath.Join(sourceDir, "components", componentName+".html"), content); err != nil {
			return fmt.Errorf("write component %q: %w", row.Name, err)
		}
	}

	for _, row := range state.Pages {
		pageName, err := sanitizeResourceName(row.Name)
		if err != nil {
			return fmt.Errorf("invalid page name %q: %w", row.Name, err)
		}
		layout := []model.PageLayoutItem{}
		if strings.TrimSpace(row.LayoutJSON) != "" {
			if err := json.Unmarshal([]byte(row.LayoutJSON), &layout); err != nil {
				return fmt.Errorf("parse layout json for page %q: %w", row.Name, err)
			}
		}
		head, err := parsePageHeadJSON(row.Name, row.HeadJSON)
		if err != nil {
			return err
		}
		pageDoc := model.Page{
			APIVersion: model.APIVersionV1,
			Kind:       model.KindPage,
			Metadata:   model.Metadata{Name: pageName},
			Spec: model.PageSpec{
				Route:       row.Route,
				Title:       row.Title,
				Description: row.Description,
				Layout:      layout,
				Head:        head,
			},
		}
		pageBytes, err := yaml.Marshal(pageDoc)
		if err != nil {
			return fmt.Errorf("marshal page %q yaml: %w", row.Name, err)
		}
		if err := writeFile(filepath.Join(sourceDir, "pages", pageName+".page.yaml"), pageBytes); err != nil {
			return fmt.Errorf("write page %q file: %w", row.Name, err)
		}
	}

	styleRefs, err := resolveDefaultStyleRefs(state.Website.DefaultStyleBundle, state.StyleBundles)
	if err != nil {
		return err
	}
	for name, hash := range styleRefs {
		content, err := b.readBlob(ctx, hash)
		if err != nil {
			return fmt.Errorf("load style file %q blob: %w", name, err)
		}
		if err := writeFile(filepath.Join(sourceDir, "styles", name), content); err != nil {
			return fmt.Errorf("write style file %q: %w", name, err)
		}
	}

	for _, row := range state.Assets {
		rel, err := sanitizeRelPath(row.Filename)
		if err != nil {
			return fmt.Errorf("invalid asset filename %q: %w", row.Filename, err)
		}
		content, err := b.readBlob(ctx, row.ContentHash)
		if err != nil {
			return fmt.Errorf("load asset %q blob: %w", row.Filename, err)
		}
		if err := writeFile(filepath.Join(sourceDir, filepath.FromSlash(rel)), content); err != nil {
			return fmt.Errorf("write asset source file %q: %w", row.Filename, err)
		}
	}
	for _, row := range state.WebsiteIcons {
		rel, err := sanitizeRelPath(row.SourcePath)
		if err != nil {
			return fmt.Errorf("invalid website icon source path %q: %w", row.SourcePath, err)
		}
		content, err := b.readBlob(ctx, row.ContentHash)
		if err != nil {
			return fmt.Errorf("load website icon %q blob: %w", row.SourcePath, err)
		}
		if err := writeFile(filepath.Join(sourceDir, filepath.FromSlash(rel)), content); err != nil {
			return fmt.Errorf("write website icon source file %q: %w", row.SourcePath, err)
		}
	}

	return nil
}

func resolveDefaultStyleRefs(defaultName string, bundles []dbpkg.StyleBundleRow) (map[string]string, error) {
	name := strings.TrimSpace(defaultName)
	if name == "" {
		name = "default"
	}
	var selected *dbpkg.StyleBundleRow
	for i := range bundles {
		if bundles[i].Name == name {
			selected = &bundles[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("default style bundle %q not found", name)
	}
	refs := []bundle.FileRef{}
	if err := json.Unmarshal([]byte(selected.FilesJSON), &refs); err != nil {
		return nil, fmt.Errorf("parse style bundle %q files: %w", selected.Name, err)
	}
	files := map[string]string{}
	for _, ref := range refs {
		base := path.Base(ref.File)
		switch base {
		case styleTokensFile, styleDefaultFile:
			files[base] = ref.Hash
		}
	}
	if files[styleTokensFile] == "" || files[styleDefaultFile] == "" {
		return nil, fmt.Errorf("style bundle %q must include tokens.css and default.css", selected.Name)
	}
	return files, nil
}

func (b *Builder) readBlob(ctx context.Context, contentHash string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hashHex, err := bundle.HashHex(contentHash)
	if err != nil {
		return nil, fmt.Errorf("invalid content hash %q: %w", contentHash, err)
	}
	blobPath := b.blobs.Path(hashHex)
	content, err := os.ReadFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob %s not found", hashHex)
		}
		return nil, fmt.Errorf("read blob file %s: %w", blobPath, err)
	}
	return content, nil
}

func (b *Builder) ensureOGBlobs(ctx context.Context, site *model.Site, log *buildLog) (map[string]string, error) {
	pageHashes := make(map[string]string, len(site.Pages))
	for _, pageName := range sortedPageNames(site.Pages) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page := site.Pages[pageName]
		card := ogCardForPage(site, page)
		key := ogimage.CacheKey(card)
		hashHex := hex.EncodeToString(key[:])
		blobPath := b.blobs.Path(hashHex)

		if _, err := os.Stat(blobPath); err == nil {
			pageHashes[pageName] = hashHex
			continue
		} else if err != nil && !os.IsNotExist(err) {
			log.Addf("warning: og image generation failed page=%s: stat cache blob: %v", pageName, err)
			continue
		}

		pngBytes, err := b.generateFn(card)
		if err != nil {
			log.Addf("warning: og image generation failed page=%s: %v", pageName, err)
			continue
		}
		if _, err := b.blobs.Put(ctx, hashHex, pngBytes); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			log.Addf("warning: og image generation failed page=%s: put blob: %v", pageName, err)
			continue
		}
		pageHashes[pageName] = hashHex
	}
	return pageHashes, nil
}

func (b *Builder) preflightOGMaterialization(ctx context.Context, pageHashes map[string]string, probeDir string, log *buildLog) (map[string]string, error) {
	ready := make(map[string]string, len(pageHashes))
	for _, pageName := range sortedStringKeys(pageHashes) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		hashHex := pageHashes[pageName]
		sourcePath := b.blobs.Path(hashHex)
		targetPath := filepath.Join(probeDir, "og", pageName+".png")
		if err := b.materializeOGBlob(sourcePath, targetPath); err != nil {
			log.Addf("warning: og image materialization failed page=%s: %v", pageName, err)
			continue
		}
		ready[pageName] = hashHex
		_ = os.Remove(targetPath)
	}
	_ = os.RemoveAll(probeDir)
	return ready, nil
}

func (b *Builder) materializeOGImages(ctx context.Context, releaseDir string, pageHashes map[string]string, log *buildLog) error {
	if len(pageHashes) == 0 {
		return nil
	}
	ogDir := filepath.Join(releaseDir, "og")
	if err := os.MkdirAll(ogDir, 0o755); err != nil {
		log.Addf("warning: og image materialization failed: create og directory: %v", err)
		return nil
	}
	for _, pageName := range sortedStringKeys(pageHashes) {
		if err := ctx.Err(); err != nil {
			return err
		}
		hashHex := pageHashes[pageName]
		sourcePath := b.blobs.Path(hashHex)
		targetPath := filepath.Join(ogDir, pageName+".png")
		if err := b.materializeOGBlob(sourcePath, targetPath); err != nil {
			log.Addf("warning: og image materialization failed page=%s: %v", pageName, err)
			continue
		}
	}
	return nil
}

// warnLocalhostMetadataURLs scans all page head URL fields for loopback/localhost
// addresses and logs a warning for each one found. These URLs are valid http(s)
// URLs so they pass other validation, but referencing localhost in production
// metadata is always a misconfiguration.
func warnLocalhostMetadataURLs(site *model.Site, log *buildLog) {
	type urlField struct{ field, value string }
	for _, pageName := range sortedPageNames(site.Pages) {
		page := site.Pages[pageName]
		if page.Spec.Head == nil {
			continue
		}
		head := page.Spec.Head
		var fields []urlField
		if head.CanonicalURL != "" {
			fields = append(fields, urlField{"canonicalURL", head.CanonicalURL})
		}
		if head.OpenGraph != nil {
			if head.OpenGraph.URL != "" {
				fields = append(fields, urlField{"openGraph.url", head.OpenGraph.URL})
			}
			if head.OpenGraph.Image != "" {
				fields = append(fields, urlField{"openGraph.image", head.OpenGraph.Image})
			}
		}
		if head.Twitter != nil {
			if head.Twitter.URL != "" {
				fields = append(fields, urlField{"twitter.url", head.Twitter.URL})
			}
			if head.Twitter.Image != "" {
				fields = append(fields, urlField{"twitter.image", head.Twitter.Image})
			}
		}
		for _, f := range fields {
			if isLocalhostURL(f.value) {
				log.Addf("warning: page=%s field=%s contains local host URL %q — update to production URL before promoting", pageName, f.field, f.value)
			}
		}
	}
}

// isLocalhostURL reports whether rawURL has a loopback or local-only host.
func isLocalhostURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || !u.IsAbs() {
		return false
	}
	h := u.Hostname()
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func (b *Builder) injectOGImageMetadata(site *model.Site, pageHashes map[string]string, log *buildLog) {
	for _, pageName := range sortedStringKeys(pageHashes) {
		page := site.Pages[pageName]
		ogImageURL, ok := canonicalOGImageURL(page.Spec.Head, pageName)
		if !ok {
			continue
		}
		// Guard: if the derived OG URL resolves to a local host (i.e. canonicalURL
		// points to localhost), skip injection entirely. Injecting a localhost URL
		// into production metadata would silently break sharing. warnLocalhostMetadataURLs
		// already emits a warning for the canonicalURL itself, so no second warning here.
		if isLocalhostURL(ogImageURL) {
			continue
		}
		if page.Spec.Head.OpenGraph == nil {
			page.Spec.Head.OpenGraph = &model.OpenGraph{}
		}
		if strings.TrimSpace(page.Spec.Head.OpenGraph.Image) == "" {
			page.Spec.Head.OpenGraph.Image = ogImageURL
		} else {
			log.Addf("info: page=%s og card not injected into openGraph.image (already set to %q)", pageName, page.Spec.Head.OpenGraph.Image)
		}
		if page.Spec.Head.Twitter == nil {
			page.Spec.Head.Twitter = &model.TwitterCard{}
		}
		if strings.TrimSpace(page.Spec.Head.Twitter.Image) == "" {
			page.Spec.Head.Twitter.Image = ogImageURL
		} else {
			log.Addf("info: page=%s og card not injected into twitter.image (already set to %q)", pageName, page.Spec.Head.Twitter.Image)
		}
		site.Pages[pageName] = page
	}
}

func ogCardForPage(site *model.Site, page model.Page) ogimage.Card {
	title := strings.TrimSpace(page.Spec.Title)
	description := strings.TrimSpace(page.Spec.Description)
	siteName := strings.TrimSpace(site.Website.Metadata.Name)
	if page.Spec.Head != nil && page.Spec.Head.OpenGraph != nil {
		if v := strings.TrimSpace(page.Spec.Head.OpenGraph.Title); v != "" {
			title = v
		}
		if v := strings.TrimSpace(page.Spec.Head.OpenGraph.Description); v != "" {
			description = v
		}
		if v := strings.TrimSpace(page.Spec.Head.OpenGraph.SiteName); v != "" {
			siteName = v
		}
	}
	// Read accent color from the site's design tokens.
	// Sites define --og-accent: #hexcolor in tokens.css to brand their OG cards.
	accentColor := parseCSSVarColor(site.Styles.TokensCSS, "--og-accent")
	return ogimage.Card{
		Title:       title,
		Description: description,
		SiteName:    siteName,
		AccentColor: accentColor,
	}
}

// parseCSSVarColor returns the first #rgb or #rrggbb hex value assigned to varName
// in the given CSS text, or the empty string if not found.
// The search is boundary-aware: varName must be preceded by whitespace, '{', or ';'
// to avoid partial matches against longer property names.
func parseCSSVarColor(css, varName string) string {
	re := regexp.MustCompile(`(?:^|[\s{;])` + regexp.QuoteMeta(varName) + `\s*:\s*(#[0-9a-fA-F]{6}|#[0-9a-fA-F]{3})`)
	if m := re.FindStringSubmatch(css); len(m) >= 2 {
		return strings.ToLower(m[1])
	}
	return ""
}

func canonicalOGImageURL(head *model.PageHead, pageName string) (string, bool) {
	if head == nil {
		return "", false
	}
	rawCanonical := strings.TrimSpace(head.CanonicalURL)
	if rawCanonical == "" {
		return "", false
	}
	parsed, err := url.Parse(rawCanonical)
	if err != nil || !parsed.IsAbs() {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	parsed.Path = path.Join("/", "og", pageName+".png")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}

func (b *Builder) materializeOGBlob(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create target directory %s: %w", filepath.Dir(targetPath), err)
	}
	linkErr := b.linkFileFn(sourcePath, targetPath)
	if linkErr == nil {
		_ = os.Chmod(targetPath, 0o644)
		return nil
	}
	copyErr := b.copyFileFn(sourcePath, targetPath)
	if copyErr == nil {
		_ = os.Chmod(targetPath, 0o644)
		return nil
	}
	return fmt.Errorf("link blob %s -> %s: %v; copy fallback failed: %w", sourcePath, targetPath, linkErr, copyErr)
}

func (b *Builder) copyOriginalStyles(sourceDir, releaseDir string) error {
	for _, name := range []string{styleTokensFile, styleDefaultFile} {
		src := filepath.Join(sourceDir, "styles", name)
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read source style file %s: %w", src, err)
		}
		if err := writeFile(filepath.Join(releaseDir, "styles", name), content); err != nil {
			return fmt.Errorf("write release style file %s: %w", name, err)
		}
	}
	return nil
}

func (b *Builder) copyOriginalAssets(ctx context.Context, assets []dbpkg.AssetRow, releaseDir string) error {
	for _, row := range assets {
		rel, err := sanitizeRelPath(row.Filename)
		if err != nil {
			return fmt.Errorf("invalid asset filename %q: %w", row.Filename, err)
		}
		content, err := b.readBlob(ctx, row.ContentHash)
		if err != nil {
			return fmt.Errorf("copy release asset %q: %w", row.Filename, err)
		}
		if err := writeFile(filepath.Join(releaseDir, filepath.FromSlash(rel)), content); err != nil {
			return fmt.Errorf("write release asset file %q: %w", row.Filename, err)
		}
	}
	return nil
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

func computeOutputHashes(root string) (map[string]string, error) {
	entries := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".output-hashes.json" {
			return nil
		}
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk release output files: %w", err)
	}
	sort.Strings(entries)
	out := make(map[string]string, len(entries))
	for _, rel := range entries {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read output file %s: %w", rel, err)
		}
		sum := sha256.Sum256(content)
		out[rel] = "sha256:" + hex.EncodeToString(sum[:])
	}
	return out, nil
}

func sanitizeRelPath(p string) (string, error) {
	clean := path.Clean(strings.TrimSpace(strings.ReplaceAll(p, "\\", "/")))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("path must be relative")
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return clean, nil
}

func sanitizeResourceName(name string) (string, error) {
	if err := names.ValidateResourceName(name); err != nil {
		return "", err
	}
	return name, nil
}

func sortedPageNames(pages map[string]model.Page) []string {
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parsePageHeadJSON(pageName, raw string) (*model.PageHead, error) {
	normalized := dbpkg.PageRow{HeadJSON: raw}.HeadJSONOrDefault()
	if normalized == "{}" {
		return nil, nil
	}
	var head model.PageHead
	if err := json.Unmarshal([]byte(normalized), &head); err != nil {
		return nil, fmt.Errorf("parse head json for page %q: %w", pageName, err)
	}
	return &head, nil
}

func parseWebsiteHeadJSON(raw string) (*model.WebsiteHead, error) {
	normalized := dbpkg.WebsiteRow{HeadJSON: raw}.HeadJSONOrDefault()
	if strings.TrimSpace(normalized) == "{}" {
		return nil, nil
	}
	var head model.WebsiteHead
	if err := json.Unmarshal([]byte(normalized), &head); err != nil {
		return nil, fmt.Errorf("parse website head json: %w", err)
	}
	return &head, nil
}

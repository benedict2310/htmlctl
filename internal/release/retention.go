package release

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/blob"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

const retentionPreviewTimestampLayout = "2006-01-02T15:04:05.000000000Z"

var retentionHashHexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

var deleteReleasesByEnvironment = func(ctx context.Context, q *dbpkg.Queries, environmentID int64, releaseIDs []string) (int64, error) {
	return q.DeleteReleasesByEnvironment(ctx, environmentID, releaseIDs)
}

type RetentionOptions struct {
	Keep   int
	DryRun bool
	BlobGC bool
}

type RetentionResult struct {
	Website                 string
	Environment             string
	EnvironmentID           int64
	Keep                    int
	DryRun                  bool
	BlobGC                  bool
	ActiveReleaseID         *string
	RollbackReleaseID       *string
	PreviewPinnedReleaseIDs []string
	RetainedReleaseIDs      []string
	PrunableReleaseIDs      []string
	PrunedReleaseIDs        []string
	MarkedBlobCount         int
	BlobDeleteCandidates    []string
	DeletedBlobHashes       []string
	Warnings                []string
}

type retentionPlan struct {
	website            dbpkg.WebsiteRow
	environment        dbpkg.EnvironmentRow
	activeReleaseID    *string
	rollbackReleaseID  *string
	previewPinnedIDs   []string
	retainedReleaseIDs []string
	prunableReleaseIDs []string
}

type quarantinedRelease struct {
	releaseID      string
	originalPath   string
	quarantinePath string
}

func RunRetention(ctx context.Context, db *sql.DB, blobs *blob.Store, websitesRoot, websiteName, envName string, opts RetentionOptions) (RetentionResult, error) {
	var out RetentionResult

	if db == nil {
		return out, fmt.Errorf("database is required")
	}
	if opts.Keep < 0 {
		return out, fmt.Errorf("keep must be >= 0")
	}
	if strings.TrimSpace(websitesRoot) == "" {
		return out, fmt.Errorf("websites root is required")
	}
	if opts.BlobGC && blobs == nil {
		return out, fmt.Errorf("blob store is required when blob gc is enabled")
	}
	if opts.BlobGC && strings.TrimSpace(blobs.Root()) == "" {
		return out, fmt.Errorf("blob store root is required when blob gc is enabled")
	}

	websiteName = strings.TrimSpace(websiteName)
	envName = strings.TrimSpace(envName)
	if websiteName == "" || envName == "" {
		return out, fmt.Errorf("website and environment are required")
	}

	q := dbpkg.NewQueries(db)
	plan, err := buildRetentionPlan(ctx, q, websiteName, envName, opts.Keep)
	if err != nil {
		return out, err
	}

	out = RetentionResult{
		Website:                 plan.website.Name,
		Environment:             plan.environment.Name,
		EnvironmentID:           plan.environment.ID,
		Keep:                    opts.Keep,
		DryRun:                  opts.DryRun,
		BlobGC:                  opts.BlobGC,
		ActiveReleaseID:         plan.activeReleaseID,
		RollbackReleaseID:       plan.rollbackReleaseID,
		PreviewPinnedReleaseIDs: append([]string(nil), plan.previewPinnedIDs...),
		RetainedReleaseIDs:      append([]string(nil), plan.retainedReleaseIDs...),
		PrunableReleaseIDs:      append([]string(nil), plan.prunableReleaseIDs...),
	}

	if !opts.DryRun && len(plan.prunableReleaseIDs) > 0 {
		quarantined, err := quarantineReleaseDirectories(ctx, websitesRoot, plan.website.Name, plan.environment.Name, plan.prunableReleaseIDs)
		if err != nil {
			return out, err
		}
		prunedIDs := extractReleaseIDs(quarantined)
		if _, err := deleteReleasesByEnvironment(ctx, q, plan.environment.ID, prunedIDs); err != nil {
			if restoreErr := restoreQuarantinedDirectories(quarantined); restoreErr != nil {
				return out, fmt.Errorf("delete release rows: %w (restore failed: %v)", err, restoreErr)
			}
			return out, fmt.Errorf("delete release rows: %w", err)
		}
		out.PrunedReleaseIDs = append(out.PrunedReleaseIDs, prunedIDs...)
		out.Warnings = append(out.Warnings, removeQuarantinedDirectories(quarantined)...)
	}

	if opts.BlobGC {
		markedHashes, err := q.ListReferencedBlobHashes(ctx)
		if err != nil {
			return out, fmt.Errorf("list referenced blob hashes: %w", err)
		}
		out.MarkedBlobCount = len(markedHashes)
		candidates, err := blobDeleteCandidates(ctx, blobs.Root(), markedHashes)
		if err != nil {
			return out, err
		}
		out.BlobDeleteCandidates = candidates
		if !opts.DryRun {
			deleted, err := deleteBlobs(ctx, blobs, candidates)
			out.DeletedBlobHashes = deleted
			if err != nil {
				return out, err
			}
		}
	}

	return out, nil
}

func buildRetentionPlan(ctx context.Context, q *dbpkg.Queries, websiteName, envName string, keep int) (retentionPlan, error) {
	var out retentionPlan

	websiteRow, err := q.GetWebsiteByName(ctx, websiteName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("website %q not found", websiteName)}
		}
		return out, fmt.Errorf("lookup website %q: %w", websiteName, err)
	}
	envRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, envName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("environment %q not found", envName)}
		}
		return out, fmt.Errorf("lookup environment %q: %w", envName, err)
	}
	releases, err := q.ListReleasesByEnvironment(ctx, envRow.ID)
	if err != nil {
		return out, fmt.Errorf("list releases: %w", err)
	}

	activeReleaseID, rollbackReleaseID, err := retentionPinnedHistory(releases, envRow.ActiveReleaseID)
	if err != nil {
		return out, err
	}
	previewRows, err := q.ListReleasePreviewsByEnvironment(ctx, envRow.ID, time.Now().UTC().Format(retentionPreviewTimestampLayout))
	if err != nil {
		return out, fmt.Errorf("list release previews: %w", err)
	}
	previewPinned := uniqueNonEmpty(releaseIDsFromPreviewRows(previewRows))

	pinned := map[string]struct{}{}
	if activeReleaseID != nil {
		pinned[*activeReleaseID] = struct{}{}
	}
	if rollbackReleaseID != nil {
		pinned[*rollbackReleaseID] = struct{}{}
	}
	for _, releaseID := range previewPinned {
		pinned[releaseID] = struct{}{}
	}
	for i := 0; i < keep && i < len(releases); i++ {
		pinned[releases[i].ID] = struct{}{}
	}

	retained := make([]string, 0, len(releases))
	prunable := make([]string, 0, len(releases))
	for _, rel := range releases {
		if _, ok := pinned[rel.ID]; ok {
			retained = append(retained, rel.ID)
			continue
		}
		prunable = append(prunable, rel.ID)
	}

	out = retentionPlan{
		website:            websiteRow,
		environment:        envRow,
		activeReleaseID:    activeReleaseID,
		rollbackReleaseID:  rollbackReleaseID,
		previewPinnedIDs:   orderedIDsFromHistory(releases, previewPinned),
		retainedReleaseIDs: retained,
		prunableReleaseIDs: prunable,
	}
	return out, nil
}

func retentionPinnedHistory(releases []dbpkg.ReleaseRow, activeReleaseID *string) (*string, *string, error) {
	var active *string
	if activeReleaseID != nil {
		trimmed := strings.TrimSpace(*activeReleaseID)
		if trimmed != "" {
			active = &trimmed
		}
	}
	if active == nil {
		return nil, nil, nil
	}

	activeIdx := -1
	for i, rel := range releases {
		if rel.ID == *active {
			activeIdx = i
			break
		}
	}
	if activeIdx == -1 {
		return nil, nil, fmt.Errorf("active release %q was not found in release history", *active)
	}

	var rollback *string
	for i := activeIdx + 1; i < len(releases); i++ {
		if strings.EqualFold(strings.TrimSpace(releases[i].Status), "failed") {
			continue
		}
		id := releases[i].ID
		rollback = &id
		break
	}
	return active, rollback, nil
}

func releaseIDsFromPreviewRows(rows []dbpkg.ReleasePreviewRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ReleaseID)
	}
	return out
}

func orderedIDsFromHistory(releases []dbpkg.ReleaseRow, ids []string) []string {
	keep := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		keep[id] = struct{}{}
	}
	out := make([]string, 0, len(keep))
	for _, rel := range releases {
		if _, ok := keep[rel.ID]; ok {
			out = append(out, rel.ID)
			delete(keep, rel.ID)
		}
	}
	if len(keep) > 0 {
		remaining := make([]string, 0, len(keep))
		for id := range keep {
			remaining = append(remaining, id)
		}
		sort.Strings(remaining)
		out = append(out, remaining...)
	}
	return out
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func quarantineReleaseDirectories(ctx context.Context, websitesRoot, website, env string, releaseIDs []string) ([]quarantinedRelease, error) {
	releasesRoot := filepath.Join(websitesRoot, website, "envs", env, "releases")
	quarantined := make([]quarantinedRelease, 0, len(releaseIDs))
	for _, releaseID := range releaseIDs {
		if err := ctx.Err(); err != nil {
			_ = restoreQuarantinedDirectories(quarantined)
			return nil, err
		}
		releaseID = strings.TrimSpace(releaseID)
		if releaseID == "" {
			continue
		}
		originalPath := filepath.Join(releasesRoot, releaseID)
		info, err := os.Stat(originalPath)
		if err != nil {
			_ = restoreQuarantinedDirectories(quarantined)
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("release directory %s is missing", originalPath)
			}
			return nil, fmt.Errorf("stat release directory %s: %w", originalPath, err)
		}
		if !info.IsDir() {
			_ = restoreQuarantinedDirectories(quarantined)
			return nil, fmt.Errorf("release path is not a directory: %s", originalPath)
		}

		quarantinePath := filepath.Join(releasesRoot, "."+releaseID+".quarantine")
		if _, err := os.Stat(quarantinePath); err == nil {
			_ = restoreQuarantinedDirectories(quarantined)
			return nil, fmt.Errorf("quarantine path already exists: %s", quarantinePath)
		} else if !os.IsNotExist(err) {
			_ = restoreQuarantinedDirectories(quarantined)
			return nil, fmt.Errorf("stat quarantine path %s: %w", quarantinePath, err)
		}
		if err := os.Rename(originalPath, quarantinePath); err != nil {
			_ = restoreQuarantinedDirectories(quarantined)
			return nil, fmt.Errorf("quarantine release directory %s: %w", originalPath, err)
		}
		quarantined = append(quarantined, quarantinedRelease{
			releaseID:      releaseID,
			originalPath:   originalPath,
			quarantinePath: quarantinePath,
		})
	}
	return quarantined, nil
}

func restoreQuarantinedDirectories(quarantined []quarantinedRelease) error {
	var restoreErr error
	for i := len(quarantined) - 1; i >= 0; i-- {
		item := quarantined[i]
		if err := os.Rename(item.quarantinePath, item.originalPath); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("restore %s: %w", item.releaseID, err))
		}
	}
	return restoreErr
}

func removeQuarantinedDirectories(quarantined []quarantinedRelease) []string {
	warnings := []string{}
	for _, item := range quarantined {
		if err := os.RemoveAll(item.quarantinePath); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to remove quarantined release directory %s: %v", item.releaseID, err))
		}
	}
	return warnings
}

func extractReleaseIDs(quarantined []quarantinedRelease) []string {
	out := make([]string, 0, len(quarantined))
	for _, item := range quarantined {
		out = append(out, item.releaseID)
	}
	return out
}

func blobDeleteCandidates(ctx context.Context, blobRoot string, markedHashes []string) ([]string, error) {
	marked := map[string]struct{}{}
	for _, hashHex := range markedHashes {
		marked[strings.TrimSpace(hashHex)] = struct{}{}
	}

	entries, err := os.ReadDir(blobRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read blob root %s: %w", blobRoot, err)
	}

	candidates := []string{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		if !retentionHashHexPattern.MatchString(name) {
			continue
		}
		if !dirEntryIsRegularFile(entry) {
			continue
		}
		if _, ok := marked[name]; ok {
			continue
		}
		candidates = append(candidates, name)
	}
	sort.Strings(candidates)
	return candidates, nil
}

func deleteBlobs(ctx context.Context, blobs *blob.Store, hashes []string) ([]string, error) {
	deleted := make([]string, 0, len(hashes))
	for _, hashHex := range hashes {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		path := blobs.Path(hashHex)
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return deleted, fmt.Errorf("delete blob %s: %w", hashHex, err)
		}
		deleted = append(deleted, hashHex)
	}
	return deleted, nil
}

func dirEntryIsRegularFile(entry os.DirEntry) bool {
	mode := entry.Type()
	if mode.IsRegular() {
		return true
	}
	if mode != 0 {
		return false
	}
	info, err := entry.Info()
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

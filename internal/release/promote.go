package release

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

var (
	ErrPromotionSourceTargetMatch = errors.New("source and target environments must be different")
	ErrPromotionSourceNoActive    = errors.New("source environment has no active release")
	linkFile                      = os.Link
)

type HashMismatchError struct {
	Reason string
}

func (e *HashMismatchError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "promotion hash verification failed"
	}
	return fmt.Sprintf("promotion hash verification failed: %s", e.Reason)
}

type PromoteResult struct {
	SourceEnvironmentID int64
	TargetEnvironmentID int64
	SourceReleaseID     string
	ReleaseID           string
	FileCount           int
	Hash                string
	Strategy            string
}

func Promote(ctx context.Context, db *sql.DB, websitesRoot, websiteName, sourceEnvName, targetEnvName string) (out PromoteResult, err error) {
	if db == nil {
		return out, fmt.Errorf("database is required")
	}
	websiteName = strings.TrimSpace(websiteName)
	sourceEnvName = strings.TrimSpace(sourceEnvName)
	targetEnvName = strings.TrimSpace(targetEnvName)
	if websiteName == "" || sourceEnvName == "" || targetEnvName == "" {
		return out, fmt.Errorf("website, source environment, and target environment are required")
	}
	if sourceEnvName == targetEnvName {
		return out, ErrPromotionSourceTargetMatch
	}

	q := dbpkg.NewQueries(db)
	websiteRow, err := q.GetWebsiteByName(ctx, websiteName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("website %q not found", websiteName)}
		}
		return out, fmt.Errorf("lookup website %q: %w", websiteName, err)
	}
	sourceEnvRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, sourceEnvName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("environment %q not found", sourceEnvName)}
		}
		return out, fmt.Errorf("lookup source environment %q: %w", sourceEnvName, err)
	}
	targetEnvRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, targetEnvName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, &NotFoundError{msg: fmt.Sprintf("environment %q not found", targetEnvName)}
		}
		return out, fmt.Errorf("lookup target environment %q: %w", targetEnvName, err)
	}
	if sourceEnvRow.ActiveReleaseID == nil || strings.TrimSpace(*sourceEnvRow.ActiveReleaseID) == "" {
		return out, ErrPromotionSourceNoActive
	}

	sourceReleaseID := strings.TrimSpace(*sourceEnvRow.ActiveReleaseID)
	sourceReleaseRow, err := q.GetReleaseByID(ctx, sourceReleaseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, fmt.Errorf("source release %q not found", sourceReleaseID)
		}
		return out, fmt.Errorf("lookup source release %q: %w", sourceReleaseID, err)
	}
	if sourceReleaseRow.EnvironmentID != sourceEnvRow.ID {
		return out, fmt.Errorf("active source release %q does not belong to environment %q", sourceReleaseID, sourceEnvName)
	}

	sourceReleaseDir := filepath.Join(websitesRoot, websiteRow.Name, "envs", sourceEnvRow.Name, "releases", sourceReleaseID)
	sourceInfo, err := os.Stat(sourceReleaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, fmt.Errorf("source release directory %s does not exist", sourceReleaseDir)
		}
		return out, fmt.Errorf("stat source release directory %s: %w", sourceReleaseDir, err)
	}
	if !sourceInfo.IsDir() {
		return out, fmt.Errorf("source release path is not a directory: %s", sourceReleaseDir)
	}

	releaseID, err := NewReleaseID(time.Now().UTC())
	if err != nil {
		return out, err
	}
	out.SourceEnvironmentID = sourceEnvRow.ID
	out.TargetEnvironmentID = targetEnvRow.ID
	out.SourceReleaseID = sourceReleaseID
	out.ReleaseID = releaseID

	targetEnvDir := filepath.Join(websitesRoot, websiteRow.Name, "envs", targetEnvRow.Name)
	targetReleasesRoot := filepath.Join(targetEnvDir, "releases")
	targetTmpDir := filepath.Join(targetReleasesRoot, releaseID+".tmp")
	targetFinalDir := filepath.Join(targetReleasesRoot, releaseID)

	if err := os.MkdirAll(targetReleasesRoot, 0o755); err != nil {
		return out, fmt.Errorf("create target releases directory %s: %w", targetReleasesRoot, err)
	}
	if _, err := os.Stat(targetFinalDir); err == nil {
		return out, fmt.Errorf("target release directory already exists: %s", targetFinalDir)
	} else if err != nil && !os.IsNotExist(err) {
		return out, fmt.Errorf("stat target release directory %s: %w", targetFinalDir, err)
	}
	_ = os.RemoveAll(targetTmpDir)

	prevTarget, hadPrevTarget, err := ReadCurrentSymlinkTarget(targetEnvDir)
	if err != nil {
		return out, err
	}
	switchedCurrent := false
	finalizedTargetDir := false
	defer func() {
		if err == nil {
			return
		}
		_ = os.RemoveAll(targetTmpDir)
		if switchedCurrent {
			restoreTarget := ""
			if hadPrevTarget {
				restoreTarget = prevTarget
			}
			_ = SetCurrentSymlinkTarget(targetEnvDir, restoreTarget)
		}
		if finalizedTargetDir {
			_ = os.RemoveAll(targetFinalDir)
		}
	}()

	linkedCount, copiedCount, fileCount, err := copyReleaseContent(sourceReleaseDir, targetTmpDir)
	if err != nil {
		return out, err
	}
	if copiedCount == 0 {
		out.Strategy = "hardlink"
	} else if linkedCount == 0 {
		out.Strategy = "copy"
	} else {
		out.Strategy = "hardlink+copy"
	}
	out.FileCount = fileCount

	sourceHashes, err := loadSourcePromotionHashes(sourceReleaseRow.OutputHashes)
	if err != nil {
		return out, fmt.Errorf("load source release hashes: %w", err)
	}
	if len(sourceHashes) == 0 {
		sourceHashes, err = computePromotionHashes(sourceReleaseDir)
		if err != nil {
			return out, err
		}
	}
	targetHashes, err := computePromotionHashes(targetTmpDir)
	if err != nil {
		return out, err
	}
	if mismatch := comparePromotionHashes(sourceHashes, targetHashes); mismatch != "" {
		return out, &HashMismatchError{Reason: mismatch}
	}
	out.Hash = promotionManifestDigest(targetHashes)

	manifestJSON, err := promoteManifestJSON(sourceReleaseRow.ManifestJSON, sourceEnvName, targetEnvName, sourceReleaseID)
	if err != nil {
		return out, err
	}
	buildLog := promoteBuildLog(sourceReleaseID, sourceEnvName, targetEnvName)
	if err := writeFile(filepath.Join(targetTmpDir, ".manifest.json"), []byte(manifestJSON)); err != nil {
		return out, fmt.Errorf("write target manifest metadata: %w", err)
	}
	if err := writeFile(filepath.Join(targetTmpDir, ".build-log.txt"), []byte(buildLog)); err != nil {
		return out, fmt.Errorf("write target build log metadata: %w", err)
	}

	finalOutputHashes, err := computeOutputHashes(targetTmpDir)
	if err != nil {
		return out, err
	}
	finalOutputHashesJSON, err := json.MarshalIndent(finalOutputHashes, "", "  ")
	if err != nil {
		return out, fmt.Errorf("marshal target output hashes: %w", err)
	}
	if err := writeFile(filepath.Join(targetTmpDir, ".output-hashes.json"), finalOutputHashesJSON); err != nil {
		return out, fmt.Errorf("write target output hashes metadata: %w", err)
	}

	if err := os.Rename(targetTmpDir, targetFinalDir); err != nil {
		return out, fmt.Errorf("finalize promoted release directory: %w", err)
	}
	finalizedTargetDir = true

	if err := SwitchCurrentSymlink(targetEnvDir, releaseID); err != nil {
		return out, err
	}
	switchedCurrent = true

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("begin promotion transaction: %w", err)
	}
	txDone := false
	defer func() {
		if !txDone {
			_ = tx.Rollback()
		}
	}()
	txQ := dbpkg.NewQueries(tx)
	if err := txQ.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            releaseID,
		EnvironmentID: targetEnvRow.ID,
		ManifestJSON:  manifestJSON,
		OutputHashes:  string(finalOutputHashesJSON),
		BuildLog:      buildLog,
		Status:        "active",
	}); err != nil {
		return out, fmt.Errorf("insert promoted release row: %w", err)
	}
	if err := txQ.UpdateEnvironmentActiveRelease(ctx, targetEnvRow.ID, &releaseID); err != nil {
		return out, fmt.Errorf("set target environment active release: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("commit promotion transaction: %w", err)
	}
	txDone = true
	switchedCurrent = false

	return out, nil
}

func copyReleaseContent(sourceRoot, targetRoot string) (linkedCount int, copiedCount int, fileCount int, err error) {
	err = filepath.WalkDir(sourceRoot, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, current)
		if err != nil {
			return fmt.Errorf("resolve relative path for %s: %w", current, err)
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if rel == "." {
				return nil
			}
			return os.MkdirAll(filepath.Join(targetRoot, filepath.FromSlash(rel)), 0o755)
		}

		if isReleaseMetadataFile(rel) {
			return nil
		}

		sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		targetPath := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create target directory for %s: %w", rel, err)
		}

		fileCount++
		if d.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(sourcePath)
			if err != nil {
				return fmt.Errorf("read source symlink %s: %w", rel, err)
			}
			if err := os.Symlink(target, targetPath); err != nil {
				return fmt.Errorf("create target symlink %s: %w", rel, err)
			}
			copiedCount++
			return nil
		}

		if err := linkFile(sourcePath, targetPath); err == nil {
			linkedCount++
			return nil
		}
		if err := copyFile(sourcePath, targetPath); err != nil {
			return fmt.Errorf("copy target file %s: %w", rel, err)
		}
		copiedCount++
		return nil
	})
	return linkedCount, copiedCount, fileCount, err
}

func copyFile(sourcePath, targetPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source file %s: %w", sourcePath, err)
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", sourcePath, err)
	}
	defer in.Close()
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("open target file %s: %w", targetPath, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy bytes %s -> %s: %w", sourcePath, targetPath, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close target file %s: %w", targetPath, err)
	}
	return nil
}

func isReleaseMetadataFile(relPath string) bool {
	switch filepath.ToSlash(relPath) {
	case ".manifest.json", ".build-log.txt", ".output-hashes.json":
		return true
	default:
		return false
	}
}

func loadSourcePromotionHashes(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	parsed := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	filtered := map[string]string{}
	for pathValue, hashValue := range parsed {
		pathValue = filepath.ToSlash(strings.TrimSpace(pathValue))
		if pathValue == "" || isReleaseMetadataFile(pathValue) {
			continue
		}
		filtered[pathValue] = strings.TrimSpace(hashValue)
	}
	return filtered, nil
}

func computePromotionHashes(root string) (map[string]string, error) {
	entries := []string{}
	err := filepath.WalkDir(root, func(pathValue string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, pathValue)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isReleaseMetadataFile(rel) {
			return nil
		}
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk promotion files: %w", err)
	}
	sort.Strings(entries)

	out := map[string]string{}
	for _, rel := range entries {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read promotion file %s: %w", rel, err)
		}
		sum := sha256.Sum256(content)
		out[rel] = "sha256:" + hex.EncodeToString(sum[:])
	}
	return out, nil
}

func comparePromotionHashes(source, target map[string]string) string {
	for pathValue, sourceHash := range source {
		targetHash, ok := target[pathValue]
		if !ok {
			return fmt.Sprintf("target is missing file %s", pathValue)
		}
		if targetHash != sourceHash {
			return fmt.Sprintf("hash mismatch for %s", pathValue)
		}
	}
	for pathValue := range target {
		if _, ok := source[pathValue]; !ok {
			return fmt.Sprintf("target has unexpected file %s", pathValue)
		}
	}
	return ""
}

func promotionManifestDigest(hashes map[string]string) string {
	paths := make([]string, 0, len(hashes))
	for pathValue := range hashes {
		paths = append(paths, pathValue)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, pathValue := range paths {
		_, _ = h.Write([]byte(pathValue))
		_, _ = h.Write([]byte{'\n'})
		_, _ = h.Write([]byte(hashes[pathValue]))
		_, _ = h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func promoteManifestJSON(sourceManifestJSON, sourceEnvName, targetEnvName, sourceReleaseID string) (string, error) {
	manifest := map[string]any{}
	if strings.TrimSpace(sourceManifestJSON) != "" {
		if err := json.Unmarshal([]byte(sourceManifestJSON), &manifest); err != nil {
			return "", fmt.Errorf("parse source manifest metadata: %w", err)
		}
	}
	if manifest == nil {
		manifest = map[string]any{}
	}
	manifest["environment"] = targetEnvName
	manifest["generatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	manifest["sourceReleaseId"] = sourceReleaseID
	manifest["sourceEnv"] = sourceEnvName

	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal promoted manifest metadata: %w", err)
	}
	return string(b), nil
}

func promoteBuildLog(sourceReleaseID, sourceEnvName, targetEnvName string) string {
	return fmt.Sprintf(
		"%s promoted release %s from %s to %s\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		sourceReleaseID,
		sourceEnvName,
		targetEnvName,
	)
}

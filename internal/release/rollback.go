package release

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

var ErrNoPreviousRelease = errors.New("no previous release available")

type MissingReleaseDirError struct {
	ReleaseID string
	Path      string
}

func (e *MissingReleaseDirError) Error() string {
	return fmt.Sprintf("rollback target release directory is missing for release %s at %s", e.ReleaseID, e.Path)
}

type RollbackResult struct {
	EnvironmentID int64
	FromReleaseID string
	ToReleaseID   string
}

func Rollback(ctx context.Context, db *sql.DB, websitesRoot, websiteName, envName string) (RollbackResult, error) {
	var out RollbackResult

	if db == nil {
		return out, fmt.Errorf("database is required")
	}
	websiteName = strings.TrimSpace(websiteName)
	envName = strings.TrimSpace(envName)
	if websiteName == "" || envName == "" {
		return out, fmt.Errorf("website and environment are required")
	}

	q := dbpkg.NewQueries(db)
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
	if envRow.ActiveReleaseID == nil || strings.TrimSpace(*envRow.ActiveReleaseID) == "" {
		return out, ErrNoPreviousRelease
	}
	activeReleaseID := strings.TrimSpace(*envRow.ActiveReleaseID)

	releases, err := q.ListReleasesByEnvironment(ctx, envRow.ID)
	if err != nil {
		return out, fmt.Errorf("list releases: %w", err)
	}
	activeIdx := -1
	for i, rel := range releases {
		if rel.ID == activeReleaseID {
			activeIdx = i
			break
		}
	}
	if activeIdx == -1 {
		return out, fmt.Errorf("active release %q was not found in release history", activeReleaseID)
	}

	targetReleaseID := ""
	for i := activeIdx + 1; i < len(releases); i++ {
		if strings.EqualFold(strings.TrimSpace(releases[i].Status), "failed") {
			continue
		}
		targetReleaseID = releases[i].ID
		break
	}
	if targetReleaseID == "" {
		return out, ErrNoPreviousRelease
	}
	targetReleaseDir := filepath.Join(websitesRoot, websiteRow.Name, "envs", envRow.Name, "releases", targetReleaseID)
	targetInfo, err := os.Stat(targetReleaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, &MissingReleaseDirError{
				ReleaseID: targetReleaseID,
				Path:      targetReleaseDir,
			}
		}
		return out, fmt.Errorf("stat rollback target release directory: %w", err)
	}
	if !targetInfo.IsDir() {
		return out, fmt.Errorf("rollback target release path is not a directory: %s", targetReleaseDir)
	}

	envDir := filepath.Join(websitesRoot, websiteRow.Name, "envs", envRow.Name)
	prevTarget, hadPrevTarget, err := ReadCurrentSymlinkTarget(envDir)
	if err != nil {
		return out, err
	}
	switchedCurrent := false
	defer func() {
		if err == nil || !switchedCurrent {
			return
		}
		restoreTarget := ""
		if hadPrevTarget {
			restoreTarget = prevTarget
		}
		_ = SetCurrentSymlinkTarget(envDir, restoreTarget)
	}()

	if err = SwitchCurrentSymlink(envDir, targetReleaseID); err != nil {
		return out, err
	}
	switchedCurrent = true

	if err = q.UpdateEnvironmentActiveRelease(ctx, envRow.ID, &targetReleaseID); err != nil {
		return out, fmt.Errorf("update environment active release: %w", err)
	}
	switchedCurrent = false

	out = RollbackResult{
		EnvironmentID: envRow.ID,
		FromReleaseID: activeReleaseID,
		ToReleaseID:   targetReleaseID,
	}
	return out, nil
}

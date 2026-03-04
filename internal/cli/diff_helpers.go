package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benedict2310/htmlctl/internal/bundle"
	"github.com/benedict2310/htmlctl/internal/client"
	diffpkg "github.com/benedict2310/htmlctl/internal/diff"
)

func computeDesiredStateDiff(ctx context.Context, api *client.APIClient, website, environment, from string) (diffpkg.Report, error) {
	sourceRoot, err := diffSourceRoot(from)
	if err != nil {
		return diffpkg.Report{}, fmt.Errorf("local validation failed: %w", err)
	}
	_, localManifest, err := bundle.BuildTarFromDir(sourceRoot, website)
	if err != nil {
		return diffpkg.Report{}, fmt.Errorf("local validation failed: %w", err)
	}

	remoteManifest, err := api.GetDesiredStateManifest(ctx, website, environment)
	if err != nil {
		return diffpkg.Report{}, err
	}

	localFiles := make([]diffpkg.FileRecord, 0, len(localManifest.Resources))
	for _, resource := range localManifest.Resources {
		for _, ref := range resource.FileEntries() {
			localFiles = append(localFiles, diffpkg.FileRecord{
				Path: ref.File,
				Hash: ref.Hash,
			})
		}
	}
	remoteFiles := make([]diffpkg.FileRecord, 0, len(remoteManifest.Files))
	for _, file := range remoteManifest.Files {
		remoteFiles = append(remoteFiles, diffpkg.FileRecord{
			Path: file.Path,
			Hash: file.Hash,
		})
	}

	result, err := diffpkg.Compute(localFiles, remoteFiles)
	if err != nil {
		return diffpkg.Report{}, err
	}
	return diffpkg.Report{
		Website:     website,
		Environment: environment,
		Result:      result,
	}, nil
}

func diffSourceRoot(from string) (string, error) {
	info, err := os.Stat(from)
	if err != nil {
		return "", fmt.Errorf("stat source path %s: %w", from, err)
	}
	if info.IsDir() {
		return from, nil
	}
	abs, err := filepath.Abs(from)
	if err != nil {
		return "", fmt.Errorf("resolve source path: %w", err)
	}
	current := filepath.Dir(abs)
	for {
		if _, err := os.Stat(filepath.Join(current, "website.yaml")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("source file %s is not inside a site root containing website.yaml", from)
}

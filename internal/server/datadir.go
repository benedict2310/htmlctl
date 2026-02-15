package server

import (
	"fmt"
	"os"
	"path/filepath"
)

type DataPaths struct {
	RootDir      string
	DBPath       string
	BlobsSHA256  string
	WebsitesRoot string
}

func InitDataDir(root string) (DataPaths, error) {
	paths := DataPaths{
		RootDir:      root,
		DBPath:       filepath.Join(root, "db.sqlite"),
		BlobsSHA256:  filepath.Join(root, "blobs", "sha256"),
		WebsitesRoot: filepath.Join(root, "websites"),
	}

	if err := os.MkdirAll(paths.BlobsSHA256, 0o755); err != nil {
		return paths, fmt.Errorf("create blobs directory %s: %w", paths.BlobsSHA256, err)
	}
	if err := os.MkdirAll(paths.WebsitesRoot, 0o755); err != nil {
		return paths, fmt.Errorf("create websites directory %s: %w", paths.WebsitesRoot, err)
	}

	db, err := os.OpenFile(paths.DBPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return paths, fmt.Errorf("create/open sqlite file %s: %w", paths.DBPath, err)
	}
	if err := db.Close(); err != nil {
		return paths, fmt.Errorf("close sqlite file %s: %w", paths.DBPath, err)
	}

	return paths, nil
}

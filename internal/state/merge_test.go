package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/benedict2310/htmlctl/internal/blob"
	"github.com/benedict2310/htmlctl/internal/bundle"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestApplyRejectsInvalidComponentAndPageNames(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		resourceKind string
		resourceName string
		filePath     string
		content      []byte
	}{
		{
			name:         "invalid component name",
			resourceKind: "Component",
			resourceName: "../evil",
			filePath:     "components/header.html",
			content:      []byte("<section id=\"header\">Header</section>"),
		},
		{
			name:         "invalid page name",
			resourceKind: "Page",
			resourceName: "home\nadmin",
			filePath:     "pages/index.page.yaml",
			content:      []byte("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: Home\n"),
		},
		{
			name:         "invalid stylebundle name",
			resourceKind: "StyleBundle",
			resourceName: "../../evil",
			filePath:     "styles/default.css",
			content:      []byte("body { margin: 0; }"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
			defer db.Close()

			blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
			applier, err := NewApplier(db, blobStore)
			if err != nil {
				t.Fatalf("NewApplier() error = %v", err)
			}

			manifest := bundle.Manifest{
				Mode:    bundle.ApplyModePartial,
				Website: "futurelab",
				Resources: []bundle.Resource{
					{
						Kind: tc.resourceKind,
						Name: tc.resourceName,
						File: tc.filePath,
						Hash: "sha256:" + sha256Hex(tc.content),
					},
				},
			}
			b := bundle.Bundle{
				Manifest: manifest,
				Files: map[string][]byte{
					tc.filePath: tc.content,
				},
			}

			_, err = applier.Apply(ctx, "futurelab", "staging", b, false)
			if err == nil {
				t.Fatalf("Apply() expected error")
			}
			var badRequestErr *BadRequestError
			if !errors.As(err, &badRequestErr) {
				t.Fatalf("Apply() expected BadRequestError, got %T (%v)", err, err)
			}

			q := dbpkg.NewQueries(db)
			_, err = q.GetWebsiteByName(ctx, "futurelab")
			if !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("expected no committed website row after failed apply, got err=%v", err)
			}
		})
	}
}

func openStateTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := dbpkg.Open(dbpkg.DefaultOptions(path))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := dbpkg.RunMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return db
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

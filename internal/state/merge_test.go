package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
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

func TestApplyPersistsPageHeadMetadata(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	pageYAML := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout: []
  head:
    canonicalURL: https://futurelab.studio/
    meta:
      robots: index,follow
    openGraph:
      title: Futurelab
    twitter:
      card: summary
    jsonLD:
      - id: website
        payload:
          "@context": https://schema.org
          "@type": WebSite
          name: Futurelab
`)

	manifest := bundle.Manifest{
		Mode:    bundle.ApplyModePartial,
		Website: "futurelab",
		Resources: []bundle.Resource{
			{
				Kind: "Page",
				Name: "index",
				File: "pages/index.page.yaml",
				Hash: "sha256:" + sha256Hex(pageYAML),
			},
		},
	}
	b := bundle.Bundle{
		Manifest: manifest,
		Files: map[string][]byte{
			"pages/index.page.yaml": pageYAML,
		},
	}

	if _, err := applier.Apply(ctx, "futurelab", "staging", b, false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	q := dbpkg.NewQueries(db)
	website, err := q.GetWebsiteByName(ctx, "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	pages, err := q.ListPagesByWebsite(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListPagesByWebsite() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page row, got %d", len(pages))
	}
	if strings.TrimSpace(pages[0].HeadJSON) == "" || pages[0].HeadJSON == "{}" {
		t.Fatalf("expected persisted head_json, got %q", pages[0].HeadJSON)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(pages[0].HeadJSON), &got); err != nil {
		t.Fatalf("unmarshal head_json: %v", err)
	}
	if got["canonicalURL"] != "https://futurelab.studio/" {
		t.Fatalf("unexpected canonicalURL in head_json: %#v", got["canonicalURL"])
	}
}

func TestApplyDetectsHeadOnlyChanges(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db := openStateTestDB(t, filepath.Join(dataDir, "db.sqlite"))
	defer db.Close()

	blobStore := blob.NewStore(filepath.Join(dataDir, "blobs", "sha256"))
	applier, err := NewApplier(db, blobStore)
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}

	fixedHash := "sha256:" + strings.Repeat("a", 64)
	initial := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout: []
  head:
    canonicalURL: https://foo.example/
`)
	updated := []byte(`apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Home
  description: Home page
  layout: []
  head:
    canonicalURL: https://bar.example/
`)

	apply := func(content []byte) (ApplyResult, error) {
		return applier.Apply(ctx, "futurelab", "staging", bundle.Bundle{
			Manifest: bundle.Manifest{
				Mode:    bundle.ApplyModePartial,
				Website: "futurelab",
				Resources: []bundle.Resource{
					{
						Kind: "Page",
						Name: "index",
						File: "pages/index.page.yaml",
						Hash: fixedHash,
					},
				},
			},
			Files: map[string][]byte{
				"pages/index.page.yaml": content,
			},
		}, false)
	}

	first, err := apply(initial)
	if err != nil {
		t.Fatalf("first Apply() error = %v", err)
	}
	if first.Changes.Created != 1 || first.Changes.Updated != 0 {
		t.Fatalf("unexpected first apply changes: %#v", first.Changes)
	}

	second, err := apply(updated)
	if err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if second.Changes.Updated != 1 {
		t.Fatalf("expected head-only change to count as updated=1, got %#v", second.Changes)
	}

	q := dbpkg.NewQueries(db)
	website, err := q.GetWebsiteByName(ctx, "futurelab")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	pages, err := q.ListPagesByWebsite(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListPagesByWebsite() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page row, got %d", len(pages))
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(pages[0].HeadJSON), &got); err != nil {
		t.Fatalf("unmarshal updated head_json: %v", err)
	}
	if got["canonicalURL"] != "https://bar.example/" {
		t.Fatalf("expected updated canonicalURL, got %#v", got["canonicalURL"])
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

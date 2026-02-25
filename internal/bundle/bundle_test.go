package bundle

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestReadTarValidBundle(t *testing.T) {
	component := []byte("<section id=\"header\">ok</section>")
	hash := sha256String(component)
	manifest := fmt.Sprintf(`{
		"apiVersion":"htmlctl.dev/v1",
		"kind":"Bundle",
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Component","name":"header","file":"components/header.html","hash":"sha256:%s"}
		]
	}`, hash)
	body := tarBody(t, map[string][]byte{
		"manifest.json":          []byte(manifest),
		"components/header.html": component,
		"components/unused.html": []byte("<div>x</div>"),
	})
	b, err := ReadTar(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ReadTar() error = %v", err)
	}
	if len(b.Files) != 2 {
		t.Fatalf("expected 2 non-manifest files, got %d", len(b.Files))
	}
	if len(b.ExtraFiles) != 1 || b.ExtraFiles[0] != "components/unused.html" {
		t.Fatalf("unexpected extra files: %#v", b.ExtraFiles)
	}
}

func TestReadTarMissingManifest(t *testing.T) {
	body := tarBody(t, map[string][]byte{"components/header.html": []byte("x")})
	if _, err := ReadTar(bytes.NewReader(body)); err == nil || !strings.Contains(err.Error(), "manifest.json") {
		t.Fatalf("expected missing manifest error, got %v", err)
	}
}

func TestReadTarHashMismatch(t *testing.T) {
	manifest := `{
		"apiVersion":"htmlctl.dev/v1",
		"kind":"Bundle",
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Component","name":"header","file":"components/header.html","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
		]
	}`
	body := tarBody(t, map[string][]byte{
		"manifest.json":          []byte(manifest),
		"components/header.html": []byte("different"),
	})
	_, err := ReadTar(bytes.NewReader(body))
	if err == nil {
		t.Fatalf("expected hash mismatch error")
	}
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T (%v)", err, err)
	}
}

func tarBody(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("write content %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	return buf.Bytes()
}

func sha256String(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

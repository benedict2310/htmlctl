package bundle

import "testing"

func TestParseManifestValid(t *testing.T) {
	data := []byte(`{
		"apiVersion":"htmlctl.dev/v1",
		"kind":"Bundle",
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Component","name":"header","file":"components/header.html","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{"kind":"StyleBundle","name":"default","files":[
				{"file":"styles/tokens.css","hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
				{"file":"styles/default.css","hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
			]}
		]
	}`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.Mode != ApplyModePartial {
		t.Fatalf("unexpected mode %q", m.Mode)
	}
	if len(m.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(m.Resources))
	}
}

func TestParseManifestInvalidMode(t *testing.T) {
	data := []byte(`{"mode":"bad","website":"sample","resources":[{"kind":"Component","name":"x","file":"components/x.html","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected invalid mode error")
	}
}

func TestHashHexValidation(t *testing.T) {
	if _, err := HashHex("sha256:not-hex"); err == nil {
		t.Fatalf("expected invalid hash error")
	}
	if got, err := HashHex("sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"); err != nil || got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected normalized hash got=%q err=%v", got, err)
	}
}

func TestParseManifestAssetNameMustMatchFile(t *testing.T) {
	data := []byte(`{
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Asset","name":"logo.svg","file":"assets/logo.svg","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
		]
	}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected asset name/file mismatch error")
	}
}

func TestParseManifestAssetMustHaveExactlyOneFile(t *testing.T) {
	data := []byte(`{
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Asset","name":"assets/logo.svg","files":[
				{"file":"assets/logo.svg","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				{"file":"assets/logo@2x.svg","hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
			]}
		]
	}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected multiple asset file error")
	}
}

func TestParseManifestComponentMustHaveExactlyOneFile(t *testing.T) {
	data := []byte(`{
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Component","name":"header","files":[
				{"file":"components/header.html","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				{"file":"components/header.css","hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
			]}
		]
	}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected multiple component file error")
	}
}

func TestParseManifestDeletedAssetRequiresFile(t *testing.T) {
	data := []byte(`{
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Asset","name":"assets/logo.svg","deleted":true}
		]
	}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected deleted asset file requirement error")
	}
}

func TestParseManifestRejectsDuplicateResourceKindName(t *testing.T) {
	data := []byte(`{
		"mode":"partial",
		"website":"sample",
		"resources":[
			{"kind":"Component","name":"header","file":"components/header.html","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{"kind":"component","name":"header","file":"components/header-v2.html","hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
		]
	}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected duplicate resource validation error")
	}
}

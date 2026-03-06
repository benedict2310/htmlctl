package extensionspec

import (
	"path/filepath"
	"runtime"
	"testing"
)

func boolPtr(v bool) *bool {
	return &v
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func TestLoadNewsletterManifest(t *testing.T) {
	root := repoRoot(t)
	manifestPath := filepath.Join(root, "extensions", "newsletter", "extension.yaml")

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", manifestPath, err)
	}
	if manifest.Metadata.Name != "newsletter" {
		t.Fatalf("expected name newsletter, got %q", manifest.Metadata.Name)
	}
	if manifest.Spec.Compatibility.MinHTMLCTL == "" || manifest.Spec.Compatibility.MinHTMLSERVD == "" {
		t.Fatalf("expected compatibility fields to be populated")
	}
}

func TestValidateAllManifests(t *testing.T) {
	if err := ValidateAllManifests(repoRoot(t)); err != nil {
		t.Fatalf("ValidateAllManifests() error = %v", err)
	}
}

func TestValidateManifestRejectsInvalidBackendPath(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected error for invalid backend path")
	}
}

func TestValidateManifestAcceptsPreReleaseAndBuildSemver(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "1.2.3-rc.1+build.5",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0-rc.1+build.2",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("expected semver to be accepted, got error: %v", err)
	}
}

func TestValidateManifestRejectsMissingEnvBooleanFields(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_DATABASE_URL",
				Description: "dsn",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected missing required/secret flags to fail validation")
	}
}

func TestValidateManifestRejectsWhitespacePaddedName(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    " newsletter ",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected whitespace-padded metadata.name to fail validation")
	}
}

func TestValidateManifestRejectsWhitespaceOnlySummary(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "   ",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected whitespace-only summary to fail validation")
	}
}

func TestParseManifestRejectsUnknownFields(t *testing.T) {
	data := []byte(`
apiVersion: htmlctl.dev/extensions/v1
kind: Extension
metadata:
  name: newsletter
  version: 0.1.0
spec:
  summary: test
  compatibility:
    minHTMLCTL: 0.11.0
    minHTMLSERVD: 0.11.0
  runtime:
    requires: [postgresql]
    healthEndpoints: [/healthz]
  integration:
    backendPaths: [/newsletter/*]
  env:
    - name: NEWSLETTER_ENV
      required: true
      secret: false
      description: env
  security:
    listenerPolicy: loopback-only
    requiresRateLimiting: true
    requiresSanitized5xx: true
  typoField: bad
`)

	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected unknown field to fail parsing")
	}
}

func TestParseManifestRejectsTrailingYAMLDocument(t *testing.T) {
	data := []byte(`
apiVersion: htmlctl.dev/extensions/v1
kind: Extension
metadata:
  name: newsletter
  version: 0.1.0
spec:
  summary: test
  compatibility:
    minHTMLCTL: 0.11.0
    minHTMLSERVD: 0.11.0
  runtime:
    requires: [postgresql]
    healthEndpoints: [/healthz]
  integration:
    backendPaths: [/newsletter/*]
  env:
    - name: NEWSLETTER_ENV
      required: true
      secret: false
      description: env
  security:
    listenerPolicy: loopback-only
    requiresRateLimiting: true
    requiresSanitized5xx: true
---
apiVersion: htmlctl.dev/extensions/v1
kind: Extension
`)

	if _, err := ParseManifest(data); err == nil {
		t.Fatalf("expected trailing YAML document to fail parsing")
	}
}

func TestValidateManifestRejectsNonSemverCompatibility(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "01.2.3",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected invalid semver to fail validation")
	}
}

func TestValidateManifestRejectsBlankRuntimeDependency(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql", ""},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected blank dependency to fail validation")
	}
}

func TestValidateManifestRejectsPaddedBackendPath(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{"/healthz"},
			},
			Integration: Integration{
				BackendPaths: []string{" /newsletter/* "},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected padded backend path to fail validation")
	}
}

func TestValidateManifestRejectsPaddedHealthEndpoint(t *testing.T) {
	manifest := Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Metadata: Metadata{
			Name:    "newsletter",
			Version: "0.1.0",
		},
		Spec: Spec{
			Summary: "test",
			Compatibility: Compatibility{
				MinHTMLCTL:   "0.11.0",
				MinHTMLSERVD: "0.11.0",
			},
			Runtime: Runtime{
				Requires:        []string{"postgresql"},
				HealthEndpoints: []string{" /healthz "},
			},
			Integration: Integration{
				BackendPaths: []string{"/newsletter/*"},
			},
			Env: []EnvVar{{
				Name:        "NEWSLETTER_ENV",
				Required:    boolPtr(true),
				Secret:      boolPtr(false),
				Description: "env",
			}},
			Security: Security{
				ListenerPolicy:       "loopback-only",
				RequiresRateLimiting: boolPtr(true),
				RequiresSanitized5xx: boolPtr(true),
			},
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatalf("expected padded health endpoint to fail validation")
	}
}

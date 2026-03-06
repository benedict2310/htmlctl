package extensionspec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/benedict2310/htmlctl/internal/backend"
	"github.com/benedict2310/htmlctl/internal/names"
)

const (
	ManifestAPIVersion = "htmlctl.dev/extensions/v1"
	ManifestKind       = "Extension"
)

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var envNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
var dependencyNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Spec struct {
	Summary       string        `yaml:"summary"`
	Homepage      string        `yaml:"homepage"`
	Compatibility Compatibility `yaml:"compatibility"`
	Runtime       Runtime       `yaml:"runtime"`
	Integration   Integration   `yaml:"integration"`
	Env           []EnvVar      `yaml:"env"`
	Security      Security      `yaml:"security"`
}

type Compatibility struct {
	MinHTMLCTL   string `yaml:"minHTMLCTL"`
	MinHTMLSERVD string `yaml:"minHTMLSERVD"`
}

type Runtime struct {
	Requires        []string `yaml:"requires"`
	HealthEndpoints []string `yaml:"healthEndpoints"`
}

type Integration struct {
	BackendPaths []string `yaml:"backendPaths"`
}

type EnvVar struct {
	Name        string `yaml:"name"`
	Required    *bool  `yaml:"required"`
	Secret      *bool  `yaml:"secret"`
	Description string `yaml:"description"`
}

type Security struct {
	ListenerPolicy       string `yaml:"listenerPolicy"`
	RequiresRateLimiting *bool  `yaml:"requiresRateLimiting"`
	RequiresSanitized5xx *bool  `yaml:"requiresSanitized5xx"`
}

func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse extension manifest: %w", err)
	}
	var trailingDoc any
	if err := decoder.Decode(&trailingDoc); err != io.EOF {
		if err == nil {
			return Manifest{}, fmt.Errorf("parse extension manifest: only one YAML document is allowed")
		}
		return Manifest{}, fmt.Errorf("parse extension manifest trailing document: %w", err)
	}
	return manifest, nil
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read extension manifest %q: %w", path, err)
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, fmt.Errorf("validate extension manifest %q: %w", path, err)
	}
	return manifest, nil
}

func ValidateManifest(manifest Manifest) error {
	if manifest.APIVersion != ManifestAPIVersion {
		return fmt.Errorf("apiVersion must be %q", ManifestAPIVersion)
	}
	if manifest.Kind != ManifestKind {
		return fmt.Errorf("kind must be %q", ManifestKind)
	}
	if err := names.ValidateResourceName(manifest.Metadata.Name); err != nil {
		return fmt.Errorf("metadata.name: %w", err)
	}
	if !validSemver(manifest.Metadata.Version) {
		return fmt.Errorf("metadata.version must be a semver value")
	}
	if strings.TrimSpace(manifest.Spec.Summary) == "" {
		return fmt.Errorf("spec.summary is required")
	}
	if !validSemver(manifest.Spec.Compatibility.MinHTMLCTL) {
		return fmt.Errorf("spec.compatibility.minHTMLCTL must be a semver value")
	}
	if !validSemver(manifest.Spec.Compatibility.MinHTMLSERVD) {
		return fmt.Errorf("spec.compatibility.minHTMLSERVD must be a semver value")
	}
	if len(manifest.Spec.Runtime.Requires) == 0 {
		return fmt.Errorf("spec.runtime.requires must contain at least one dependency")
	}
	for _, dependency := range manifest.Spec.Runtime.Requires {
		trimmed := strings.TrimSpace(dependency)
		if dependency == "" || trimmed != dependency {
			return fmt.Errorf("spec.runtime.requires contains an empty or padded dependency value")
		}
		if !dependencyNamePattern.MatchString(dependency) {
			return fmt.Errorf("spec.runtime.requires value %q must match %q", dependency, dependencyNamePattern.String())
		}
	}
	if len(manifest.Spec.Runtime.HealthEndpoints) == 0 {
		return fmt.Errorf("spec.runtime.healthEndpoints must contain at least one endpoint")
	}
	for _, endpoint := range manifest.Spec.Runtime.HealthEndpoints {
		trimmed := strings.TrimSpace(endpoint)
		if endpoint == "" || trimmed != endpoint {
			return fmt.Errorf("spec.runtime.healthEndpoints must not contain empty or padded values")
		}
		if trimmed == "" {
			return fmt.Errorf("spec.runtime.healthEndpoints must not contain empty values")
		}
		if !strings.HasPrefix(trimmed, "/") {
			return fmt.Errorf("spec.runtime.healthEndpoints value %q must start with /", trimmed)
		}
	}
	if len(manifest.Spec.Integration.BackendPaths) == 0 {
		return fmt.Errorf("spec.integration.backendPaths must contain at least one path")
	}
	for _, pathPrefix := range manifest.Spec.Integration.BackendPaths {
		normalized, err := backend.ValidatePathPrefix(pathPrefix)
		if err != nil {
			return fmt.Errorf("spec.integration.backendPaths: %w", err)
		}
		if normalized != pathPrefix {
			return fmt.Errorf("spec.integration.backendPaths value %q must be canonical and unpadded", pathPrefix)
		}
	}
	if len(manifest.Spec.Env) == 0 {
		return fmt.Errorf("spec.env must contain at least one variable")
	}

	seenEnvNames := make(map[string]struct{}, len(manifest.Spec.Env))
	for _, envVar := range manifest.Spec.Env {
		name := envVar.Name
		if !envNamePattern.MatchString(name) {
			return fmt.Errorf("spec.env name %q must match %q", name, envNamePattern.String())
		}
		if _, exists := seenEnvNames[name]; exists {
			return fmt.Errorf("spec.env name %q is duplicated", name)
		}
		seenEnvNames[name] = struct{}{}
		if envVar.Required == nil {
			return fmt.Errorf("spec.env.required for %q is required", name)
		}
		if envVar.Secret == nil {
			return fmt.Errorf("spec.env.secret for %q is required", name)
		}
		if strings.TrimSpace(envVar.Description) == "" {
			return fmt.Errorf("spec.env description for %q is required", name)
		}
	}
	if strings.TrimSpace(manifest.Spec.Security.ListenerPolicy) == "" {
		return fmt.Errorf("spec.security.listenerPolicy is required")
	}
	if manifest.Spec.Security.RequiresRateLimiting == nil {
		return fmt.Errorf("spec.security.requiresRateLimiting is required")
	}
	if manifest.Spec.Security.RequiresSanitized5xx == nil {
		return fmt.Errorf("spec.security.requiresSanitized5xx is required")
	}

	return nil
}

func validSemver(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "v") {
		value = value[1:]
	}
	return semverPattern.MatchString(value)
}

func ValidateAllManifests(repoRoot string) error {
	paths, err := filepath.Glob(filepath.Join(repoRoot, "extensions", "*", "extension.yaml"))
	if err != nil {
		return fmt.Errorf("glob extension manifests: %w", err)
	}
	if len(paths) == 0 {
		return fmt.Errorf("no extension manifests found")
	}
	sort.Strings(paths)

	for _, path := range paths {
		if _, err := LoadManifest(path); err != nil {
			return err
		}
		extensionDir := filepath.Dir(path)
		for _, filename := range []string{"README.md", "CHANGELOG.md"} {
			requiredPath := filepath.Join(extensionDir, filename)
			if _, err := os.Stat(requiredPath); err != nil {
				return fmt.Errorf("required extension file %q is missing: %w", requiredPath, err)
			}
		}
	}

	return nil
}

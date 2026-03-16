package client

import "github.com/benedict2310/htmlctl/pkg/model"

type Website struct {
	Name               string `json:"name" yaml:"name"`
	DefaultStyleBundle string `json:"defaultStyleBundle" yaml:"defaultStyleBundle"`
	BaseTemplate       string `json:"baseTemplate" yaml:"baseTemplate"`
	CreatedAt          string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt          string `json:"updatedAt" yaml:"updatedAt"`
}

type HealthResponse struct {
	Status string `json:"status" yaml:"status"`
}

type VersionResponse struct {
	Version string `json:"version" yaml:"version"`
}

type WebsitesResponse struct {
	Websites []Website `json:"websites" yaml:"websites"`
}

type Environment struct {
	Name            string  `json:"name" yaml:"name"`
	ActiveReleaseID *string `json:"activeReleaseId,omitempty" yaml:"activeReleaseId,omitempty"`
	CreatedAt       string  `json:"createdAt" yaml:"createdAt"`
	UpdatedAt       string  `json:"updatedAt" yaml:"updatedAt"`
}

type EnvironmentsResponse struct {
	Website      string        `json:"website" yaml:"website"`
	Environments []Environment `json:"environments" yaml:"environments"`
}

type Release struct {
	ReleaseID string `json:"releaseId" yaml:"releaseId"`
	Actor     string `json:"actor,omitempty" yaml:"actor,omitempty"`
	Status    string `json:"status" yaml:"status"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	Active    bool   `json:"active" yaml:"active"`
}

type ReleasesResponse struct {
	Website         string    `json:"website" yaml:"website"`
	Environment     string    `json:"environment" yaml:"environment"`
	ActiveReleaseID *string   `json:"activeReleaseId,omitempty" yaml:"activeReleaseId,omitempty"`
	Limit           int       `json:"limit" yaml:"limit"`
	Offset          int       `json:"offset" yaml:"offset"`
	Releases        []Release `json:"releases" yaml:"releases"`
}

type ResourceCounts struct {
	Pages      int `json:"pages" yaml:"pages"`
	Components int `json:"components" yaml:"components"`
	Styles     int `json:"styles" yaml:"styles"`
	Assets     int `json:"assets" yaml:"assets"`
	Scripts    int `json:"scripts" yaml:"scripts"`
}

type StatusResponse struct {
	Website                string         `json:"website" yaml:"website"`
	Environment            string         `json:"environment" yaml:"environment"`
	ActiveReleaseID        *string        `json:"activeReleaseId,omitempty" yaml:"activeReleaseId,omitempty"`
	ActiveReleaseTimestamp *string        `json:"activeReleaseTimestamp,omitempty" yaml:"activeReleaseTimestamp,omitempty"`
	DefaultStyleBundle     string         `json:"defaultStyleBundle,omitempty" yaml:"defaultStyleBundle,omitempty"`
	BaseTemplate           string         `json:"baseTemplate,omitempty" yaml:"baseTemplate,omitempty"`
	ResourceCounts         ResourceCounts `json:"resourceCounts" yaml:"resourceCounts"`
}

type WebsiteResource struct {
	Name               string             `json:"name" yaml:"name"`
	DefaultStyleBundle string             `json:"defaultStyleBundle" yaml:"defaultStyleBundle"`
	BaseTemplate       string             `json:"baseTemplate" yaml:"baseTemplate"`
	Head               *model.WebsiteHead `json:"head,omitempty" yaml:"head,omitempty"`
	SEO                *model.WebsiteSEO  `json:"seo,omitempty" yaml:"seo,omitempty"`
	ContentHash        string             `json:"contentHash,omitempty" yaml:"contentHash,omitempty"`
	CreatedAt          string             `json:"createdAt" yaml:"createdAt"`
	UpdatedAt          string             `json:"updatedAt" yaml:"updatedAt"`
}

type PageResource struct {
	Name        string                 `json:"name" yaml:"name"`
	Route       string                 `json:"route" yaml:"route"`
	Title       string                 `json:"title" yaml:"title"`
	Description string                 `json:"description" yaml:"description"`
	Layout      []model.PageLayoutItem `json:"layout" yaml:"layout"`
	Head        *model.PageHead        `json:"head,omitempty" yaml:"head,omitempty"`
	ContentHash string                 `json:"contentHash" yaml:"contentHash"`
	CreatedAt   string                 `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt" yaml:"updatedAt"`
}

type ComponentResource struct {
	Name        string `json:"name" yaml:"name"`
	Scope       string `json:"scope" yaml:"scope"`
	HasCSS      bool   `json:"hasCss" yaml:"hasCss"`
	HasJS       bool   `json:"hasJs" yaml:"hasJs"`
	ContentHash string `json:"contentHash" yaml:"contentHash"`
	CSSHash     string `json:"cssHash,omitempty" yaml:"cssHash,omitempty"`
	JSHash      string `json:"jsHash,omitempty" yaml:"jsHash,omitempty"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
}

type StyleFile struct {
	Path string `json:"path" yaml:"path"`
	Hash string `json:"hash" yaml:"hash"`
}

type StyleResource struct {
	Name      string      `json:"name" yaml:"name"`
	Files     []StyleFile `json:"files" yaml:"files"`
	CreatedAt string      `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string      `json:"updatedAt" yaml:"updatedAt"`
}

type AssetResource struct {
	Path        string `json:"path" yaml:"path"`
	ContentType string `json:"contentType" yaml:"contentType"`
	SizeBytes   int64  `json:"sizeBytes" yaml:"sizeBytes"`
	ContentHash string `json:"contentHash" yaml:"contentHash"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
}

type BrandingResource struct {
	Slot        string `json:"slot" yaml:"slot"`
	SourcePath  string `json:"sourcePath" yaml:"sourcePath"`
	ContentType string `json:"contentType" yaml:"contentType"`
	SizeBytes   int64  `json:"sizeBytes" yaml:"sizeBytes"`
	ContentHash string `json:"contentHash" yaml:"contentHash"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
}

type ResourcesResponse struct {
	Website        string              `json:"website" yaml:"website"`
	Environment    string              `json:"environment" yaml:"environment"`
	Site           WebsiteResource     `json:"site" yaml:"site"`
	Pages          []PageResource      `json:"pages" yaml:"pages"`
	Components     []ComponentResource `json:"components" yaml:"components"`
	Styles         []StyleResource     `json:"styles" yaml:"styles"`
	Assets         []AssetResource     `json:"assets" yaml:"assets"`
	Branding       []BrandingResource  `json:"branding" yaml:"branding"`
	ResourceCounts ResourceCounts      `json:"resourceCounts" yaml:"resourceCounts"`
}

type DesiredStateManifestFile struct {
	Path string `json:"path" yaml:"path"`
	Hash string `json:"hash" yaml:"hash"`
}

type DesiredStateManifestResponse struct {
	Website     string                     `json:"website" yaml:"website"`
	Environment string                     `json:"environment" yaml:"environment"`
	Files       []DesiredStateManifestFile `json:"files" yaml:"files"`
}

type ApplyUploadResponse struct {
	Website          string `json:"website" yaml:"website"`
	Environment      string `json:"environment" yaml:"environment"`
	Mode             string `json:"mode" yaml:"mode"`
	DryRun           bool   `json:"dryRun" yaml:"dryRun"`
	AcceptedResource []struct {
		Kind string `json:"kind" yaml:"kind"`
		Name string `json:"name" yaml:"name"`
		Hash string `json:"hash,omitempty" yaml:"hash,omitempty"`
	} `json:"acceptedResources" yaml:"acceptedResources"`
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Changes  struct {
		Created int `json:"created" yaml:"created"`
		Updated int `json:"updated" yaml:"updated"`
		Deleted int `json:"deleted" yaml:"deleted"`
	} `json:"changes" yaml:"changes"`
}

type ReleaseCreateResponse struct {
	Website           string  `json:"website" yaml:"website"`
	Environment       string  `json:"environment" yaml:"environment"`
	ReleaseID         string  `json:"releaseId" yaml:"releaseId"`
	PreviousReleaseID *string `json:"previousReleaseId,omitempty" yaml:"previousReleaseId,omitempty"`
	Status            string  `json:"status" yaml:"status"`
}

type RollbackResponse struct {
	Website       string `json:"website" yaml:"website"`
	Environment   string `json:"environment" yaml:"environment"`
	FromReleaseID string `json:"fromReleaseId" yaml:"fromReleaseId"`
	ToReleaseID   string `json:"toReleaseId" yaml:"toReleaseId"`
}

type PromoteResponse struct {
	Website         string   `json:"website" yaml:"website"`
	FromEnvironment string   `json:"fromEnvironment" yaml:"fromEnvironment"`
	ToEnvironment   string   `json:"toEnvironment" yaml:"toEnvironment"`
	SourceReleaseID string   `json:"sourceReleaseId" yaml:"sourceReleaseId"`
	ReleaseID       string   `json:"releaseId" yaml:"releaseId"`
	FileCount       int      `json:"fileCount" yaml:"fileCount"`
	Hash            string   `json:"hash" yaml:"hash"`
	HashVerified    bool     `json:"hashVerified" yaml:"hashVerified"`
	Strategy        string   `json:"strategy" yaml:"strategy"`
	Warnings        []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type DomainBinding struct {
	ID          int64  `json:"id" yaml:"id"`
	Domain      string `json:"domain" yaml:"domain"`
	Website     string `json:"website" yaml:"website"`
	Environment string `json:"environment" yaml:"environment"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
}

type DomainBindingsResponse struct {
	Domains []DomainBinding `json:"domains" yaml:"domains"`
}

type Backend struct {
	ID          int64  `json:"id,omitempty" yaml:"id,omitempty"`
	PathPrefix  string `json:"pathPrefix" yaml:"pathPrefix"`
	Upstream    string `json:"upstream" yaml:"upstream"`
	CreatedAt   string `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Website     string `json:"website,omitempty" yaml:"website,omitempty"`
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
}

type BackendsResponse struct {
	Website     string    `json:"website,omitempty" yaml:"website,omitempty"`
	Environment string    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Backends    []Backend `json:"backends" yaml:"backends"`
}

type AuthPolicy struct {
	ID          int64  `json:"id,omitempty" yaml:"id,omitempty"`
	PathPrefix  string `json:"pathPrefix" yaml:"pathPrefix"`
	Username    string `json:"username" yaml:"username"`
	CreatedAt   string `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Website     string `json:"website,omitempty" yaml:"website,omitempty"`
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
}

type AuthPoliciesResponse struct {
	Website      string       `json:"website,omitempty" yaml:"website,omitempty"`
	Environment  string       `json:"environment,omitempty" yaml:"environment,omitempty"`
	AuthPolicies []AuthPolicy `json:"authPolicies" yaml:"authPolicies"`
}

type Preview struct {
	ID          int64  `json:"id" yaml:"id"`
	ReleaseID   string `json:"releaseId" yaml:"releaseId"`
	Hostname    string `json:"hostname" yaml:"hostname"`
	Website     string `json:"website,omitempty" yaml:"website,omitempty"`
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	ExpiresAt   string `json:"expiresAt" yaml:"expiresAt"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
}

type PreviewsResponse struct {
	Website     string    `json:"website,omitempty" yaml:"website,omitempty"`
	Environment string    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Previews    []Preview `json:"previews" yaml:"previews"`
}

type RetentionResponse struct {
	Website                 string   `json:"website" yaml:"website"`
	Environment             string   `json:"environment" yaml:"environment"`
	Keep                    int      `json:"keep" yaml:"keep"`
	DryRun                  bool     `json:"dryRun" yaml:"dryRun"`
	BlobGC                  bool     `json:"blobGC" yaml:"blobGC"`
	ActiveReleaseID         *string  `json:"activeReleaseId,omitempty" yaml:"activeReleaseId,omitempty"`
	RollbackReleaseID       *string  `json:"rollbackReleaseId,omitempty" yaml:"rollbackReleaseId,omitempty"`
	PreviewPinnedReleaseIDs []string `json:"previewPinnedReleaseIds" yaml:"previewPinnedReleaseIds"`
	RetainedReleaseIDs      []string `json:"retainedReleaseIds" yaml:"retainedReleaseIds"`
	PrunableReleaseIDs      []string `json:"prunableReleaseIds" yaml:"prunableReleaseIds"`
	PrunedReleaseIDs        []string `json:"prunedReleaseIds" yaml:"prunedReleaseIds"`
	MarkedBlobCount         int      `json:"markedBlobCount" yaml:"markedBlobCount"`
	BlobDeleteCandidates    []string `json:"blobDeleteCandidates" yaml:"blobDeleteCandidates"`
	DeletedBlobHashes       []string `json:"deletedBlobHashes" yaml:"deletedBlobHashes"`
	Warnings                []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type LogsResponse struct {
	Entries []AuditLogEntry `json:"entries" yaml:"entries"`
	Total   int             `json:"total" yaml:"total"`
	Limit   int             `json:"limit" yaml:"limit"`
	Offset  int             `json:"offset" yaml:"offset"`
}

type AuditLogEntry struct {
	ID              int64          `json:"id" yaml:"id"`
	Actor           string         `json:"actor" yaml:"actor"`
	Timestamp       string         `json:"timestamp" yaml:"timestamp"`
	EnvironmentID   *int64         `json:"environmentId,omitempty" yaml:"environmentId,omitempty"`
	Operation       string         `json:"operation" yaml:"operation"`
	ResourceSummary string         `json:"resourceSummary" yaml:"resourceSummary"`
	ReleaseID       *string        `json:"releaseId,omitempty" yaml:"releaseId,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ApplyCommandResponse struct {
	Website     string                 `json:"website" yaml:"website"`
	Environment string                 `json:"environment" yaml:"environment"`
	DryRun      bool                   `json:"dryRun" yaml:"dryRun"`
	Upload      ApplyUploadResponse    `json:"upload" yaml:"upload"`
	Release     *ReleaseCreateResponse `json:"release,omitempty" yaml:"release,omitempty"`
}

package client

type Website struct {
	Name               string `json:"name" yaml:"name"`
	DefaultStyleBundle string `json:"defaultStyleBundle" yaml:"defaultStyleBundle"`
	BaseTemplate       string `json:"baseTemplate" yaml:"baseTemplate"`
	CreatedAt          string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt          string `json:"updatedAt" yaml:"updatedAt"`
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
	ResourceCounts         ResourceCounts `json:"resourceCounts" yaml:"resourceCounts"`
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

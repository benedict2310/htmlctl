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
	Status    string `json:"status" yaml:"status"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	Active    bool   `json:"active" yaml:"active"`
}

type ReleasesResponse struct {
	Website         string    `json:"website" yaml:"website"`
	Environment     string    `json:"environment" yaml:"environment"`
	ActiveReleaseID *string   `json:"activeReleaseId,omitempty" yaml:"activeReleaseId,omitempty"`
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

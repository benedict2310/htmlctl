package db

import "strings"

type WebsiteRow struct {
	ID                 int64
	Name               string
	DefaultStyleBundle string
	BaseTemplate       string
	HeadJSON           string
	ContentHash        string
	CreatedAt          string
	UpdatedAt          string
}

func (r WebsiteRow) HeadJSONOrDefault() string {
	if strings.TrimSpace(r.HeadJSON) == "" {
		return "{}"
	}
	return r.HeadJSON
}

type EnvironmentRow struct {
	ID              int64
	WebsiteID       int64
	Name            string
	ActiveReleaseID *string
	CreatedAt       string
	UpdatedAt       string
}

type PageRow struct {
	ID          int64
	WebsiteID   int64
	Name        string
	Route       string
	Title       string
	Description string
	LayoutJSON  string
	HeadJSON    string
	ContentHash string
	CreatedAt   string
	UpdatedAt   string
}

func (r PageRow) HeadJSONOrDefault() string {
	if strings.TrimSpace(r.HeadJSON) == "" {
		return "{}"
	}
	return r.HeadJSON
}

type ComponentRow struct {
	ID          int64
	WebsiteID   int64
	Name        string
	Scope       string
	ContentHash string
	CreatedAt   string
	UpdatedAt   string
}

type StyleBundleRow struct {
	ID        int64
	WebsiteID int64
	Name      string
	FilesJSON string
	CreatedAt string
	UpdatedAt string
}

type AssetRow struct {
	ID          int64
	WebsiteID   int64
	Filename    string
	ContentType string
	SizeBytes   int64
	ContentHash string
	CreatedAt   string
}

type WebsiteIconRow struct {
	ID          int64
	WebsiteID   int64
	Slot        string
	SourcePath  string
	ContentType string
	SizeBytes   int64
	ContentHash string
	CreatedAt   string
	UpdatedAt   string
}

type ReleaseRow struct {
	ID            string
	EnvironmentID int64
	ManifestJSON  string
	OutputHashes  string
	BuildLog      string
	Status        string
	CreatedAt     string
}

type AuditLogRow struct {
	ID              int64
	Actor           string
	Timestamp       string
	EnvironmentID   *int64
	Operation       string
	ResourceSummary string
	ReleaseID       *string
	MetadataJSON    string
}

type DomainBindingRow struct {
	ID            int64
	Domain        string
	EnvironmentID int64
	CreatedAt     string
	UpdatedAt     string
}

type DomainBindingResolvedRow struct {
	ID              int64
	Domain          string
	EnvironmentID   int64
	WebsiteName     string
	EnvironmentName string
	CreatedAt       string
	UpdatedAt       string
}

type TelemetryEventRow struct {
	ID            int64
	EnvironmentID int64
	EventName     string
	Path          string
	OccurredAt    *string
	ReceivedAt    string
	SessionID     *string
	AttrsJSON     string
}

type ListTelemetryEventsParams struct {
	EnvironmentID int64
	EventName     string
	Since         *string
	Until         *string
	Limit         int
	Offset        int
}

package db

type WebsiteRow struct {
	ID                 int64
	Name               string
	DefaultStyleBundle string
	BaseTemplate       string
	CreatedAt          string
	UpdatedAt          string
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
	ContentHash string
	CreatedAt   string
	UpdatedAt   string
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

package audit

import (
	"context"
	"time"
)

const (
	OperationApply           = "apply"
	OperationReleaseBuild    = "release.build"
	OperationReleaseActivate = "release.activate"
	OperationRollback        = "rollback"
	OperationPromote         = "promote"
	OperationDomainAdd       = "domain.add"
	OperationDomainRemove    = "domain.remove"
)

type Entry struct {
	ID              int64          `json:"id"`
	Actor           string         `json:"actor"`
	Timestamp       time.Time      `json:"timestamp"`
	EnvironmentID   *int64         `json:"environmentId,omitempty"`
	Operation       string         `json:"operation"`
	ResourceSummary string         `json:"resourceSummary"`
	ReleaseID       *string        `json:"releaseId,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type Filter struct {
	WebsiteID     int64
	EnvironmentID *int64
	Operation     string
	Since         *time.Time
	Until         *time.Time
	Limit         int
	Offset        int
}

type QueryResult struct {
	Entries []Entry
	Total   int
	Limit   int
	Offset  int
}

type Logger interface {
	Log(ctx context.Context, entry Entry) error
	Query(ctx context.Context, filter Filter) (QueryResult, error)
}

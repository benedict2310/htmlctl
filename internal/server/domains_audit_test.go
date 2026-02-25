package server

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/benedict2310/htmlctl/internal/audit"
)

type stubAuditLogger struct {
	logErr  error
	waitErr error
}

func (s *stubAuditLogger) Log(ctx context.Context, entry audit.Entry) error {
	return s.logErr
}

func (s *stubAuditLogger) Query(ctx context.Context, filter audit.Filter) (audit.QueryResult, error) {
	return audit.QueryResult{}, nil
}

func (s *stubAuditLogger) WaitIdle(ctx context.Context) error {
	return s.waitErr
}

func TestLogDomainAuditBranches(t *testing.T) {
	s := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Nil audit logger should no-op.
	s.logDomainAudit(context.Background(), "tester", audit.OperationDomainAdd, 1, "example.com", "sample", "staging")

	// Log error branch.
	s.auditLogger = &stubAuditLogger{logErr: context.DeadlineExceeded}
	s.logDomainAudit(context.Background(), "tester", audit.OperationDomainAdd, 1, "example.com", "sample", "staging")

	// WaitIdle warning branch.
	s.auditLogger = &stubAuditLogger{waitErr: context.DeadlineExceeded}
	s.logDomainAudit(context.Background(), "tester", audit.OperationDomainRemove, 1, "example.com", "sample", "staging")

	// Success branch.
	s.auditLogger = &stubAuditLogger{}
	s.logDomainAudit(context.Background(), "tester", audit.OperationDomainAdd, 1, "example.com", "sample", "staging")
}

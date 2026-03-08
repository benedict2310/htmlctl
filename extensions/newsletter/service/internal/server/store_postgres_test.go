package server

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClaimCampaignSend(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	staleBefore := now.Add(-10 * time.Minute)

	t.Run("claims fresh recipient", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO campaign_sends (campaign_id, subscriber_id, status, error_text, created_at, updated_at, attempt_count)
VALUES ($1, $2, 'sending', NULL, $3, $3, 1)
ON CONFLICT (campaign_id, subscriber_id) DO UPDATE
SET status = 'sending',
    provider_message_id = NULL,
    error_text = NULL,
    updated_at = EXCLUDED.updated_at,
    attempt_count = campaign_sends.attempt_count + 1
WHERE campaign_sends.status <> 'sent'
  AND (campaign_sends.status <> 'sending' OR campaign_sends.updated_at < $4)
RETURNING id
`)).
			WithArgs(int64(9), int64(42), now, staleBefore).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(77)))

		claimed, err := NewPostgresStore(db).ClaimCampaignSend(context.Background(), 9, 42, now, staleBefore)
		if err != nil {
			t.Fatalf("ClaimCampaignSend() error = %v", err)
		}
		if !claimed {
			t.Fatal("expected claim to succeed")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("returns false when row is already actively claimed", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO campaign_sends (campaign_id, subscriber_id, status, error_text, created_at, updated_at, attempt_count)
VALUES ($1, $2, 'sending', NULL, $3, $3, 1)
ON CONFLICT (campaign_id, subscriber_id) DO UPDATE
SET status = 'sending',
    provider_message_id = NULL,
    error_text = NULL,
    updated_at = EXCLUDED.updated_at,
    attempt_count = campaign_sends.attempt_count + 1
WHERE campaign_sends.status <> 'sent'
  AND (campaign_sends.status <> 'sending' OR campaign_sends.updated_at < $4)
RETURNING id
`)).
			WithArgs(int64(9), int64(42), now, staleBefore).
			WillReturnError(sql.ErrNoRows)

		claimed, err := NewPostgresStore(db).ClaimCampaignSend(context.Background(), 9, 42, now, staleBefore)
		if err != nil {
			t.Fatalf("ClaimCampaignSend() error = %v", err)
		}
		if claimed {
			t.Fatal("expected claim to be skipped")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestImportLegacySubscribers_DryRunSummarizesInsertUpdateAndSkip(t *testing.T) {
	sourceDB, sourceMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(source) error = %v", err)
	}
	t.Cleanup(func() { _ = sourceDB.Close() })

	targetDB, targetMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(target) error = %v", err)
	}
	t.Cleanup(func() { _ = targetDB.Close() })

	subscribedAt := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	confirmedAt := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 3, 11, 0, 0, 0, time.UTC)
	olderUpdatedAt := time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)
	unsubscribedAt := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	sourceRows := sqlmock.NewRows([]string{"email", "status", "subscribed_at", "confirmed_at", "unsubscribed_at", "updated_at"}).
		AddRow("insert@example.com", "confirmed", subscribedAt, confirmedAt, nil, updatedAt).
		AddRow("skip@example.com", "pending", subscribedAt, nil, nil, updatedAt).
		AddRow("update@example.com", "unsubscribed", subscribedAt, confirmedAt, unsubscribedAt, updatedAt)
	sourceMock.ExpectQuery(regexp.QuoteMeta(`
SELECT email, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at
FROM subscribers
ORDER BY id
`)).WillReturnRows(sourceRows)

	targetMock.ExpectBegin()

	targetMock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at
FROM subscribers
WHERE LOWER(email) = LOWER($1)
FOR UPDATE
`)).
		WithArgs("insert@example.com").
		WillReturnError(sql.ErrNoRows)
	targetMock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO subscribers (email, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
`)).
		WithArgs("insert@example.com", "confirmed", subscribedAt, confirmedAt, nil, updatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	targetMock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at
FROM subscribers
WHERE LOWER(email) = LOWER($1)
FOR UPDATE
`)).
		WithArgs("skip@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "subscribed_at", "confirmed_at", "unsubscribed_at", "updated_at"}).
			AddRow(int64(2), "pending", subscribedAt, nil, nil, updatedAt))

	targetMock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at
FROM subscribers
WHERE LOWER(email) = LOWER($1)
FOR UPDATE
`)).
		WithArgs("update@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "subscribed_at", "confirmed_at", "unsubscribed_at", "updated_at"}).
			AddRow(int64(3), "pending", subscribedAt, nil, nil, olderUpdatedAt))
	targetMock.ExpectExec(regexp.QuoteMeta(`
UPDATE subscribers
SET status = $2,
    subscribed_at = $3,
    confirmed_at = $4,
    unsubscribed_at = $5,
    updated_at = $6
WHERE id = $1
`)).
		WithArgs(int64(3), "unsubscribed", subscribedAt, confirmedAt, unsubscribedAt, updatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	targetMock.ExpectExec(regexp.QuoteMeta(`
UPDATE verification_tokens
SET used_at = COALESCE(used_at, $2)
WHERE subscriber_id = $1
`)).
		WithArgs(int64(3), updatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	targetMock.ExpectRollback()

	summary, err := NewPostgresStore(targetDB).ImportLegacySubscribers(context.Background(), sourceDB, true)
	if err != nil {
		t.Fatalf("ImportLegacySubscribers() error = %v", err)
	}
	if summary.SourceTotal != 3 {
		t.Fatalf("expected SourceTotal 3, got %d", summary.SourceTotal)
	}
	if summary.Inserted != 1 || summary.Updated != 1 || summary.Skipped != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if summary.StatusCounts["confirmed"] != 1 || summary.StatusCounts["pending"] != 1 || summary.StatusCounts["unsubscribed"] != 1 {
		t.Fatalf("unexpected status counts: %+v", summary.StatusCounts)
	}

	if err := sourceMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet source sqlmock expectations: %v", err)
	}
	if err := targetMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet target sqlmock expectations: %v", err)
	}
}

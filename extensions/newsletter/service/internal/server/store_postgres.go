package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/campaign"
)

type PostgresStore struct {
	DB *sql.DB
}

type ImportSummary struct {
	SourceTotal  int
	Inserted     int
	Updated      int
	Skipped      int
	StatusCounts map[string]int
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{DB: db}
}

func (s PostgresStore) Signup(ctx context.Context, email string, tokenHash []byte, expiresAt, now time.Time) (bool, error) {
	if s.DB == nil {
		return false, errors.New("database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		subscriberID int64
		status       string
	)
	err = tx.QueryRowContext(ctx, `
SELECT id, status
FROM subscribers
WHERE LOWER(email) = LOWER($1)
FOR UPDATE
`, email).Scan(&subscriberID, &status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("lookup subscriber: %w", err)
	}

	needsVerification := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.QueryRowContext(ctx, `
INSERT INTO subscribers (email, status, subscribed_at, updated_at)
VALUES ($1, 'pending', $2, $2)
RETURNING id
`, email, now).Scan(&subscriberID); err != nil {
			return false, fmt.Errorf("insert subscriber: %w", err)
		}
		needsVerification = true
	case status == "pending":
		if _, err := tx.ExecContext(ctx, `
UPDATE subscribers
SET updated_at = $2
WHERE id = $1
`, subscriberID, now); err != nil {
			return false, fmt.Errorf("touch pending subscriber: %w", err)
		}
		needsVerification = true
	case status == "unsubscribed":
		if _, err := tx.ExecContext(ctx, `
UPDATE subscribers
SET status = 'pending', unsubscribed_at = NULL, confirmed_at = NULL, updated_at = $2
WHERE id = $1
`, subscriberID, now); err != nil {
			return false, fmt.Errorf("reactivate subscriber: %w", err)
		}
		needsVerification = true
	case status == "confirmed":
		if _, err := tx.ExecContext(ctx, `
UPDATE subscribers
SET updated_at = $2
WHERE id = $1
`, subscriberID, now); err != nil {
			return false, fmt.Errorf("touch confirmed subscriber: %w", err)
		}
	default:
		return false, fmt.Errorf("unsupported subscriber status %q", status)
	}

	if needsVerification {
		if _, err := tx.ExecContext(ctx, `
UPDATE verification_tokens
SET used_at = $2
WHERE subscriber_id = $1 AND used_at IS NULL
`, subscriberID, now); err != nil {
			return false, fmt.Errorf("expire prior tokens: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO verification_tokens (subscriber_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4)
`, subscriberID, tokenHash, expiresAt, now); err != nil {
			return false, fmt.Errorf("insert verification token: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit signup tx: %w", err)
	}
	return needsVerification, nil
}

func (s PostgresStore) Verify(ctx context.Context, tokenHash []byte, now time.Time) (VerifyResult, error) {
	if s.DB == nil {
		return VerifyInvalid, errors.New("database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return VerifyInvalid, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		tokenID      int64
		subscriberID int64
		expiresAt    time.Time
		usedAt       sql.NullTime
	)

	err = tx.QueryRowContext(ctx, `
SELECT id, subscriber_id, expires_at, used_at
FROM verification_tokens
WHERE token_hash = $1
ORDER BY id DESC
LIMIT 1
FOR UPDATE
`, tokenHash).Scan(&tokenID, &subscriberID, &expiresAt, &usedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VerifyInvalid, errInvalidToken
		}
		return VerifyInvalid, fmt.Errorf("lookup token: %w", err)
	}

	if usedAt.Valid {
		if err := tx.Commit(); err != nil {
			return VerifyInvalid, fmt.Errorf("commit already-used token tx: %w", err)
		}
		return VerifyAlreadyUsed, nil
	}
	if now.After(expiresAt) {
		if err := tx.Commit(); err != nil {
			return VerifyInvalid, fmt.Errorf("commit expired token tx: %w", err)
		}
		return VerifyExpired, errExpiredToken
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE subscribers
SET status = 'confirmed', confirmed_at = COALESCE(confirmed_at, $2), unsubscribed_at = NULL, updated_at = $2
WHERE id = $1
`, subscriberID, now); err != nil {
		return VerifyInvalid, fmt.Errorf("confirm subscriber: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE verification_tokens
SET used_at = $2
WHERE id = $1
`, tokenID, now); err != nil {
		return VerifyInvalid, fmt.Errorf("mark token used: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return VerifyInvalid, fmt.Errorf("commit verify tx: %w", err)
	}
	return VerifyConfirmed, nil
}

func (s PostgresStore) Unsubscribe(ctx context.Context, subscriberID int64, now time.Time) (UnsubscribeResult, error) {
	if s.DB == nil {
		return UnsubscribeInvalid, errors.New("database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return UnsubscribeInvalid, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRowContext(ctx, `
SELECT status
FROM subscribers
WHERE id = $1
FOR UPDATE
`, subscriberID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UnsubscribeInvalid, errInvalidUnsubscribeToken
		}
		return UnsubscribeInvalid, fmt.Errorf("lookup subscriber: %w", err)
	}

	switch status {
	case "unsubscribed":
		if err := tx.Commit(); err != nil {
			return UnsubscribeInvalid, fmt.Errorf("commit already-unsubscribed tx: %w", err)
		}
		return UnsubscribeAlreadyUnsubscribed, nil
	case "pending", "confirmed":
		if _, err := tx.ExecContext(ctx, `
UPDATE subscribers
SET status = 'unsubscribed', unsubscribed_at = $2, updated_at = $2
WHERE id = $1
`, subscriberID, now); err != nil {
			return UnsubscribeInvalid, fmt.Errorf("mark subscriber unsubscribed: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return UnsubscribeInvalid, fmt.Errorf("commit unsubscribe tx: %w", err)
		}
		return UnsubscribeConfirmed, nil
	default:
		return UnsubscribeInvalid, fmt.Errorf("unsupported subscriber status %q", status)
	}
}

func (s PostgresStore) UpsertCampaign(ctx context.Context, slug, subject, htmlBody, textBody string, now time.Time) (campaign.Campaign, error) {
	if s.DB == nil {
		return campaign.Campaign{}, errors.New("database is required")
	}
	var c campaign.Campaign
	err := s.DB.QueryRowContext(ctx, `
INSERT INTO campaigns (slug, subject, html_body, text_body, status, created_at, sent_at)
VALUES ($1, $2, $3, $4, 'draft', $5, NULL)
ON CONFLICT (slug) DO UPDATE
SET subject = EXCLUDED.subject,
    html_body = EXCLUDED.html_body,
    text_body = EXCLUDED.text_body,
    status = 'draft',
    sent_at = NULL
RETURNING id, slug, subject, html_body, text_body, status
`, slug, subject, htmlBody, textBody, now).Scan(&c.ID, &c.Slug, &c.Subject, &c.HTMLBody, &c.TextBody, &c.Status)
	if err != nil {
		return campaign.Campaign{}, fmt.Errorf("upsert campaign: %w", err)
	}
	return c, nil
}

func (s PostgresStore) GetCampaignBySlug(ctx context.Context, slug string) (campaign.Campaign, error) {
	if s.DB == nil {
		return campaign.Campaign{}, errors.New("database is required")
	}
	var c campaign.Campaign
	err := s.DB.QueryRowContext(ctx, `
SELECT id, slug, subject, html_body, text_body, status
FROM campaigns
WHERE slug = $1
`, slug).Scan(&c.ID, &c.Slug, &c.Subject, &c.HTMLBody, &c.TextBody, &c.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return campaign.Campaign{}, fmt.Errorf("campaign %q not found", slug)
		}
		return campaign.Campaign{}, fmt.Errorf("get campaign: %w", err)
	}
	return c, nil
}

func (s PostgresStore) ListEligibleRecipients(ctx context.Context, campaignID int64, seedEmails []string) ([]campaign.Recipient, error) {
	if s.DB == nil {
		return nil, errors.New("database is required")
	}
	query := `
SELECT s.id, s.email
FROM subscribers s
LEFT JOIN campaign_sends cs
  ON cs.campaign_id = $1 AND cs.subscriber_id = s.id
WHERE s.status = 'confirmed'
  AND COALESCE(cs.status, '') <> 'sent'`
	args := []any{campaignID}
	if len(seedEmails) > 0 {
		query += ` AND LOWER(s.email) = ANY($2)`
		args = append(args, seedEmails)
	}
	query += ` ORDER BY s.id`

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list eligible recipients: %w", err)
	}
	defer rows.Close()

	var recipients []campaign.Recipient
	for rows.Next() {
		var r campaign.Recipient
		if err := rows.Scan(&r.SubscriberID, &r.Email); err != nil {
			return nil, fmt.Errorf("scan eligible recipient: %w", err)
		}
		recipients = append(recipients, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eligible recipients: %w", err)
	}
	return recipients, nil
}

func (s PostgresStore) ClaimCampaignSend(ctx context.Context, campaignID, subscriberID int64, now, staleBefore time.Time) (bool, error) {
	if s.DB == nil {
		return false, errors.New("database is required")
	}
	var claimedID int64
	err := s.DB.QueryRowContext(ctx, `
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
`, campaignID, subscriberID, now, staleBefore).Scan(&claimedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("claim campaign send: %w", err)
	}
	return claimedID > 0, nil
}

func (s PostgresStore) RecordCampaignSendSuccess(ctx context.Context, campaignID, subscriberID int64, providerMessageID string, now time.Time) error {
	if s.DB == nil {
		return errors.New("database is required")
	}
	_, err := s.DB.ExecContext(ctx, `
UPDATE campaign_sends
SET status = 'sent',
    provider_message_id = $3,
    error_text = NULL,
    updated_at = $4
WHERE campaign_id = $1 AND subscriber_id = $2
`, campaignID, subscriberID, strings.TrimSpace(providerMessageID), now)
	if err != nil {
		return fmt.Errorf("record campaign send success: %w", err)
	}
	return nil
}

func (s PostgresStore) RecordCampaignSendFailure(ctx context.Context, campaignID, subscriberID int64, errorText string, now time.Time) error {
	if s.DB == nil {
		return errors.New("database is required")
	}
	trimmed := strings.TrimSpace(errorText)
	if len(trimmed) > 500 {
		trimmed = trimmed[:500]
	}
	_, err := s.DB.ExecContext(ctx, `
UPDATE campaign_sends
SET status = 'failed',
    provider_message_id = NULL,
    error_text = $3,
    updated_at = $4
WHERE campaign_id = $1 AND subscriber_id = $2
`, campaignID, subscriberID, trimmed, now)
	if err != nil {
		return fmt.Errorf("record campaign send failure: %w", err)
	}
	return nil
}

func (s PostgresStore) MarkCampaignSent(ctx context.Context, campaignID int64, now time.Time) error {
	if s.DB == nil {
		return errors.New("database is required")
	}
	_, err := s.DB.ExecContext(ctx, `
UPDATE campaigns c
SET status = 'sent', sent_at = COALESCE(sent_at, $2)
WHERE c.id = $1
  AND NOT EXISTS (
    SELECT 1
    FROM subscribers s
    LEFT JOIN campaign_sends cs
      ON cs.campaign_id = c.id AND cs.subscriber_id = s.id
    WHERE s.status = 'confirmed'
      AND COALESCE(cs.status, '') <> 'sent'
  )
`, campaignID, now)
	if err != nil {
		return fmt.Errorf("mark campaign sent: %w", err)
	}
	return nil
}

func (s PostgresStore) ImportLegacySubscribers(ctx context.Context, sourceDB *sql.DB, dryRun bool) (ImportSummary, error) {
	if s.DB == nil {
		return ImportSummary{}, errors.New("target database is required")
	}
	if sourceDB == nil {
		return ImportSummary{}, errors.New("source database is required")
	}
	rows, err := sourceDB.QueryContext(ctx, `
SELECT email, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at
FROM subscribers
ORDER BY id
`)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("query source subscribers: %w", err)
	}
	defer rows.Close()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("begin import tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	summary := ImportSummary{StatusCounts: map[string]int{}}
	for rows.Next() {
		var (
			email          string
			status         string
			subscribedAt   time.Time
			confirmedAt    sql.NullTime
			unsubscribedAt sql.NullTime
			updatedAt      time.Time
		)
		if err := rows.Scan(&email, &status, &subscribedAt, &confirmedAt, &unsubscribedAt, &updatedAt); err != nil {
			return ImportSummary{}, fmt.Errorf("scan source subscriber: %w", err)
		}
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		if normalizedStatus != "pending" && normalizedStatus != "confirmed" && normalizedStatus != "unsubscribed" {
			return ImportSummary{}, fmt.Errorf("unsupported legacy subscriber status %q", status)
		}
		summary.SourceTotal++
		summary.StatusCounts[normalizedStatus]++

		var (
			targetID           int64
			targetStatus       string
			targetSubscribed   time.Time
			targetConfirmed    sql.NullTime
			targetUnsubscribed sql.NullTime
			targetUpdated      time.Time
		)
		err := tx.QueryRowContext(ctx, `
SELECT id, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at
FROM subscribers
WHERE LOWER(email) = LOWER($1)
FOR UPDATE
`, email).Scan(&targetID, &targetStatus, &targetSubscribed, &targetConfirmed, &targetUnsubscribed, &targetUpdated)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ImportSummary{}, fmt.Errorf("lookup target subscriber: %w", err)
		}

		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO subscribers (email, status, subscribed_at, confirmed_at, unsubscribed_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
`, email, normalizedStatus, subscribedAt, nullTimeValue(confirmedAt), nullTimeValue(unsubscribedAt), updatedAt); err != nil {
				return ImportSummary{}, fmt.Errorf("insert imported subscriber: %w", err)
			}
			summary.Inserted++
			continue
		}

		if sameSubscriberRow(normalizedStatus, subscribedAt, confirmedAt, unsubscribedAt, updatedAt, targetStatus, targetSubscribed, targetConfirmed, targetUnsubscribed, targetUpdated) {
			summary.Skipped++
			continue
		}

		if _, err := tx.ExecContext(ctx, `
UPDATE subscribers
SET status = $2,
    subscribed_at = $3,
    confirmed_at = $4,
    unsubscribed_at = $5,
    updated_at = $6
WHERE id = $1
`, targetID, normalizedStatus, subscribedAt, nullTimeValue(confirmedAt), nullTimeValue(unsubscribedAt), updatedAt); err != nil {
			return ImportSummary{}, fmt.Errorf("update imported subscriber: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE verification_tokens
SET used_at = COALESCE(used_at, $2)
WHERE subscriber_id = $1
`, targetID, updatedAt); err != nil {
			return ImportSummary{}, fmt.Errorf("expire imported verification tokens: %w", err)
		}
		summary.Updated++
	}
	if err := rows.Err(); err != nil {
		return ImportSummary{}, fmt.Errorf("iterate source subscribers: %w", err)
	}

	if dryRun {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			return ImportSummary{}, fmt.Errorf("rollback import dry-run: %w", err)
		}
		return summary, nil
	}
	if err := tx.Commit(); err != nil {
		return ImportSummary{}, fmt.Errorf("commit import tx: %w", err)
	}
	return summary, nil
}

func sameSubscriberRow(sourceStatus string, sourceSubscribed time.Time, sourceConfirmed, sourceUnsubscribed sql.NullTime, sourceUpdated time.Time, targetStatus string, targetSubscribed time.Time, targetConfirmed, targetUnsubscribed sql.NullTime, targetUpdated time.Time) bool {
	return sourceStatus == targetStatus &&
		sourceSubscribed.Equal(targetSubscribed) &&
		nullTimesEqual(sourceConfirmed, targetConfirmed) &&
		nullTimesEqual(sourceUnsubscribed, targetUnsubscribed) &&
		sourceUpdated.Equal(targetUpdated)
}

func nullTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func nullTimesEqual(a, b sql.NullTime) bool {
	if a.Valid != b.Valid {
		return false
	}
	if !a.Valid {
		return true
	}
	return a.Time.Equal(b.Time)
}

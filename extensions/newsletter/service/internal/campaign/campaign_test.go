package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubStore struct {
	campaign   Campaign
	recipients []Recipient
	claimed    map[int64]bool
	failed     []int64
	sent       []int64
	upserted   bool
	sentMarked bool
}

func (s *stubStore) UpsertCampaign(context.Context, string, string, string, string, time.Time) (Campaign, error) {
	s.upserted = true
	return s.campaign, nil
}

func (s *stubStore) GetCampaignBySlug(context.Context, string) (Campaign, error) {
	return s.campaign, nil
}
func (s *stubStore) ListEligibleRecipients(context.Context, int64, []string) ([]Recipient, error) {
	return s.recipients, nil
}
func (s *stubStore) ClaimCampaignSend(_ context.Context, _ int64, subscriberID int64, _, _ time.Time) (bool, error) {
	if s.claimed == nil {
		s.claimed = map[int64]bool{}
	}
	if s.claimed[subscriberID] {
		return false, nil
	}
	s.claimed[subscriberID] = true
	return true, nil
}
func (s *stubStore) RecordCampaignSendSuccess(_ context.Context, _ int64, subscriberID int64, _ string, _ time.Time) error {
	s.sent = append(s.sent, subscriberID)
	return nil
}
func (s *stubStore) RecordCampaignSendFailure(_ context.Context, _ int64, subscriberID int64, _ string, _ time.Time) error {
	s.failed = append(s.failed, subscriberID)
	return nil
}
func (s *stubStore) MarkCampaignSent(context.Context, int64, time.Time) error {
	s.sentMarked = true
	return nil
}

type stubMailer struct {
	errForEmail string
	htmlBodies  []string
	textBodies  []string
}

func (m *stubMailer) SendCampaign(_ context.Context, email, _ string, htmlBody, textBody string) (string, error) {
	m.htmlBodies = append(m.htmlBodies, htmlBody)
	m.textBodies = append(m.textBodies, textBody)
	if email == m.errForEmail {
		return "", errors.New("provider failed")
	}
	return "msg-123", nil
}

func TestSendAddsUnsubscribeFooterAndPacing(t *testing.T) {
	store := &stubStore{
		campaign:   Campaign{ID: 10, Slug: "launch", Subject: "Launch", HTMLBody: "<p>Hello</p>", TextBody: "Hello"},
		recipients: []Recipient{{SubscriberID: 1, Email: "a@example.com"}, {SubscriberID: 2, Email: "b@example.com"}},
	}
	mailer := &stubMailer{}
	sleeps := 0
	_, err := Send(context.Background(), store, mailer, SendOptions{
		Slug:          "launch",
		Mode:          "all",
		Confirm:       true,
		Interval:      time.Second,
		PublicBaseURL: "https://example.com",
		LinkSecret:    "secret",
		Now:           func() time.Time { return time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC) },
		Sleep:         func(time.Duration) { sleeps++ },
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if sleeps != 1 {
		t.Fatalf("expected one pacing sleep, got %d", sleeps)
	}
	if len(store.sent) != 2 || !store.sentMarked {
		t.Fatalf("expected both recipients sent and campaign marked sent: %+v", store)
	}
	if !strings.Contains(mailer.htmlBodies[0], "/newsletter/unsubscribe?token=") {
		t.Fatalf("expected unsubscribe link in html body: %s", mailer.htmlBodies[0])
	}
	if !strings.Contains(mailer.textBodies[0], "Unsubscribe instantly") {
		t.Fatalf("expected unsubscribe link in text body: %s", mailer.textBodies[0])
	}
}

func TestSendPreviewFooterOmitsUnsubscribe(t *testing.T) {
	store := &stubStore{campaign: Campaign{ID: 1, Slug: "launch", Subject: "Launch", HTMLBody: "<p>Hello</p>", TextBody: "Hello"}}
	mailer := &stubMailer{}
	if err := Preview(context.Background(), store, mailer, "launch", "preview@example.com"); err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if !strings.Contains(mailer.htmlBodies[0], "recipient-specific unsubscribe link") {
		t.Fatalf("expected preview note in html body: %s", mailer.htmlBodies[0])
	}
	if strings.Contains(mailer.htmlBodies[0], "/newsletter/unsubscribe?token=") {
		t.Fatalf("did not expect unsubscribe link in preview body: %s", mailer.htmlBodies[0])
	}
}

func TestSendRequiresConfirm(t *testing.T) {
	_, err := Send(context.Background(), &stubStore{}, &stubMailer{}, SendOptions{Slug: "launch"})
	if err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("expected confirm error, got %v", err)
	}
}

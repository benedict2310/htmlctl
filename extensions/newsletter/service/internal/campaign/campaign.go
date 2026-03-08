package campaign

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/links"
)

type Campaign struct {
	ID       int64
	Slug     string
	Subject  string
	HTMLBody string
	TextBody string
	Status   string
}

type Recipient struct {
	SubscriberID int64
	Email        string
}

type Summary struct {
	Eligible  int
	Attempted int
	Succeeded int
	Failed    int
	Skipped   int
}

type Store interface {
	UpsertCampaign(ctx context.Context, slug, subject, htmlBody, textBody string, now time.Time) (Campaign, error)
	GetCampaignBySlug(ctx context.Context, slug string) (Campaign, error)
	ListEligibleRecipients(ctx context.Context, campaignID int64, seedEmails []string) ([]Recipient, error)
	ClaimCampaignSend(ctx context.Context, campaignID, subscriberID int64, now, staleBefore time.Time) (bool, error)
	RecordCampaignSendSuccess(ctx context.Context, campaignID, subscriberID int64, providerMessageID string, now time.Time) error
	RecordCampaignSendFailure(ctx context.Context, campaignID, subscriberID int64, errorText string, now time.Time) error
	MarkCampaignSent(ctx context.Context, campaignID int64, now time.Time) error
}

type Mailer interface {
	SendCampaign(ctx context.Context, email, subject, htmlBody, textBody string) (string, error)
}

type SendOptions struct {
	Slug          string
	Mode          string
	SeedEmails    []string
	Interval      time.Duration
	StaleAfter    time.Duration
	Confirm       bool
	PublicBaseURL string
	LinkSecret    string
	Now           func() time.Time
	Sleep         func(time.Duration)
}

func UpsertFromFiles(ctx context.Context, store Store, slug, subject, htmlPath, textPath string, now time.Time) (Campaign, error) {
	if strings.TrimSpace(slug) == "" {
		return Campaign{}, errors.New("slug is required")
	}
	if strings.TrimSpace(subject) == "" {
		return Campaign{}, errors.New("subject is required")
	}
	htmlBody, err := readRequiredFile(htmlPath)
	if err != nil {
		return Campaign{}, err
	}
	textBody, err := readRequiredFile(textPath)
	if err != nil {
		return Campaign{}, err
	}
	return store.UpsertCampaign(ctx, strings.TrimSpace(slug), strings.TrimSpace(subject), htmlBody, textBody, now.UTC())
}

func Preview(ctx context.Context, store Store, mailer Mailer, slug, to string) error {
	if strings.TrimSpace(to) == "" {
		return errors.New("preview recipient is required")
	}
	campaign, err := store.GetCampaignBySlug(ctx, strings.TrimSpace(slug))
	if err != nil {
		return err
	}
	_, err = mailer.SendCampaign(ctx, strings.TrimSpace(to), campaign.Subject, renderHTML(campaign.HTMLBody, "", true), renderText(campaign.TextBody, "", true))
	return err
}

func Send(ctx context.Context, store Store, mailer Mailer, opts SendOptions) (Summary, error) {
	if !opts.Confirm {
		return Summary{}, errors.New("campaign send requires --confirm")
	}
	if strings.TrimSpace(opts.Slug) == "" {
		return Summary{}, errors.New("slug is required")
	}
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "seed" {
		return Summary{}, fmt.Errorf("unsupported send mode %q", opts.Mode)
	}
	seedEmails := normalizeSeedEmails(opts.SeedEmails)
	if mode == "seed" && len(seedEmails) == 0 {
		return Summary{}, errors.New("seed mode requires at least one seed email")
	}
	if opts.PublicBaseURL == "" {
		return Summary{}, errors.New("public base URL is required")
	}
	if strings.TrimSpace(opts.LinkSecret) == "" {
		return Summary{}, errors.New("link secret is required")
	}
	if opts.Interval < 0 {
		return Summary{}, errors.New("interval must be non-negative")
	}
	if opts.StaleAfter <= 0 {
		opts.StaleAfter = 10 * time.Minute
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}

	campaign, err := store.GetCampaignBySlug(ctx, strings.TrimSpace(opts.Slug))
	if err != nil {
		return Summary{}, err
	}
	recipients, err := store.ListEligibleRecipients(ctx, campaign.ID, seedEmails)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Eligible: len(recipients)}

	for i, recipient := range recipients {
		now := opts.Now().UTC()
		claimed, err := store.ClaimCampaignSend(ctx, campaign.ID, recipient.SubscriberID, now, now.Add(-opts.StaleAfter))
		if err != nil {
			return summary, err
		}
		if !claimed {
			summary.Skipped++
			continue
		}

		summary.Attempted++
		unsubscribeURL := links.UnsubscribeURL(opts.PublicBaseURL, opts.LinkSecret, recipient.SubscriberID)
		messageID, err := mailer.SendCampaign(ctx, recipient.Email, campaign.Subject, renderHTML(campaign.HTMLBody, unsubscribeURL, false), renderText(campaign.TextBody, unsubscribeURL, false))
		if err != nil {
			summary.Failed++
			if recordErr := store.RecordCampaignSendFailure(ctx, campaign.ID, recipient.SubscriberID, err.Error(), opts.Now().UTC()); recordErr != nil {
				return summary, fmt.Errorf("record failed send: %w", recordErr)
			}
		} else {
			summary.Succeeded++
			if recordErr := store.RecordCampaignSendSuccess(ctx, campaign.ID, recipient.SubscriberID, messageID, opts.Now().UTC()); recordErr != nil {
				return summary, fmt.Errorf("record successful send: %w", recordErr)
			}
		}

		if opts.Interval > 0 && i < len(recipients)-1 {
			opts.Sleep(opts.Interval)
		}
	}

	if mode == "all" && summary.Failed == 0 {
		if err := store.MarkCampaignSent(ctx, campaign.ID, opts.Now().UTC()); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

func readRequiredFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("file path is required")
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	content := strings.TrimSpace(string(bytes))
	if content == "" {
		return "", fmt.Errorf("%s is empty", path)
	}
	return content, nil
}

func normalizeSeedEmails(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			normalized := strings.ToLower(strings.TrimSpace(part))
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out
}

func renderHTML(body, unsubscribeURL string, preview bool) string {
	footerTitle := "Manage your subscription"
	footerText := template.HTMLEscapeString("You are receiving this because you subscribed to newsletter updates.")
	cta := `<p style="margin:12px 0 0;font-size:13px;line-height:1.6;color:#86b2c0;">A recipient-specific unsubscribe link will be added when this campaign is sent live.</p>`
	if !preview && unsubscribeURL != "" {
		cta = fmt.Sprintf(`<p style="margin:12px 0 0;font-size:13px;line-height:1.6;color:#86b2c0;"><a href="%s" style="color:#86b2c0;">Unsubscribe instantly</a></p>`, template.HTMLEscapeString(unsubscribeURL))
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
</head>
<body style="margin:0;background:#0b0e14;color:#e0e5e9;font-family:Inter,-apple-system,Segoe UI,Roboto,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:24px 12px;background:#0b0e14;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:640px;background:#101722;border:1px solid rgba(255,255,255,0.12);border-radius:18px;overflow:hidden;">
          <tr>
            <td style="padding:32px 32px 18px;">%s</td>
          </tr>
          <tr>
            <td style="padding:18px 32px 28px;border-top:1px solid rgba(255,255,255,0.08);">
              <p style="margin:0;font-size:12px;letter-spacing:.10em;text-transform:uppercase;color:#93a0aa;">%s</p>
              <p style="margin:10px 0 0;font-size:13px;line-height:1.6;color:#90a0aa;">%s</p>
              %s
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, body, footerTitle, footerText, cta)
}

func renderText(body, unsubscribeURL string, preview bool) string {
	footer := "\n\nManage your subscription\nYou are receiving this because you subscribed to newsletter updates."
	if preview {
		footer += "\nA recipient-specific unsubscribe link will be added when this campaign is sent live."
	} else if unsubscribeURL != "" {
		footer += "\nUnsubscribe instantly: " + unsubscribeURL
	}
	return strings.TrimSpace(body) + footer + "\n"
}

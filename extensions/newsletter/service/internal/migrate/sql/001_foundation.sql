CREATE TABLE IF NOT EXISTS subscribers (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed', 'unsubscribed')),
  subscribed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  confirmed_at TIMESTAMPTZ,
  unsubscribed_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS subscribers_email_unique_idx
  ON subscribers (LOWER(email));

CREATE TABLE IF NOT EXISTS verification_tokens (
  id BIGSERIAL PRIMARY KEY,
  subscriber_id BIGINT NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE,
  token_hash BYTEA NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS verification_tokens_subscriber_id_idx
  ON verification_tokens (subscriber_id);

CREATE INDEX IF NOT EXISTS verification_tokens_expires_at_idx
  ON verification_tokens (expires_at);

CREATE TABLE IF NOT EXISTS campaigns (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  subject TEXT NOT NULL,
  html_body TEXT NOT NULL,
  text_body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  sent_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS campaign_sends (
  id BIGSERIAL PRIMARY KEY,
  campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
  subscriber_id BIGINT NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE,
  provider_message_id TEXT,
  status TEXT NOT NULL,
  error_text TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(campaign_id, subscriber_id)
);

CREATE INDEX IF NOT EXISTS campaign_sends_campaign_id_idx
  ON campaign_sends (campaign_id);

CREATE INDEX IF NOT EXISTS campaign_sends_subscriber_id_idx
  ON campaign_sends (subscriber_id);

ALTER TABLE campaign_sends
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE campaign_sends
  ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0;

UPDATE campaign_sends
SET updated_at = COALESCE(updated_at, created_at, NOW()),
    attempt_count = CASE WHEN attempt_count > 0 THEN attempt_count ELSE 1 END;

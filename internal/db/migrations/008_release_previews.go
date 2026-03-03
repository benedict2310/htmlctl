package migrations

const releasePreviewsMigrationSQL = `
CREATE TABLE IF NOT EXISTS release_previews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    release_id TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_release_previews_env
    ON release_previews(environment_id);

CREATE INDEX IF NOT EXISTS idx_release_previews_expires_at
    ON release_previews(expires_at);
`

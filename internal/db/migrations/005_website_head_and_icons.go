package migrations

const websiteHeadAndIconsMigrationSQL = `
ALTER TABLE websites ADD COLUMN head_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE websites ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS website_icons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    slot TEXT NOT NULL CHECK(slot IN ('svg', 'ico', 'apple_touch')),
    source_path TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, slot)
);
CREATE INDEX IF NOT EXISTS idx_website_icons_website_id ON website_icons(website_id);
`

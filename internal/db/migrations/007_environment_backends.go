package migrations

const environmentBackendsMigrationSQL = `
CREATE TABLE IF NOT EXISTS environment_backends (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    path_prefix TEXT NOT NULL,
    upstream TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(environment_id, path_prefix)
);

CREATE INDEX IF NOT EXISTS idx_environment_backends_environment_id
    ON environment_backends(environment_id);
`

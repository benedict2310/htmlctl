package migrations

const authPoliciesMigrationSQL = `
CREATE TABLE IF NOT EXISTS auth_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    path_prefix TEXT NOT NULL,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(environment_id, path_prefix)
);

CREATE INDEX IF NOT EXISTS idx_auth_policies_environment_id
    ON auth_policies(environment_id);
`

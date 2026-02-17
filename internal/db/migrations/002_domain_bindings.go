package migrations

const domainBindingsSchemaSQL = `
CREATE TABLE IF NOT EXISTS domain_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_domain_bindings_environment_id ON domain_bindings(environment_id);
`

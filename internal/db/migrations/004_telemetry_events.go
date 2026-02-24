package migrations

const telemetryEventsSchemaSQL = `
CREATE TABLE IF NOT EXISTS telemetry_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    event_name TEXT NOT NULL,
    path TEXT NOT NULL,
    occurred_at TEXT NULL,
    received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    session_id TEXT NULL,
    attrs_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_telemetry_events_env_received
    ON telemetry_events(environment_id, received_at);
CREATE INDEX IF NOT EXISTS idx_telemetry_events_env_name_received
    ON telemetry_events(environment_id, event_name, received_at);
`

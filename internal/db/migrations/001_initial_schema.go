package migrations

type Migration struct {
	Version int
	Name    string
	UpSQL   string
}

const initialSchemaSQL = `
CREATE TABLE IF NOT EXISTS websites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    default_style_bundle TEXT NOT NULL DEFAULT 'default',
    base_template TEXT NOT NULL DEFAULT 'default',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    active_release_id TEXT REFERENCES releases(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, name)
);

CREATE TABLE IF NOT EXISTS pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    route TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    layout_json TEXT NOT NULL DEFAULT '[]',
    content_hash TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, name),
    UNIQUE(website_id, route)
);

CREATE TABLE IF NOT EXISTS components (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'global',
    content_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, name)
);

CREATE TABLE IF NOT EXISTS style_bundles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    files_json TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, name)
);

CREATE TABLE IF NOT EXISTS assets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, filename)
);

CREATE TABLE IF NOT EXISTS releases (
    id TEXT PRIMARY KEY,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE RESTRICT,
    manifest_json TEXT NOT NULL,
    output_hashes TEXT NOT NULL DEFAULT '{}',
    build_log TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'building',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor TEXT NOT NULL DEFAULT 'system',
    timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    environment_id INTEGER REFERENCES environments(id) ON DELETE SET NULL,
    operation TEXT NOT NULL,
    resource_summary TEXT NOT NULL DEFAULT '',
    release_id TEXT REFERENCES releases(id) ON DELETE SET NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_environments_website_id ON environments(website_id);
CREATE INDEX IF NOT EXISTS idx_pages_website_id ON pages(website_id);
CREATE INDEX IF NOT EXISTS idx_components_website_id ON components(website_id);
CREATE INDEX IF NOT EXISTS idx_style_bundles_website_id ON style_bundles(website_id);
CREATE INDEX IF NOT EXISTS idx_assets_website_id ON assets(website_id);
CREATE INDEX IF NOT EXISTS idx_releases_environment_id ON releases(environment_id);
CREATE INDEX IF NOT EXISTS idx_releases_created_at ON releases(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_environment_id ON audit_log(environment_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_release_id ON audit_log(release_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
`

func All() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "initial_schema",
			UpSQL:   initialSchemaSQL,
		},
		{
			Version: 2,
			Name:    "domain_bindings",
			UpSQL:   domainBindingsSchemaSQL,
		},
		{
			Version: 3,
			Name:    "pages_head_metadata",
			UpSQL:   pagesHeadMetadataMigrationSQL,
		},
		{
			Version: 4,
			Name:    "telemetry_events",
			UpSQL:   telemetryEventsSchemaSQL,
		},
		{
			Version: 5,
			Name:    "website_head_and_icons",
			UpSQL:   websiteHeadAndIconsMigrationSQL,
		},
		{
			Version: 6,
			Name:    "website_seo",
			UpSQL:   websiteSEOMigrationSQL,
		},
		{
			Version: 7,
			Name:    "environment_backends",
			UpSQL:   environmentBackendsMigrationSQL,
		},
	}
}

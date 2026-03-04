package migrations

const componentFragmentsMigrationSQL = `
ALTER TABLE components ADD COLUMN css_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE components ADD COLUMN js_hash TEXT NOT NULL DEFAULT '';
`

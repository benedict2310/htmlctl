package migrations

const websiteSEOMigrationSQL = `
ALTER TABLE websites ADD COLUMN seo_json TEXT NOT NULL DEFAULT '{}';
`

package migrations

const pagesHeadMetadataMigrationSQL = `
ALTER TABLE pages ADD COLUMN head_json TEXT NOT NULL DEFAULT '{}';
`

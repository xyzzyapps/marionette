// @tag-block architecture
// @tag layer: "persistence"
// @tag domain: "tagdoc-core"
// @tag depends-on: []
// @tag description: "SQLite schema DDL v2.0.0 — tags, files, tag_properties, call_edges, symbols, analytics_snapshots tables"
package storage

const schemaVersion = "2.0.0"

const schemaDDL = `
CREATE TABLE IF NOT EXISTS index_metadata (
    key   TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT NOT NULL UNIQUE,
    language   TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    md5_hash   TEXT NOT NULL DEFAULT '',
    tags_count INTEGER NOT NULL DEFAULT 0,
    indexed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_hash ON files(md5_hash);
CREATE INDEX IF NOT EXISTS idx_files_lang ON files(language);

CREATE TABLE IF NOT EXISTS tags (
    id          TEXT PRIMARY KEY NOT NULL,
    file_id     INTEGER NOT NULL,
    language    TEXT NOT NULL,
    tag_type    TEXT NOT NULL CHECK(tag_type IN ('block', 'inline')),
    entity_name TEXT,
    entity_kind TEXT,
    start_line  INTEGER NOT NULL,
    start_col   INTEGER NOT NULL,
    end_line    INTEGER NOT NULL,
    end_col     INTEGER NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tags_file ON tags(file_id);
CREATE INDEX IF NOT EXISTS idx_tags_type ON tags(tag_type);
CREATE INDEX IF NOT EXISTS idx_tags_entity ON tags(entity_kind);

CREATE TABLE IF NOT EXISTS tag_properties (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id   TEXT NOT NULL,
    key      TEXT NOT NULL,
    value    TEXT NOT NULL,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_props_tag ON tag_properties(tag_id);
CREATE INDEX IF NOT EXISTS idx_props_key ON tag_properties(key);
CREATE INDEX IF NOT EXISTS idx_props_value ON tag_properties(value);
CREATE INDEX IF NOT EXISTS idx_props_key_value ON tag_properties(key, value);

-- Call graph tables
CREATE TABLE IF NOT EXISTS symbols (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id   INTEGER NOT NULL,
    name      TEXT NOT NULL,
    kind      TEXT NOT NULL,
    line      INTEGER NOT NULL,
    language  TEXT NOT NULL,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);

CREATE TABLE IF NOT EXISTS call_edges (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    caller_file   INTEGER NOT NULL,
    caller_symbol TEXT NOT NULL,
    callee_file   INTEGER NOT NULL,
    callee_symbol TEXT NOT NULL,
    call_line     INTEGER NOT NULL,
    FOREIGN KEY (caller_file) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (callee_file) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_calls_caller ON call_edges(caller_symbol);
CREATE INDEX IF NOT EXISTS idx_calls_callee ON call_edges(callee_symbol);

-- Analytics snapshots
CREATE TABLE IF NOT EXISTS analytics_snapshots (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp  TEXT NOT NULL DEFAULT (datetime('now')),
    total_tags INTEGER NOT NULL,
    total_files INTEGER NOT NULL,
    by_language TEXT,
    by_kind    TEXT,
    by_key     TEXT
);
`

const (
	qGetFileHash = `SELECT id, md5_hash FROM files WHERE path = ?`
	qDeleteFileTags = `DELETE FROM tag_properties WHERE tag_id IN (SELECT id FROM tags WHERE file_id = ?)`
	qDeleteTags     = `DELETE FROM tags WHERE file_id = ?`
	qUpsertFile = `
		INSERT INTO files (path, language, size, md5_hash, tags_count, indexed_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(path) DO UPDATE SET
			language    = excluded.language,
			size        = excluded.size,
			md5_hash    = excluded.md5_hash,
			tags_count  = excluded.tags_count,
			indexed_at  = excluded.indexed_at`
	qInsertTag = `
		INSERT INTO tags (id, file_id, language, tag_type, entity_name, entity_kind,
		                  start_line, start_col, end_line, end_col, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`
	qInsertProp = `INSERT INTO tag_properties (tag_id, key, value) VALUES (?, ?, ?)`
	qGetMeta    = `SELECT value FROM index_metadata WHERE key = ?`
	qSetMeta    = `INSERT OR REPLACE INTO index_metadata (key, value) VALUES (?, ?)`

	// Call graph queries
	qInsertSymbol = `INSERT OR IGNORE INTO symbols (file_id, name, kind, line, language) VALUES (?, ?, ?, ?, ?)`
	qInsertCall   = `INSERT OR IGNORE INTO call_edges (caller_file, caller_symbol, callee_file, callee_symbol, call_line) VALUES (?, ?, ?, ?, ?)`

	// Analytics
	qInsertSnapshot = `INSERT INTO analytics_snapshots (total_tags, total_files, by_language, by_kind, by_key) VALUES (?, ?, ?, ?, ?)`
	qGetSnapshots   = `SELECT timestamp, total_tags, total_files FROM analytics_snapshots ORDER BY timestamp DESC LIMIT ?`
)

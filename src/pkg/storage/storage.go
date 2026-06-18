// Package storage provides an embedded SQLite-backed index for Marionette.
//
// It supports incremental indexing via MD5 hash caching: files that haven't
// changed since the last index are skipped automatically.
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/xyzzyapps/marionette/src/pkg/models"

	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed persistent index for tags and file metadata.
type Store struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

// NewStore opens or creates a Marionette index database at the given path.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{
		db:   db,
		path: dbPath,
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return store, nil
}

// initialize creates the schema and sets metadata.
func (s *Store) initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(schemaDDL)
	if err != nil {
		return fmt.Errorf("schema creation failed: %w", err)
	}

	var existing string
	err = s.db.QueryRow(qGetMeta, "schema_version").Scan(&existing)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec(qSetMeta, "schema_version", schemaVersion)
		if err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// GetFileHash retrieves the stored MD5 hash and file ID for a given path.
func (s *Store) GetFileHash(path string) (fileID int64, hash string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(qGetFileHash, path)
	err = row.Scan(&fileID, &hash)
	if err == sql.ErrNoRows {
		return 0, "", nil
	}
	return fileID, hash, err
}

// FileNeedsIndexing checks whether a file needs re-indexing by comparing MD5 hashes.
func (s *Store) FileNeedsIndexing(path, currentHash string) (bool, error) {
	_, storedHash, err := s.GetFileHash(path)
	if err != nil {
		return false, err
	}
	return storedHash != currentHash, nil
}

// BeginTx starts a transaction for bulk indexing operations.
func (s *Store) BeginTx() (*sql.Tx, error) {
	return s.db.Begin()
}

// ReplaceFileTags atomically replaces all tags for a file within a transaction.
func (s *Store) ReplaceFileTags(tx *sql.Tx, fileID int64, tags []models.Tag) error {
	// Delete existing tags and properties
	if _, err := tx.Exec(qDeleteFileTags, fileID); err != nil {
		return fmt.Errorf("failed to delete old properties: %w", err)
	}
	if _, err := tx.Exec(qDeleteTags, fileID); err != nil {
		return fmt.Errorf("failed to delete old tags: %w", err)
	}

	for _, tag := range tags {
		var entityName, entityKind *string
		if tag.EntityName != nil {
			entityName = tag.EntityName
		}
		if tag.EntityKind != nil {
			entityKind = tag.EntityKind
		}

		_, err := tx.Exec(qInsertTag,
			tag.ID, fileID, tag.Language, string(tag.TagType),
			entityName, entityKind,
			int64(tag.Range.Start.Line), int64(tag.Range.Start.Column),
			int64(tag.Range.End.Line), int64(tag.Range.End.Column),
		)
		if err != nil {
			return fmt.Errorf("failed to insert tag %s: %w", tag.ID, err)
		}

		for k, v := range tag.Metadata {
			valStr := fmt.Sprintf("%v", v)
			if _, err := tx.Exec(qInsertProp, tag.ID, k, valStr); err != nil {
				return fmt.Errorf("failed to insert property %s: %w", k, err)
			}
		}
	}

	return nil
}

// UpsertFile inserts or updates a file entry, returning the file ID.
func (s *Store) UpsertFile(tx *sql.Tx, entry models.FileEntry) (int64, error) {
	result, err := tx.Exec(qUpsertFile,
		entry.Path, entry.Language, entry.Size, entry.MD5Hash, entry.TagsCount,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert file: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	if id == 0 {
		row := tx.QueryRow("SELECT id FROM files WHERE path = ?", entry.Path)
		if err := row.Scan(&id); err != nil {
			return 0, fmt.Errorf("failed to fetch file ID: %w", err)
		}
	}

	return id, nil
}

// GetStats returns index metadata statistics.
func (s *Store) GetStats() (*models.IndexMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta := &models.IndexMetadata{
		Version: schemaVersion,
	}

	s.mu.RUnlock()
	// Count files
	if err := s.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&meta.TotalFiles); err != nil {
		return nil, err
	}
	// Count tags
	if err := s.db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&meta.TotalTags); err != nil {
		return nil, err
	}
	s.mu.RLock()

	var createdStr, updatedStr string
	s.db.QueryRow("SELECT value FROM index_metadata WHERE key = 'created_at'").Scan(&createdStr)
	s.db.QueryRow("SELECT value FROM index_metadata WHERE key = 'updated_at'").Scan(&updatedStr)

	if createdStr != "" {
		meta.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	}
	if updatedStr != "" {
		meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	}

	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
		s.db.Exec("INSERT OR REPLACE INTO index_metadata (key, value) VALUES ('created_at', ?)",
			meta.CreatedAt.Format(time.RFC3339))
	}
	meta.UpdatedAt = time.Now()
	s.db.Exec("INSERT OR REPLACE INTO index_metadata (key, value) VALUES ('updated_at', ?)",
		meta.UpdatedAt.Format(time.RFC3339))

	return meta, nil
}

// QueryTags searches for tags matching the given query parameters.
func (s *Store) QueryTags(q models.Query) ([]models.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	where := "WHERE 1=1"
	args := []interface{}{}

	if q.TagType != nil {
		where += " AND t.tag_type = ?"
		args = append(args, string(*q.TagType))
	}
	if q.Language != nil {
		where += " AND t.language = ?"
		args = append(args, *q.Language)
	}
	if q.EntityKind != nil {
		where += " AND t.entity_kind = ?"
		args = append(args, *q.EntityKind)
	}

	query := fmt.Sprintf(`
		SELECT t.id, f.path, t.language, t.tag_type, t.entity_name, t.entity_kind,
		       t.start_line, t.start_col, t.end_line, t.end_col, t.created_at, t.updated_at
		FROM tags t
		JOIN files f ON f.id = t.file_id
		%s
		ORDER BY f.path, t.start_line`, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// First pass: collect all tags with their IDs
	type rawTag struct {
		models.Tag
		createdAtStr string
		updatedAtStr string
	}
	var rawTags []rawTag

	for rows.Next() {
		var rt rawTag
		var entityName, entityKind sql.NullString

		err := rows.Scan(&rt.ID, &rt.File, &rt.Language, &rt.TagType,
			&entityName, &entityKind,
			&rt.Range.Start.Line, &rt.Range.Start.Column,
			&rt.Range.End.Line, &rt.Range.End.Column,
			&rt.createdAtStr, &rt.updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		if entityName.Valid {
			rt.EntityName = &entityName.String
		}
		if entityKind.Valid {
			rt.EntityKind = &entityKind.String
		}

		rawTags = append(rawTags, rt)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Second pass: fetch properties for all tags (separate queries)
	var idList []string
	idIndex := make(map[string]int)
	for i, rt := range rawTags {
		idList = append(idList, rt.ID)
		idIndex[rt.ID] = i
	}

	// Fetch all properties in one query
	if len(idList) > 0 {
		placeholders := ""
		propArgs := []interface{}{}
		for _, id := range idList {
			if placeholders != "" {
				placeholders += ","
			}
			placeholders += "?"
			propArgs = append(propArgs, id)
		}

		propRows, err := s.db.Query(
			"SELECT tag_id, key, value FROM tag_properties WHERE tag_id IN ("+placeholders+") ORDER BY tag_id",
			propArgs...,
		)
		if err == nil {
			for propRows.Next() {
				var tagID, k, v string
				if err := propRows.Scan(&tagID, &k, &v); err != nil {
					propRows.Close()
					break
				}
				if idx, ok := idIndex[tagID]; ok {
					if rawTags[idx].Metadata == nil {
						rawTags[idx].Metadata = make(map[string]interface{})
					}
					rawTags[idx].Metadata[k] = v
				}
			}
			propRows.Close()
		}
	}

	// Convert raw tags to final tags
	tags := make([]models.Tag, 0, len(rawTags))
	for _, rt := range rawTags {
		rt.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", rt.createdAtStr)
		rt.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", rt.updatedAtStr)
		if rt.Metadata == nil {
			rt.Metadata = make(map[string]interface{})
		}
		tags = append(tags, rt.Tag)
	}

	// Apply Kind filter
	if q.Kind != nil {
		var filtered []models.Tag
		for _, tag := range tags {
			if kind, ok := tag.Metadata["kind"]; ok {
				if fmt.Sprintf("%v", kind) == *q.Kind {
					filtered = append(filtered, tag)
				}
			}
		}
		tags = filtered
	}

	// Apply Key/Value filters
	if q.Key != nil {
		var filtered []models.Tag
		for _, tag := range tags {
			if val, ok := tag.Metadata[*q.Key]; ok {
				if q.Value == nil || fmt.Sprintf("%v", val) == *q.Value {
					filtered = append(filtered, tag)
				}
			}
		}
		tags = filtered
	}

	return tags, nil
}

// ExportJSON exports the entire index as JSON.
func (s *Store) ExportJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, err := s.GetStats()
	if err != nil {
		return nil, err
	}

	tags, err := s.QueryTags(models.Query{})
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query("SELECT path, language, size, md5_hash, tags_count, indexed_at FROM files ORDER BY path")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.FileEntry
	for rows.Next() {
		var f models.FileEntry
		var indexedAt string
		if err := rows.Scan(&f.Path, &f.Language, &f.Size, &f.MD5Hash, &f.TagsCount, &indexedAt); err != nil {
			return nil, err
		}
		f.IndexedAt, _ = time.Parse("2006-01-02 15:04:05", indexedAt)
		files = append(files, f)
	}

	output := models.IndexOutput{
		Metadata: *meta,
		Tags:     tags,
		Files:    files,
	}

	return json.MarshalIndent(output, "", "  ")
}

// WalkFiles returns all indexed file paths.
func (s *Store) WalkFiles() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT path FROM files ORDER BY path")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// AddTag inserts a manually created tag into the index.
func (s *Store) AddTag(tag models.Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find or create file entry
	var fileID int64
	row := s.db.QueryRow("SELECT id FROM files WHERE path = ?", tag.File)
	if err := row.Scan(&fileID); err != nil {
		// Create file entry
		res, err := s.db.Exec(qUpsertFile,
			tag.File, tag.Language, 0, "", 0)
		if err != nil {
			return fmt.Errorf("failed to create file entry: %w", err)
		}
		fileID, _ = res.LastInsertId()
		if fileID == 0 {
			row := s.db.QueryRow("SELECT id FROM files WHERE path = ?", tag.File)
			row.Scan(&fileID)
		}
	}

	var entityName, entityKind *string
	if tag.EntityName != nil {
		entityName = tag.EntityName
	}
	if tag.EntityKind != nil {
		entityKind = tag.EntityKind
	}

	_, err := s.db.Exec(qInsertTag,
		tag.ID, fileID, tag.Language, string(tag.TagType),
		entityName, entityKind,
		int64(tag.Range.Start.Line), int64(tag.Range.Start.Column),
		int64(tag.Range.End.Line), int64(tag.Range.End.Column))
	if err != nil {
		return fmt.Errorf("failed to insert tag: %w", err)
	}

	for k, v := range tag.Metadata {
		valStr := fmt.Sprintf("%v", v)
		if _, err := s.db.Exec(qInsertProp, tag.ID, k, valStr); err != nil {
			return fmt.Errorf("failed to insert property: %w", err)
		}
	}

	// Update tags_count
	s.db.Exec("UPDATE files SET tags_count = (SELECT COUNT(*) FROM tags WHERE file_id = ?) WHERE id = ?", fileID, fileID)
	return nil
}

// DeleteTag removes a tag from the index by its ID.
func (s *Store) DeleteTag(tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get file_id before deleting
	var fileID int64
	row := s.db.QueryRow("SELECT file_id FROM tags WHERE id = ?", tagID)
	if err := row.Scan(&fileID); err != nil {
		return fmt.Errorf("tag %s not found: %w", tagID, err)
	}

	if _, err := s.db.Exec("DELETE FROM tag_properties WHERE tag_id = ?", tagID); err != nil {
		return err
	}
	if _, err := s.db.Exec("DELETE FROM tags WHERE id = ?", tagID); err != nil {
		return err
	}

	if fileID > 0 {
		s.db.Exec("UPDATE files SET tags_count = (SELECT COUNT(*) FROM tags WHERE file_id = ?) WHERE id = ?", fileID, fileID)
	}
	return nil
}

// TagStats returns aggregated statistics about tags in the index.
type TagStats struct {
	TotalTags    int
	ByKey        map[string]int
	ByKind       map[string]int
	ByLanguage   map[string]int
	ByTagType    map[string]int
	ByEntityKind map[string]int
}

// GetTagStats computes aggregated tag statistics.
func (s *Store) GetTagStats() (*TagStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &TagStats{
		ByKey:        make(map[string]int),
		ByKind:       make(map[string]int),
		ByLanguage:   make(map[string]int),
		ByTagType:    make(map[string]int),
		ByEntityKind: make(map[string]int),
	}

	s.db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&stats.TotalTags)

	// By language
	rows, _ := s.db.Query("SELECT language, COUNT(*) FROM tags GROUP BY language ORDER BY COUNT(*) DESC")
	if rows != nil {
		for rows.Next() {
			var lang string
			var count int
			rows.Scan(&lang, &count)
			stats.ByLanguage[lang] = count
		}
		rows.Close()
	}

	// By tag_type
	rows, _ = s.db.Query("SELECT tag_type, COUNT(*) FROM tags GROUP BY tag_type")
	if rows != nil {
		for rows.Next() {
			var tt string
			var count int
			rows.Scan(&tt, &count)
			stats.ByTagType[tt] = count
		}
		rows.Close()
	}

	// By entity_kind
	rows, _ = s.db.Query("SELECT entity_kind, COUNT(*) FROM tags WHERE entity_kind IS NOT NULL GROUP BY entity_kind ORDER BY COUNT(*) DESC LIMIT 20")
	if rows != nil {
		for rows.Next() {
			var ek string
			var count int
			rows.Scan(&ek, &count)
			stats.ByEntityKind[ek] = count
		}
		rows.Close()
	}

	// By kind (from tag_properties)
	rows, _ = s.db.Query("SELECT value, COUNT(*) FROM tag_properties WHERE key = 'kind' GROUP BY value ORDER BY COUNT(*) DESC")
	if rows != nil {
		for rows.Next() {
			var kind string
			var count int
			rows.Scan(&kind, &count)
			stats.ByKind[kind] = count
		}
		rows.Close()
	}

	// Top 20 keys
	rows, _ = s.db.Query("SELECT key, COUNT(DISTINCT tag_id) FROM tag_properties GROUP BY key ORDER BY COUNT(DISTINCT tag_id) DESC LIMIT 20")
	if rows != nil {
		for rows.Next() {
			var key string
			var count int
			rows.Scan(&key, &count)
			stats.ByKey[key] = count
		}
		rows.Close()
	}

	return stats, nil
}

// CallerEntry represents a single caller found via the call graph.
type CallerEntry struct {
	CallerSymbol string `json:"caller_symbol"`
	File         string `json:"file"`
	CallLine     int    `json:"call_line"`
}

// CalleeEntry represents a single callee found via the call graph.
type CalleeEntry struct {
	CalleeSymbol string `json:"callee_symbol"`
	File         string `json:"file"`
	CallLine     int    `json:"call_line"`
}

// QueryCallers returns all functions that call the given symbol.
func (s *Store) QueryCallers(symbol string) ([]CallerEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT ce.caller_symbol, f.path, ce.call_line
		FROM call_edges ce
		JOIN files f ON f.id = ce.caller_file
		WHERE ce.callee_symbol = ?
		ORDER BY ce.caller_symbol`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []CallerEntry
	for rows.Next() {
		var e CallerEntry
		if err := rows.Scan(&e.CallerSymbol, &e.File, &e.CallLine); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// QueryCallees returns all functions that the given symbol calls.
func (s *Store) QueryCallees(symbol string) ([]CalleeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT ce.callee_symbol, f.path, ce.call_line
		FROM call_edges ce
		JOIN files f ON f.id = ce.callee_file
		WHERE ce.caller_symbol = ?
		ORDER BY ce.callee_symbol`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []CalleeEntry
	for rows.Next() {
		var e CalleeEntry
		if err := rows.Scan(&e.CalleeSymbol, &e.File, &e.CallLine); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// SearchResult represents a FTS5 search hit.
type SearchResult struct {
	TagID string  `json:"tag_id"`
	File  string  `json:"file"`
	Key   string  `json:"key"`
	Value string  `json:"value"`
	Score float64 `json:"score"`
}

// SearchTags performs full-text search on tag metadata using LIKE queries.
func (s *Store) SearchTags(query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := "%" + query + "%"
	sqlQuery := `SELECT DISTINCT tp.tag_id, f.path, tp.key, tp.value
		FROM tag_properties tp
		JOIN tags t ON t.id = tp.tag_id
		JOIN files f ON f.id = t.file_id
		WHERE tp.key LIKE ? OR tp.value LIKE ?
		LIMIT ?`

	rows, err := s.db.Query(sqlQuery, q, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.TagID, &r.File, &r.Key, &r.Value); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// Snapshot represents a recorded analytics snapshot.
type Snapshot struct {
	Timestamp  string
	TotalTags  int
	TotalFiles int
}

// RecordSnapshot saves an analytics snapshot.
func (s *Store) RecordSnapshot(totalTags, totalFiles int, byLang, byKind, byKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(qInsertSnapshot, totalTags, totalFiles, byLang, byKind, byKey)
}

// GetSnapshots returns recent analytics snapshots.
func (s *Store) GetSnapshots(limit int) ([]Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(qGetSnapshots, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.Timestamp, &s.TotalTags, &s.TotalFiles); err != nil {
			continue
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// GetFileTags returns all tags for a specific file (used by LSP).
func (s *Store) GetFileTags(filePath string) ([]models.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT t.id, f.path, t.language, t.tag_type, t.entity_name, t.entity_kind,
		       t.start_line, t.start_col, t.end_line, t.end_col, t.created_at, t.updated_at
		FROM tags t
		JOIN files f ON f.id = t.file_id
		WHERE f.path = ?
		ORDER BY t.start_line`, absPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawTag struct {
		models.Tag
		createdAtStr string
		updatedAtStr string
	}
	var rawTags []rawTag

	for rows.Next() {
		var rt rawTag
		var entityName, entityKind sql.NullString

		err := rows.Scan(&rt.ID, &rt.File, &rt.Language, &rt.TagType,
			&entityName, &entityKind,
			&rt.Range.Start.Line, &rt.Range.Start.Column,
			&rt.Range.End.Line, &rt.Range.End.Column,
			&rt.createdAtStr, &rt.updatedAtStr)
		if err != nil {
			return nil, err
		}

		if entityName.Valid {
			rt.EntityName = &entityName.String
		}
		if entityKind.Valid {
			rt.EntityKind = &entityKind.String
		}
		rawTags = append(rawTags, rt)
	}
	rows.Close()

	// Fetch properties
	for i, rt := range rawTags {
		propRows, err := s.db.Query("SELECT key, value FROM tag_properties WHERE tag_id = ?", rt.ID)
		if err != nil {
			continue
		}
		for propRows.Next() {
			var k, v string
			if err := propRows.Scan(&k, &v); err != nil {
				propRows.Close()
				break
			}
			if rawTags[i].Metadata == nil {
				rawTags[i].Metadata = make(map[string]interface{})
			}
			rawTags[i].Metadata[k] = v
		}
		propRows.Close()
	}

	tags := make([]models.Tag, len(rawTags))
	for i, rt := range rawTags {
		rt.Tag.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", rt.createdAtStr)
		rt.Tag.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", rt.updatedAtStr)
		tags[i] = rt.Tag
	}

	return tags, nil
}

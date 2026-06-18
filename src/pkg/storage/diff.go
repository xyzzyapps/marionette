// @tag-block architecture
// @tag layer: "persistence"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["storage"]
// @tag description: "Branch diff between two SQLite databases — identifies added, removed, and changed tags with key/kind filters"
package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
	"github.com/xyzzyapps/marionette/src/pkg/models"
)

// DiffResult represents a tag that differs between two index databases.
type DiffResult struct {
	Tag       models.Tag
	Status    string // "added", "removed", "changed"
	OldValue  string
	NewValue  string
}

// Diff compares tags with a specific key between two databases.
// Returns tags that were added, removed, or changed.
func Diff(dbA, dbB string, key, kind string) ([]DiffResult, error) {
	db1, err := sql.Open("sqlite", dbA)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", dbA, err)
	}
	defer db1.Close()

	db2, err := sql.Open("sqlite", dbB)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", dbB, err)
	}
	defer db2.Close()

	// Query all tags from both databases
	tagsA, err := queryAllTags(db1, key, kind)
	if err != nil {
		return nil, err
	}
	tagsB, err := queryAllTags(db2, key, kind)
	if err != nil {
		return nil, err
	}

	// Build maps for comparison
	mapA := make(map[string]string)
	for _, t := range tagsA {
		for k, v := range t.Metadata {
			mapA[t.File+":"+k] = fmt.Sprintf("%v", v)
		}
	}
	mapB := make(map[string]string)
	for _, t := range tagsB {
		for k, v := range t.Metadata {
			mapB[t.File+":"+k] = fmt.Sprintf("%v", v)
		}
	}

	var results []DiffResult

	// Find added and changed
	for keyB, valB := range mapB {
		if valA, ok := mapA[keyB]; ok {
			if valA != valB {
				results = append(results, DiffResult{
					Status:   "changed",
					OldValue: valA,
					NewValue: valB,
				})
			}
		} else {
			results = append(results, DiffResult{Status: "added", NewValue: valB})
		}
	}

	// Find removed
	for keyA := range mapA {
		if _, ok := mapB[keyA]; !ok {
			results = append(results, DiffResult{Status: "removed", OldValue: mapA[keyA]})
		}
	}

	return results, nil
}

func queryAllTags(db *sql.DB, key, kind string) ([]models.Tag, error) {
	where := "WHERE 1=1"
	args := []interface{}{}

	if key != "" {
		where += " AND tp.key = ?"
		args = append(args, key)
	}
	if kind != "" {
		where += " AND tp.value = ? AND tp.key = 'kind'"
		args = append(args, kind)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT t.id, f.path, t.language, t.tag_type, t.entity_name, t.entity_kind,
		       t.start_line, t.start_col, t.end_line, t.end_col
		FROM tags t
		JOIN files f ON f.id = t.file_id
		LEFT JOIN tag_properties tp ON tp.tag_id = t.id
		%s
		ORDER BY f.path`, where)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []models.Tag
	tagMap := make(map[string]*models.Tag)

	for rows.Next() {
		var t models.Tag
		var en, ek sql.NullString
		err := rows.Scan(&t.ID, &t.File, &t.Language, &t.TagType,
			&en, &ek,
			&t.Range.Start.Line, &t.Range.Start.Column,
			&t.Range.End.Line, &t.Range.End.Column)
		if err != nil {
			return nil, err
		}
		if en.Valid { t.EntityName = &en.String }
		if ek.Valid { t.EntityKind = &ek.String }

		if existing, ok := tagMap[t.ID]; ok {
			t = *existing
		} else {
			t.Metadata = make(map[string]interface{})
			tagMap[t.ID] = &t
			tags = append(tags, t)
		}

		// Fetch properties for each tag
		propRows, err := db.Query("SELECT key, value FROM tag_properties WHERE tag_id = ?", t.ID)
		if err != nil {
			continue
		}
		for propRows.Next() {
			var k, v string
			propRows.Scan(&k, &v)
			tagMap[t.ID].Metadata[k] = v
		}
		propRows.Close()
	}
	return tags, nil
}

// @tag-block architecture
// @tag layer: "query"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["storage", "models"]
// @tag description: "Query engine — FindByKind, FindByKey, FindByKeyValue, FindByKeyHierarchy, FindReferences, file pattern expansion"
//
// Package query provides a structured query interface against the Marionette index.
//
// It wraps the storage layer with convenience methods for common query patterns
// used by both the CLI and LSP server.
package query

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xyzzyapps/marionette/src/pkg/models"
	"github.com/xyzzyapps/marionette/src/pkg/storage"
)

// Engine provides query operations against the tag index.
type Engine struct {
	store *storage.Store
}

// New creates a new query Engine.
func New(store *storage.Store) *Engine {
	return &Engine{store: store}
}

// Query runs a structured query against the index.
func (e *Engine) Query(q models.Query) ([]models.Tag, error) {
	// If file pattern is specified, expand it first
	if q.FilePattern != nil {
		return e.queryWithFilePattern(q)
	}
	return e.store.QueryTags(q)
}

// queryWithFilePattern filters results by file path pattern.
func (e *Engine) queryWithFilePattern(q models.Query) ([]models.Tag, error) {
	allTags, err := e.store.QueryTags(models.Query{
		Kind:       q.Kind,
		Key:        q.Key,
		Value:      q.Value,
		TagType:    q.TagType,
		Language:   q.Language,
		EntityKind: q.EntityKind,
	})
	if err != nil {
		return nil, err
	}

	pattern := *q.FilePattern
	var filtered []models.Tag
	for _, tag := range allTags {
		matched, err := filepath.Match(pattern, filepath.Base(tag.File))
		if err != nil {
			continue
		}
		if matched || strings.Contains(tag.File, pattern) {
			filtered = append(filtered, tag)
		}
	}
	return filtered, nil
}

// FindByKind returns all tags of a specific @tag-block kind.
func (e *Engine) FindByKind(kind string) ([]models.Tag, error) {
	return e.store.QueryTags(models.Query{Kind: &kind})
}

// FindByKey returns all tags that have a specific metadata key.
func (e *Engine) FindByKey(key string) ([]models.Tag, error) {
	return e.store.QueryTags(models.Query{Key: &key})
}

// FindByKeyValue returns all tags that have a specific key=value pair.
// Supports hierarchical key prefix matching with "/" (e.g., "area/billing"
// matches "area/billing/tax-calculation").
func (e *Engine) FindByKeyValue(key, value string) ([]models.Tag, error) {
	return e.store.QueryTags(models.Query{
		Key:   &key,
		Value: &value,
	})
}

// FindByKeyHierarchy returns all tags whose keys start with the given prefix.
// This enables hierarchical taxonomy queries: "area/billing" matches
// "area/billing/tax", "area/billing/invoice", etc.
func (e *Engine) FindByKeyHierarchy(prefix string) ([]models.Tag, error) {
	allTags, err := e.store.QueryTags(models.Query{})
	if err != nil {
		return nil, err
	}

	var filtered []models.Tag
	for _, tag := range allTags {
		for k := range tag.Metadata {
			if k == prefix || strings.HasPrefix(k, prefix+"/") {
				filtered = append(filtered, tag)
				break
			}
		}
	}
	return filtered, nil
}

// FindReferences finds all locations referencing a given @tag configuration.
// This is used by the LSP "Find References" feature.
func (e *Engine) FindReferences(key, value string) ([]models.Tag, error) {
	return e.FindByKeyValue(key, value)
}

// FindDefinition finds the primary definition of a tag by its ID.
func (e *Engine) FindDefinition(tagID string) (*models.Tag, error) {
	tags, err := e.store.QueryTags(models.Query{})
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if tag.ID == tagID {
			return &tag, nil
		}
	}
	return nil, fmt.Errorf("tag %s not found", tagID)
}

// GetFileTags retrieves all tags for a specific file.
func (e *Engine) GetFileTags(filePath string) ([]models.Tag, error) {
	return e.store.GetFileTags(filePath)
}

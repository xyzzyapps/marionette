// @tag-block architecture
// @tag layer: "domain"
// @tag domain: "tagdoc-core"
// @tag depends-on: []
// @tag description: "Core data types — Tag, Query, FileEntry, IndexMetadata — used by every layer"
//
// Package models defines the core data structures for the Marionette indexing system.
//
// A Tag represents a structured metadata annotation extracted from a @tag comment
// in source code. Tags come in two forms:
//   - Block tags: attached to a code block (function, class, method) via @tag-block
//   - Inline tags: attached to a specific expression/statement via @tag-inline
package models

import (
	"time"
)

// Position represents a zero-based position in a source file.
type Position struct {
	Line   uint32 `json:"line"`
	Column uint32 `json:"column"`
}

// Range represents a contiguous span in a source file (start inclusive, end exclusive by column).
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TagType indicates whether a tag is a block-level or inline annotation.
type TagType string

const (
	TagTypeBlock  TagType = "block"
	TagTypeInline TagType = "inline"
)

// Tag is the central data structure representing one extracted @tag annotation.
type Tag struct {
	// ID is a unique identifier for this tag (e.g., "tag_01j4k2").
	ID string `json:"id"`

	// File is the absolute or relative path to the source file.
	File string `json:"file"`

	// Language is the programming language of the source file (e.g., "javascript", "python", "rust").
	Language string `json:"language"`

	// TagType is either "block" or "inline".
	TagType TagType `json:"tag_type"`

	// EntityName is the name of the annotated code entity (e.g., function name, class name).
	// May be nil for inline tags that annotate expressions.
	EntityName *string `json:"entity_name"`

	// EntityKind describes the type of the annotated entity (e.g., "function_definition",
	// "class_definition", "expression_statement").
	EntityKind *string `json:"entity_kind"`

	// Range specifies the location of the annotated entity in the source file.
	Range Range `json:"range"`

	// Metadata holds the key-value pairs parsed from the @tag comment.
	// Values can be strings, numbers, booleans, or arrays of strings.
	Metadata map[string]interface{} `json:"metadata"`

	// CreatedAt records when this tag was indexed.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt records when this tag was last re-indexed.
	UpdatedAt time.Time `json:"updated_at"`
}

// FileEntry represents a source file tracked in the index.
type FileEntry struct {
	// Path is the file path relative to the project root.
	Path string `json:"path"`

	// Language is the detected programming language.
	Language string `json:"language"`

	// Size is the file size in bytes.
	Size int64 `json:"size"`

	// MD5Hash is the MD5 checksum of the file content used for cache invalidation.
	MD5Hash string `json:"md5_hash"`

	// TagsCount is the number of extracted tags in this file.
	TagsCount int `json:"tags_count"`

	// IndexedAt records when this file was last indexed.
	IndexedAt time.Time `json:"indexed_at"`
}

// IndexMetadata stores global metadata about the marionette index.
type IndexMetadata struct {
	// Version is the schema version of the index.
	Version string `json:"version"`

	// CreatedAt is when the index was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the index was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// TotalFiles is the total number of indexed files.
	TotalFiles int `json:"total_files"`

	// TotalTags is the total number of extracted tags.
	TotalTags int `json:"total_tags"`
}

// Query represents a structured query against the tag index.
type Query struct {
	// Kind filters by the @tag-block kind (e.g., "api-endpoint", "cron-job").
	Kind *string `json:"kind,omitempty"`

	// Key filters by a specific metadata key.
	Key *string `json:"key,omitempty"`

	// Value filters by a specific metadata value.
	Value *string `json:"value,omitempty"`

	// TagType filters by tag type ("block" or "inline").
	TagType *TagType `json:"tag_type,omitempty"`

	// Language filters by source language.
	Language *string `json:"language,omitempty"`

	// FilePattern filters by file path using glob patterns.
	FilePattern *string `json:"file_pattern,omitempty"`

	// EntityKind filters by the type of annotated entity.
	EntityKind *string `json:"entity_kind,omitempty"`
}

// IndexOutput is the complete index format for JSON export.
type IndexOutput struct {
	Metadata IndexMetadata `json:"metadata"`
	Tags     []Tag         `json:"tags"`
	Files    []FileEntry   `json:"files"`
}

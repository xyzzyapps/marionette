// @tag-block architecture
// @tag layer: "transform"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["parser", "models"]
// @tag description: "File extraction orchestrator — parses source, tokenizes @tag comments, builds Tag models with MD5 dedup"
package extractor

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/xyzzyapps/marionette/src/pkg/models"
	"github.com/xyzzyapps/marionette/src/pkg/parser"
)

// Extractor orchestrates the full extraction pipeline: parse source files with
// tree-sitter, extract comment-entity pairs, tokenize @tag syntax, and produce
// structured Tag models.
type Extractor struct {
	parser    *parser.Parser
	tokenizer *Tokenizer
	counter   uint64
}

// New creates a new Extractor.
func New(treeParser *parser.Parser) *Extractor {
	return &Extractor{
		parser:    treeParser,
		tokenizer: NewTokenizer(),
		counter:   0,
	}
}

// ExtractFile processes a single source file and returns extracted Tags.
func (e *Extractor) ExtractFile(path string) ([]models.Tag, *models.FileEntry, error) {
	lang, err := parser.DetectLanguage(path)
	if err != nil {
		return nil, nil, err
	}

	// Read file content for hashing and parsing
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	rawBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	fileHash := fmt.Sprintf("%x", md5.Sum(rawBytes))
	fileSize := int64(len(rawBytes))

	pairs, err := e.parser.ParseBytes(rawBytes, lang)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error for %s: %w", path, err)
	}

	langName := lang.Name

	var tags []models.Tag
	for _, pair := range pairs {
		parsedTags, err := e.tokenizer.ParseAll(pair.StrippedComment)
		if err != nil {
			continue
		}
		for _, pt := range parsedTags {
			tag := e.buildTag(path, langName, pt, pair)
			tags = append(tags, tag)
		}
	}

	fileEntry := &models.FileEntry{
		Path:      path,
		Language:  langName,
		Size:      fileSize,
		MD5Hash:   fileHash,
		TagsCount: len(tags),
		IndexedAt: time.Now(),
	}

	return tags, fileEntry, nil
}

// buildTag converts extracted parser/tokenizer results into a Tag model.
func (e *Extractor) buildTag(path, language string, pt *ParsedTag, pair parser.CommentEntityPair) models.Tag {
	tagType := models.TagTypeBlock
	if pt.TagType == "inline" {
		tagType = models.TagTypeInline
	}

	metadata := pt.Properties
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	if pt.Kind != "" {
		metadata["kind"] = pt.Kind
	}

	tag := models.Tag{
		ID:         e.generateTagID(),
		File:       path,
		Language:   language,
		TagType:    tagType,
		EntityName: pair.Entity.Name,
		EntityKind: &pair.Entity.Kind,
		Range: models.Range{
			Start: models.Position{
				Line:   pair.Entity.StartLine,
				Column: pair.Entity.StartColumn,
			},
			End: models.Position{
				Line:   pair.Entity.EndLine,
				Column: pair.Entity.EndColumn,
			},
		},
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return tag
}

// generateTagID creates a unique short ID for each tag using atomic counter.
func (e *Extractor) generateTagID() string {
	n := atomic.AddUint64(&e.counter, 1)
	r := rand.Intn(65536)
	return fmt.Sprintf("tag_%08x%04x", n, r)
}

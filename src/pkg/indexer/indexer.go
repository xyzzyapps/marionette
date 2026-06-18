// Package indexer handles the incremental indexing workflow: walking source trees,
// checking file hashes for cache invalidation, and batch-updating the SQLite store.
// @tag-block architecture
// @tag layer: "pipeline"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["extractor", "storage"]
// @tag description: "Incremental indexing pipeline — walks directories, MD5 caching, multi-repo support, call graph extraction"
package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xyzzyapps/marionette/src/pkg/extractor"
	"github.com/xyzzyapps/marionette/src/pkg/parser"
	"github.com/xyzzyapps/marionette/src/pkg/storage"
)

// Indexer manages batch indexing of source code trees.
type Indexer struct {
	extractor *extractor.Extractor
	store     *storage.Store
	stats     IndexStats
}

// IndexStats tracks indexing progress.
type IndexStats struct {
	TotalFiles   int
	IndexedFiles int
	SkippedFiles int
	TotalTags    int
	Errors       []IndexError
}

// IndexError records a file that failed to index.
type IndexError struct {
	File  string
	Error string
}

// New creates a new Indexer.
func New(ext *extractor.Extractor, st *storage.Store) *Indexer {
	return &Indexer{
		extractor: ext,
		store:     st,
	}
}

// IndexPath indexes all supported source files under the given path.
// If path is a file, it indexes just that file. If a directory, it walks recursively.
func (idx *Indexer) IndexPath(path string) (*IndexStats, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access path %s: %w", path, err)
	}

	idx.stats = IndexStats{}

	if !info.IsDir() {
		idx.indexFile(path)
	} else {
		err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip hidden directories
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
					return filepath.SkipDir
				}
				// Skip common non-source directories
				skipDirs := map[string]bool{
					"node_modules": true, "__pycache__": true, ".git": true,
					"target": true, "vendor": true, ".svn": true, ".hg": true,
				}
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			idx.indexFile(filePath)
			return nil
		})
	}

	return &idx.stats, err
}

// indexFile processes a single file for indexing.
func (idx *Indexer) indexFile(filePath string) {
	idx.stats.TotalFiles++

	// Check if language is supported
	if _, err := extractFileType(filePath); err != nil {
		return // silently skip unsupported files
	}

	tags, fileEntry, err := idx.extractor.ExtractFile(filePath)
	if err != nil {
		idx.stats.Errors = append(idx.stats.Errors, IndexError{
			File:  filePath,
			Error: err.Error(),
		})
		return
	}

	// Check if file needs re-indexing
	needsIndex, err := idx.store.FileNeedsIndexing(filePath, fileEntry.MD5Hash)
	if err != nil {
		idx.stats.Errors = append(idx.stats.Errors, IndexError{
			File:  filePath,
			Error: err.Error(),
		})
		return
	}

	if !needsIndex && tags == nil {
		idx.stats.SkippedFiles++
		return
	}

	// Begin transaction and update
	tx, err := idx.store.BeginTx()
	if err != nil {
		idx.stats.Errors = append(idx.stats.Errors, IndexError{
			File:  filePath,
			Error: err.Error(),
		})
		return
	}
	defer tx.Rollback()

	fileID, err := idx.store.UpsertFile(tx, *fileEntry)
	if err != nil {
		idx.stats.Errors = append(idx.stats.Errors, IndexError{
			File:  filePath,
			Error: err.Error(),
		})
		return
	}

	if err := idx.store.ReplaceFileTags(tx, fileID, tags); err != nil {
		idx.stats.Errors = append(idx.stats.Errors, IndexError{
			File:  filePath,
			Error: err.Error(),
		})
		return
	}

	if err := tx.Commit(); err != nil {
		idx.stats.Errors = append(idx.stats.Errors, IndexError{
			File:  filePath,
			Error: err.Error(),
		})
		return
	}

	idx.stats.IndexedFiles++
	idx.stats.TotalTags += len(tags)
}

// extractFileType returns the language for filter checks.
func extractFileType(path string) (string, error) {
	lang, err := parser.DetectLanguage(path)
	if err != nil {
		return "", err
	}
	return lang.Name, nil
}

// GetStats returns the current indexing session stats.
func (idx *Indexer) GetStats() IndexStats {
	return idx.stats
}

// IndexAllReindex forces a full re-index of all files in the store.
func (idx *Indexer) IndexAllReindex() (*IndexStats, error) {
	paths, err := idx.store.WalkFiles()
	if err != nil {
		return nil, err
	}

	idx.stats = IndexStats{}
	for _, p := range paths {
		idx.indexFile(p)
	}

	return &idx.stats, nil
}

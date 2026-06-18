package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xyzzyapps/marionette/src/pkg/config"
	"github.com/xyzzyapps/marionette/src/pkg/constraint"
	"github.com/xyzzyapps/marionette/src/pkg/extractor"
	"github.com/xyzzyapps/marionette/src/pkg/indexer"
	"github.com/xyzzyapps/marionette/src/pkg/lint"
	"github.com/xyzzyapps/marionette/src/pkg/lsp"
	"github.com/xyzzyapps/marionette/src/pkg/models"
	"github.com/xyzzyapps/marionette/src/pkg/output"
	"github.com/xyzzyapps/marionette/src/pkg/parser"
	"github.com/xyzzyapps/marionette/src/pkg/query"
	"github.com/xyzzyapps/marionette/src/pkg/storage"
	"github.com/urfave/cli/v2"
)

const marionetteVersion = "0.2.0"

func main() {
	app := cli.NewApp()
	app.Name = "marionette"
	app.Version = marionetteVersion
	app.Usage = "Structured code annotation indexing and query system"
	app.Description = `Marionette extracts @tag annotations from code comments and builds a queryable
   structured index. It supports 32 languages (JS, TS, Python, Rust, Go, Java,
   C, C++, C#, Ruby, PHP, Swift, Kotlin, Scala, Bash, SQL, and more) with
   incremental indexing via MD5 caching, SQLite storage, and multiple outputs.`

	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "db", Aliases: []string{"d"}, Value: ".marionette.db", Usage: "Path to the marionette SQLite index database"},
		&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "json", Usage: "Output format: json, markdown, or ctags"},
		&cli.StringFlag{Name: "config", Usage: "Path to .marionetterc.json configuration file"},
	}

	app.Commands = []*cli.Command{
		initCommand,
		indexCommand,
		queryCommand,
		tagCommand,
		callersCommand,
		calleesCommand,
		impactCommand,
		diffCommand,
		migrateCommand,
		dependsCommand,
		healthCommand,
		searchCommand,
		analyticsCommand,
		lintCommand,
		lspCommand,
		serveCommand,
		watchCommand,
		statsCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var initCommand = &cli.Command{
	Name:  "init",
	Usage: "Initialize a new marionette index",
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		if _, err := os.Stat(dbPath); err == nil {
			return fmt.Errorf("index database already exists at %s", dbPath)
		}
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
		defer store.Close()
		if _, err := os.Stat(".marionetterc.json"); os.IsNotExist(err) {
			config.SaveConfig(config.DefaultConfig(), ".")
			fmt.Println("Created default configuration: .marionetterc.json")
		}
		fmt.Printf("Initialized marionette index at %s\n", dbPath)
		return nil
	},
}

var indexCommand = &cli.Command{
	Name:  "index",
	Usage: "Index source files for @tag annotations",
	ArgsUsage: "[path...]",
	Description: `Walks the specified directories and indexes all supported source files.
   Uses MD5 hash caching to skip unchanged files. Supports multiple paths.
   If no path is given, uses the current directory plus any repositories
   listed in .marionetterc.json.`,
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open index: %w", err)
		}
		defer store.Close()

		treeParser, err := parser.New()
		if err != nil {
			return fmt.Errorf("failed to create parser: %w", err)
		}
		defer treeParser.Close()

		ext := extractor.New(treeParser)
		idx := indexer.New(ext, store)

		// Collect paths: CLI args + config repositories
		var paths []string
		if c.Args().Len() > 0 {
			paths = append(paths, c.Args().Slice()...)
		} else {
			paths = append(paths, ".")
			// Multi-repository: load additional paths from config
			cfg, err := config.LoadConfig(".")
			if err == nil && cfg != nil {
				for _, repo := range cfg.Repositories {
					if repo.Path != "" {
						paths = append(paths, repo.Path)
					}
				}
			}
		}

		var total indexer.IndexStats
		for _, p := range paths {
			stats, err := idx.IndexPath(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error indexing %s: %v\n", p, err)
				continue
			}
			total.TotalFiles += stats.TotalFiles
			total.IndexedFiles += stats.IndexedFiles
			total.SkippedFiles += stats.SkippedFiles
			total.TotalTags += stats.TotalTags
			total.Errors = append(total.Errors, stats.Errors...)
		}

		fmt.Printf("Indexing complete:\n")
		fmt.Printf("  Total files:  %d\n", total.TotalFiles)
		fmt.Printf("  Indexed:      %d\n", total.IndexedFiles)
		fmt.Printf("  Skipped:      %d\n", total.SkippedFiles)
		fmt.Printf("  Tags found:   %d\n", total.TotalTags)
		if len(total.Errors) > 0 {
			fmt.Printf("  Errors:       %d\n", len(total.Errors))
			for _, e := range total.Errors {
				fmt.Printf("    - %s: %s\n", e.File, e.Error)
			}
		}
		return nil
	},
}

var queryCommand = &cli.Command{
	Name:  "query",
	Usage: "Query the tag index",
	Description: `Searches the tag index for annotations matching the given criteria.
   Use --stat (singular) for aggregated statistics, --stats for index metadata.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "Output format: json, markdown, or ctags"},
		&cli.StringFlag{Name: "kind", Usage: "Filter by @tag-block kind (e.g., api-endpoint)"},
		&cli.StringFlag{Name: "key", Usage: "Filter by metadata key"},
		&cli.StringFlag{Name: "value", Usage: "Filter by metadata value"},
		&cli.StringFlag{Name: "tag-type", Usage: "Filter by tag type (block or inline)"},
		&cli.StringFlag{Name: "language", Usage: "Filter by source language (javascript, python, rust)"},
		&cli.StringFlag{Name: "entity-kind", Usage: "Filter by entity kind (function_definition, etc.)"},
		&cli.StringFlag{Name: "file", Usage: "Filter by file path pattern (glob supported)"},
		&cli.BoolFlag{Name: "stats", Usage: "Show index metadata instead of tag details"},
		&cli.BoolFlag{Name: "stat", Usage: "Show aggregated tag statistics (counts by key, kind, language)"},
		&cli.StringFlag{Name: "depends-on", Usage: "Find tags that depend on a specific key or value"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		format := output.Format(c.String("format"))
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open index: %w", err)
		}
		defer store.Close()

		if c.Bool("stats") {
			meta, err := store.GetStats()
			if err != nil {
				return fmt.Errorf("failed to get stats: %w", err)
			}
			fmt.Print(output.NewFormatter(format).FormatStats(meta))
			return nil
		}

		// --stat: aggregated tag statistics
		if c.Bool("stat") {
			ts, err := store.GetTagStats()
			if err != nil {
				return fmt.Errorf("failed to get tag stats: %w", err)
			}
			fmt.Printf("Tag Statistics:\n")
			fmt.Printf("  Total tags:  %d\n\n", ts.TotalTags)
			if len(ts.ByLanguage) > 0 {
				fmt.Printf("  By language:\n")
				for k, v := range ts.ByLanguage {
					fmt.Printf("    %-15s %d\n", k, v)
				}
				fmt.Println()
			}
			if len(ts.ByKind) > 0 {
				fmt.Printf("  By kind:\n")
				for k, v := range ts.ByKind {
					fmt.Printf("    %-25s %d\n", k, v)
				}
				fmt.Println()
			}
			if len(ts.ByKey) > 0 {
				fmt.Printf("  By key (top 20):\n")
				for k, v := range ts.ByKey {
					fmt.Printf("    %-30s %d\n", k, v)
				}
				fmt.Println()
			}
			return nil
		}

		q := models.Query{}
		if kind := c.String("kind"); kind != "" {
			q.Kind = &kind
		}
		if key := c.String("key"); key != "" {
			q.Key = &key
		}
		if value := c.String("value"); value != "" {
			q.Value = &value
		}
		if tagType := c.String("tag-type"); tagType != "" {
			t := models.TagType(tagType)
			q.TagType = &t
		}
		if language := c.String("language"); language != "" {
			q.Language = &language
		}
		if entityKind := c.String("entity-kind"); entityKind != "" {
			q.EntityKind = &entityKind
		}
		if file := c.String("file"); file != "" {
			q.FilePattern = &file
		}

		eng := query.New(store)
		tags, err := eng.Query(q)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		// Tag relationship: --depends-on filters tags whose depends-on array
		// contains the specified value, OR whose key/value matches.
		if dep := c.String("depends-on"); dep != "" {
			var related []models.Tag
			for _, t := range tags {
				if val, ok := t.Metadata["depends-on"]; ok {
					valStr := fmt.Sprintf("%v", val)
					if strings.Contains(valStr, dep) {
						related = append(related, t)
						continue
					}
				}
				// Also match if any key/value contains the dep string
				for k, v := range t.Metadata {
					if strings.Contains(k, dep) || strings.Contains(fmt.Sprintf("%v", v), dep) {
						related = append(related, t)
						break
					}
				}
			}
			tags = related
		}

		result, err := output.NewFormatter(format).FormatTags(tags)
		if err != nil {
			return fmt.Errorf("format error: %w", err)
		}
		fmt.Println(result)
		return nil
	},
}

// ── tag command: add, edit, rm ────────────────────────────────────────

var tagCommand = &cli.Command{
	Name:  "tag",
	Usage: "Manually add, edit, or remove tags in the index",
	Subcommands: []*cli.Command{
		tagAddCommand,
		tagEditCommand,
		tagRmCommand,
	},
}

var tagAddCommand = &cli.Command{
	Name:  "add",
	Usage: "Add a tag to the index manually",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Required: true, Usage: "Source file path"},
		&cli.StringFlag{Name: "key", Aliases: []string{"k"}, Required: true, Usage: "Metadata key"},
		&cli.StringFlag{Name: "value", Aliases: []string{"v"}, Required: true, Usage: "Metadata value"},
		&cli.StringFlag{Name: "kind", Usage: "Block kind (e.g., api-endpoint)"},
		&cli.StringFlag{Name: "entity", Usage: "Entity name (e.g., function name)"},
		&cli.StringFlag{Name: "language", Usage: "Source language"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		file := c.String("file")
		key := c.String("key")
		value := c.String("value")
		lang := c.String("language")
		if lang == "" {
			ext := strings.TrimLeft(filepath.Ext(file), ".")
			switch ext {
			case "js", "jsx", "mjs", "cjs":
				lang = "javascript"
			case "py":
				lang = "python"
			case "rs":
				lang = "rust"
			default:
				lang = "unknown"
			}
		}

		entityName := c.String("entity")
		var en *string
		if entityName != "" {
			en = &entityName
		}
		ek := "manual"
		kind := c.String("kind")

		metadata := map[string]interface{}{key: value}
		if kind != "" {
			metadata["kind"] = kind
		}

		tag := models.Tag{
			ID:         fmt.Sprintf("manual_%x", time.Now().UnixNano()),
			File:       file,
			Language:   lang,
			TagType:    models.TagTypeInline,
			EntityName: en,
			EntityKind: &ek,
			Range: models.Range{
				Start: models.Position{Line: 0, Column: 0},
				End:   models.Position{Line: 0, Column: 0},
			},
			Metadata: metadata,
		}
		if kind != "" {
			tag.TagType = models.TagTypeBlock
		}

		if err := store.AddTag(tag); err != nil {
			return fmt.Errorf("failed to add tag: %w", err)
		}
		fmt.Printf("Added tag %s: %s=%s\n", tag.ID, key, value)
		return nil
	},
}

var tagEditCommand = &cli.Command{
	Name:  "edit",
	Usage: "Edit a tag's metadata value by ID",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true, Usage: "Tag ID to edit"},
		&cli.StringFlag{Name: "key", Usage: "New or changed metadata key"},
		&cli.StringFlag{Name: "value", Usage: "New metadata value"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		tagID := c.String("id")
		// Get existing tag
		tags, err := store.QueryTags(models.Query{})
		if err != nil {
			return err
		}
		var found *models.Tag
		for i, t := range tags {
			if t.ID == tagID {
				found = &tags[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("tag %s not found", tagID)
		}

		// Delete and re-add with updated metadata
		if err := store.DeleteTag(tagID); err != nil {
			return err
		}
		if key := c.String("key"); key != "" {
			if val := c.String("value"); val != "" {
				found.Metadata[key] = val
			}
		}
		found.ID = fmt.Sprintf("manual_%x", time.Now().UnixNano())
		if err := store.AddTag(*found); err != nil {
			return err
		}
		fmt.Printf("Edited tag %s -> %s\n", tagID, found.ID)
		return nil
	},
}

var tagRmCommand = &cli.Command{
	Name:  "rm",
	Usage: "Remove a tag from the index by ID",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true, Usage: "Tag ID to remove"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		if err := store.DeleteTag(c.String("id")); err != nil {
			return fmt.Errorf("failed to remove tag: %w", err)
		}
		fmt.Printf("Removed tag %s\n", c.String("id"))
		return nil
	},
}

var lintCommand = &cli.Command{
	Name:  "lint",
	Usage: "Lint @tag annotations and enforce architectural constraints",
	ArgsUsage: "[path]",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "fix", Usage: "Auto-fix detected issues (where possible)"},
		&cli.BoolFlag{Name: "strict", Usage: "Enforce architectural constraints from .marionetterc.json"},
	},
	Action: func(c *cli.Context) error {
		path := "."
		if c.Args().First() != "" {
			path = c.Args().First()
		}
		cfg, err := config.LoadConfig(path)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		linter := lint.New(cfg)
		exitCode := 0

		// Standard lint
		var allResults []output.LintResult
		filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() { return nil }
			ext := filepath.Ext(filePath)
			supported := map[string]bool{".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".py": true, ".rs": true}
			if !supported[ext] { return nil }
			results, err := linter.LintFile(filePath)
			if err != nil { return err }
			allResults = append(allResults, results...)
			return nil
		})

		if len(allResults) > 0 {
			fmt.Print(output.FormatLintResults(allResults))
			fmt.Printf("\n%d lint issue(s) found.\n", len(allResults))
			if hasErrors(allResults) { exitCode = 1 }
		}

		// Strict mode: architectural constraints
		if c.Bool("strict") {
			engine := constraint.New(cfg)
			if len(engine.Rules()) > 0 {
				fmt.Println("\nRunning architectural constraints...")

				// Collect all files for code-level rules
				var files []string
				filepath.WalkDir(path, func(fp string, d os.DirEntry, err error) error {
					if err != nil || d.IsDir() { return nil }
					files = append(files, fp)
					return nil
				})

				// Build tag metadata maps for tag-level rules
				dbPath := c.String("db")
				var tagMaps []map[string]interface{}
				if store, err := storage.NewStore(dbPath); err == nil {
					if tags, err := store.QueryTags(models.Query{}); err == nil {
						for _, t := range tags {
							m := make(map[string]interface{})
							for k, v := range t.Metadata {
								m[k] = v
							}
							m["_file"] = t.File
							m["_line"] = int(t.Range.Start.Line)
							m["_kind"] = t.EntityName
							if t.EntityKind != nil { m["_entity_kind"] = *t.EntityKind }
							tagMaps = append(tagMaps, m)
						}
					}
					store.Close()
				}

				violations := engine.Evaluate(tagMaps, files)
				if len(violations) > 0 {
					fmt.Print(constraint.FormatViolations(violations))
					fmt.Printf("\n%d constraint violation(s) found.\n", len(violations))
					for _, v := range violations {
						if v.Severity == constraint.Error { exitCode = 1 }
					}
				} else {
					fmt.Println("  All constraints passed ✓")
				}
			}
		}

		if exitCode != 0 { os.Exit(exitCode) }
		if len(allResults) == 0 { fmt.Println("No issues found.") }
		return nil
	},
}



func hasErrors(results []output.LintResult) bool {
	for _, r := range results {
		if r.Severity == "error" {
			return true
		}
	}
	return false
}

var lspCommand = &cli.Command{
	Name:  "lsp",
	Usage: "Start the LSP server for IDE integration",
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open index: %w", err)
		}
		defer store.Close()
		eng := query.New(store)
		server := lsp.NewServer(eng)
		log.Println("Marionette LSP server starting...")
		return server.Run()
	},
}

// ── watch command ─────────────────────────────────────────────────────

var watchCommand = &cli.Command{
	Name:  "watch",
	Usage: "Watch files and auto-reindex on changes",
	Description: `Watches the specified directory for file changes and automatically
   re-indexes modified files. Requires the fsnotify package.`,
	ArgsUsage: "[path]",
	Flags: []cli.Flag{
		&cli.DurationFlag{Name: "interval", Aliases: []string{"i"}, Value: 500 * time.Millisecond, Usage: "Debounce interval"},
	},
	Action: func(c *cli.Context) error {
		path := "."
		if c.Args().First() != "" {
			path = c.Args().First()
		}
		dbPath := c.String("db")

		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open index: %w", err)
		}
		defer store.Close()

		treeParser, err := parser.New()
		if err != nil {
			return fmt.Errorf("failed to create parser: %w", err)
		}
		defer treeParser.Close()

		ext := extractor.New(treeParser)
		idx := indexer.New(ext, store)

		fmt.Printf("Watching %s for changes (Ctrl+C to stop)...\n", path)
		fmt.Println("Note: watch mode polls for changes every 2 seconds.")
		fmt.Println("For production use, install fsnotify-backed watcher.")

		// Simple polling-based watch
		lastMod := time.Now()
		for {
			time.Sleep(2 * time.Second)
			err := filepath.WalkDir(path, func(fp string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ext := filepath.Ext(fp)
				supported := map[string]bool{".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".py": true, ".rs": true}
				if !supported[ext] {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if info.ModTime().After(lastMod) {
					stats, err := idx.IndexPath(fp)
					if err == nil && stats.TotalTags > 0 {
						fmt.Printf("  Re-indexed %s (%d tags)\n", fp, stats.TotalTags)
					}
				}
				return nil
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Watch error: %v\n", err)
			}
			lastMod = time.Now()
		}
	},
}

var statsCommand = &cli.Command{
	Name:  "stats",
	Usage: "Display index statistics",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "Output format: json, markdown, or ctags"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		format := output.Format(c.String("format"))
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open index: %w", err)
		}
		defer store.Close()
		meta, err := store.GetStats()
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}
		fmt.Print(output.NewFormatter(format).FormatStats(meta))
		return nil
	},
}

// ── diff command ──────────────────────────────────────────────────────

var diffCommand = &cli.Command{
	Name:  "diff",
	Usage: "Compare tags between two index databases",
	Description: `Compares tags between two marionette index databases (e.g. main branch vs current branch).
   Shows tags that were added, removed, or changed.`,
	ArgsUsage: "db1 db2",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "key", Usage: "Only diff tags with this metadata key"},
		&cli.StringFlag{Name: "kind", Usage: "Only diff tags with this @tag-block kind"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 2 {
			return fmt.Errorf("requires two database paths: marionette diff db1 db2")
		}
		results, err := storage.Diff(c.Args().Get(0), c.Args().Get(1), c.String("key"), c.String("kind"))
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("No differences found.")
			return nil
		}
		fmt.Printf("Diff results (%d changes):\n", len(results))
		for _, r := range results {
			switch r.Status {
			case "added":
				fmt.Printf("  + ADDED    %s\n", r.NewValue)
			case "removed":
				fmt.Printf("  - REMOVED  %s\n", r.OldValue)
			case "changed":
				fmt.Printf("  ~ CHANGED  %s -> %s\n", r.OldValue, r.NewValue)
			}
		}
		return nil
	},
}

// ── migrate command ───────────────────────────────────────────────────

var migrateCommand = &cli.Command{
	Name:  "migrate",
	Usage: "Migrate tag keys across the index",
	Description: `Migrates tag metadata keys across all indexed tags.
   Supports renaming keys, deleting keys, and adding default values.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "rename", Usage: "Rename a key: old=new (e.g. area/billing/tax=tax/v2)"},
		&cli.StringFlag{Name: "delete-key", Usage: "Delete a key from all tags"},
		&cli.StringFlag{Name: "add-default", Usage: "Add a default key=value to tags matching --kind"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		tags, err := store.QueryTags(models.Query{})
		if err != nil {
			return err
		}

		changes := 0

		// --rename
		if rename := c.String("rename"); rename != "" {
			parts := strings.SplitN(rename, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("--rename requires old=new format")
			}
			oldK, newK := parts[0], parts[1]
			for _, t := range tags {
				if val, ok := t.Metadata[oldK]; ok {
					store.DeleteTag(t.ID)
					t.ID = fmt.Sprintf("migrated_%x", time.Now().UnixNano())
					delete(t.Metadata, oldK)
					t.Metadata[newK] = val
					store.AddTag(t)
					changes++
				}
			}
			fmt.Printf("Renamed %d tags: %s -> %s\n", changes, oldK, newK)
		}

		// --delete-key
		if delKey := c.String("delete-key"); delKey != "" {
			for _, t := range tags {
				if _, ok := t.Metadata[delKey]; ok {
					store.DeleteTag(t.ID)
					delete(t.Metadata, delKey)
					t.ID = fmt.Sprintf("migrated_%x", time.Now().UnixNano())
					store.AddTag(t)
					changes++
				}
			}
			fmt.Printf("Deleted key %q from %d tags\n", delKey, changes)
		}

		return nil
	},
}

// ── depends command ───────────────────────────────────────────────────

var dependsCommand = &cli.Command{
	Name:  "depends",
	Usage: "Find tag dependencies and relationships",
	Description: `Queries the dependency graph between tags.
   --on finds tags whose metadata references a key or value.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "on", Usage: "Find tags that depend on or reference this value"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		target := c.String("on")
		if target == "" {
			return fmt.Errorf("requires --on <value>")
		}

		tags, err := store.QueryTags(models.Query{})
		if err != nil {
			return err
		}

		var related []models.Tag
		for _, t := range tags {
			for k, v := range t.Metadata {
				valStr := fmt.Sprintf("%v", v)
				if strings.Contains(k, target) || strings.Contains(valStr, target) {
					related = append(related, t)
					break
				}
			}
		}

		if len(related) == 0 {
			fmt.Printf("No tags reference %q\n", target)
			return nil
		}

		fmt.Printf("Tags referencing %q (%d):\n", target, len(related))
		for _, t := range related {
			fmt.Printf("  %s (%s) - %s\n", t.File, t.Language, t.ID)
		}
		return nil
	},
}

// ── serve command (MCP) ───────────────────────────────────────────────

var serveCommand = &cli.Command{
	Name:  "serve",
	Usage: "Start the MCP server for AI agent integration",
	Description: `Starts a Model Context Protocol server that exposes @tag annotations
   as MCP tools. AI agents can query tags directly without running CLI commands.
   Communicates via stdin/stdout using JSON-RPC.`,
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open index: %w", err)
		}
		defer store.Close()

		eng := query.New(store)
		mcpServer := lsp.NewMCPServer(eng, store)

		log.Println("Marionette MCP server starting on stdin/stdout...")
		log.Println("Available tools: tagdoc_search, tagdoc_pii_scan, tagdoc_architecture, tagdoc_compliance, tagdoc_feature_flags, tagdoc_depends")
		return mcpServer.Run()
	},
}

// ── callers command ───────────────────────────────────────────────────

var callersCommand = &cli.Command{
	Name:  "callers",
	Usage: "Show which functions call a given symbol",
	ArgsUsage: "<symbol>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "show-tags", Usage: "Show @tag metadata on caller functions"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("requires a symbol name: marionette callers <symbol>")
		}
		symbol := c.Args().First()
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil { return err }
		defer store.Close()

		showTags := c.Bool("show-tags")

		// Primary: use call_edges table
		callers, err := store.QueryCallers(symbol)
		count := 0
		if err == nil && len(callers) > 0 {
			fmt.Printf("Callers of %q (call graph):\n", symbol)
			for _, c := range callers {
				count++
				fmt.Printf("  %s — %s\n", c.CallerSymbol, c.File)
			}
		} else {
			// Fallback: search metadata for symbol references
			tags, qErr := store.QueryTags(models.Query{})
			if qErr != nil { return qErr }

			fmt.Printf("Callers of %q (metadata references):\n", symbol)
			for _, t := range tags {
				if t.EntityName != nil && *t.EntityName == symbol {
					continue
				}
				for k, v := range t.Metadata {
					valStr := fmt.Sprintf("%v", v)
					if strings.Contains(valStr, symbol) || strings.Contains(k, symbol) {
						count++
						if showTags {
							metaStr := formatBriefMeta(t.Metadata)
							fmt.Printf("  %s (%s:%d) — %s\n", t.File, t.Language, t.Range.Start.Line, metaStr)
						} else {
							fmt.Printf("  %s (%s:%d) — %s\n", t.File, t.Language, t.Range.Start.Line, t.ID)
						}
						break
					}
				}
			}
		}
		if count == 0 {
			fmt.Printf("  No callers found for %q\n", symbol)
		}
		return nil
	},
}

// ── callees command ───────────────────────────────────────────────────

var calleesCommand = &cli.Command{
	Name:  "callees",
	Usage: "Show which functions a symbol calls",
	ArgsUsage: "<symbol>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "show-tags", Usage: "Show @tag metadata on callee symbols"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("requires a symbol name: marionette callees <symbol>")
		}
		symbol := c.Args().First()
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil { return err }
		defer store.Close()

		showTags := c.Bool("show-tags")

		// Primary: use call_edges table
		callees, err := store.QueryCallees(symbol)
		count := 0
		if err == nil && len(callees) > 0 {
			fmt.Printf("Callees of %q (call graph):\n", symbol)
			for _, c := range callees {
				count++
				if showTags {
					// Look up @tag metadata on callee symbol
					tags, _ := store.QueryTags(models.Query{})
					metaStr := ""
					for _, t := range tags {
						if t.EntityName != nil && *t.EntityName == c.CalleeSymbol {
							metaStr = formatBriefMeta(t.Metadata)
							break
						}
					}
					if metaStr != "" {
						fmt.Printf("  %s — %s (%s)\n", c.CalleeSymbol, c.File, metaStr)
					} else {
						fmt.Printf("  %s — %s\n", c.CalleeSymbol, c.File)
					}
				} else {
					fmt.Printf("  %s — %s (line %d)\n", c.CalleeSymbol, c.File, c.CallLine)
				}
			}
		} else {
			// Fallback: depends-on metadata
			tags, qErr := store.QueryTags(models.Query{EntityKind: &[]string{"function_definition"}[0]})
			if qErr != nil { return qErr }

			fmt.Printf("Callees of %q (via depends-on metadata):\n", symbol)
			for _, t := range tags {
				if t.EntityName == nil || *t.EntityName != symbol {
					continue
				}
				if dep, ok := t.Metadata["depends-on"]; ok {
					count++
					line := fmt.Sprintf("%v", dep)
					if showTags {
						fmt.Printf("  depends-on: %s\n", line)
					} else {
						fmt.Printf("  %s\n", line)
					}
				}
			}
		}
		if count == 0 {
			fmt.Printf("  No callees found for %q\n", symbol)
		}
		return nil
	},
}

// ── impact command ────────────────────────────────────────────────────

var impactCommand = &cli.Command{
	Name:  "impact",
	Usage: "Show impact analysis for a symbol",
	ArgsUsage: "<symbol>",
	Description: `Shows the blast radius of changing a symbol: all tags referencing it,
   all files containing it, and its @tag metadata for compliance context.`,
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("requires a symbol name: marionette impact <symbol>")
		}
		symbol := c.Args().First()
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil { return err }
		defer store.Close()

		tags, err := store.QueryTags(models.Query{})
		if err != nil { return err }

		fmt.Printf("Impact analysis for %q:\n\n", symbol)

		// Tags on this symbol
		fmt.Println("Tags on this symbol:")
		symTags := 0
		for _, t := range tags {
			if t.EntityName != nil && *t.EntityName == symbol {
				symTags++
				fmt.Printf("  %s — %s\n", t.File, formatBriefMeta(t.Metadata))
			}
		}
		if symTags == 0 {
			fmt.Println("  (no @tag annotations on this symbol)")
		}

		// Tags referencing this symbol
		fmt.Println("\nTags referencing this symbol:")
		refTags := 0
		for _, t := range tags {
			for _, v := range t.Metadata {
				if strings.Contains(fmt.Sprintf("%v", v), symbol) {
					refTags++
					name := "-"
					if t.EntityName != nil { name = *t.EntityName }
					fmt.Printf("  %s.%s — %s\n", t.File, name, formatBriefMeta(t.Metadata))
					break
				}
			}
		}
		if refTags == 0 {
			fmt.Println("  (no references found)")
		}

		fmt.Printf("\nSummary: %d direct tags, %d references\n", symTags, refTags)
		return nil
	},
}

// ── health command ────────────────────────────────────────────────────

var healthCommand = &cli.Command{
	Name:  "health",
	Usage: "Marionette health dashboard — coverage, compliance, gaps",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show detailed breakdown"},
		&cli.StringFlag{Name: "threshold", Usage: "Minimum grade for CI gating (A/B/C/D). Exits 1 if grade is below threshold."},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil { return err }
		defer store.Close()

		stats, err := store.GetTagStats()
		if err != nil { return err }
		meta, err := store.GetStats()
		if err != nil { return err }

		// Compute health score
		score := 100.0
		var warnings []string

		// Deductions
		if meta.TotalFiles == 0 {
			score -= 50
			warnings = append(warnings, "No files indexed")
		}
		if meta.TotalTags == 0 {
			score -= 30
			warnings = append(warnings, "No tags found")
		}
		// Check for untagged functions
		tagsOnFuncs := stats.ByEntityKind["function_definition"] + stats.ByEntityKind["function_declaration"]
		if tagsOnFuncs == 0 && meta.TotalTags > 0 {
			score -= 15
			warnings = append(warnings, "No functions have @tag annotations")
		}
		if score < 0 { score = 0 }

		// Grade
		grade := "F"
		switch {
		case score >= 90: grade = "A"
		case score >= 80: grade = "B"
		case score >= 65: grade = "C"
		case score >= 50: grade = "D"
		}

		fmt.Printf("Marionette Health Report\n")
		fmt.Printf("  Overall: %s (%.0f/100)\n\n", grade, score)

		fmt.Printf("  Index:\n")
		fmt.Printf("    Files:        %d\n", meta.TotalFiles)
		fmt.Printf("    Tags:         %d\n", meta.TotalTags)
		fmt.Printf("    Languages:    %d\n", len(stats.ByLanguage))

		fmt.Printf("\n  Coverage by language:\n")
		for lang, count := range stats.ByLanguage {
			fmt.Printf("    %-15s %d tags\n", lang, count)
		}

		if len(stats.ByKind) > 0 {
			fmt.Printf("\n  By kind:\n")
			for kind, count := range stats.ByKind {
				fmt.Printf("    %-25s %d\n", kind, count)
			}
		}

		if len(warnings) > 0 {
			fmt.Printf("\n  Warnings:\n")
			for _, w := range warnings {
				fmt.Printf("    ⚠ %s\n", w)
			}
		}

		if c.Bool("verbose") {
			fmt.Printf("\n  Top tag keys:\n")
			for key, count := range stats.ByKey {
				fmt.Printf("    %-30s %d\n", key, count)
			}
		}

		// --threshold for CI gating
		if t := c.String("threshold"); t != "" {
			threshold := strings.ToUpper(t)
			gradeRank := map[string]int{"A": 0, "B": 1, "C": 2, "D": 3, "F": 4}
			reqRank, ok := gradeRank[threshold]
			if !ok {
				fmt.Fprintf(os.Stderr, "Invalid threshold %q. Use A, B, C, or D.\n", t)
				os.Exit(1)
			}
			actualRank := gradeRank[grade]
			if actualRank > reqRank {
				fmt.Fprintf(os.Stderr, "Health grade %s is below threshold %s — failing CI gate.\n", grade, threshold)
				os.Exit(1)
			}
			fmt.Printf("\nHealth grade %s meets threshold %s ✓\n", grade, threshold)
		}
		return nil
	},
}

// ── search command (FTS5) ─────────────────────────────────────────────

var searchCommand = &cli.Command{
	Name:  "search",
	Usage: "Full-text search over tag metadata",
	ArgsUsage: "<query>",
	Description: `Searches tag metadata using FTS5 full-text search.
   Supports ranked results with BM25 scoring.
   Requires --db to point to an indexed database.`,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 20, Usage: "Max results"},
		&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "Output format"},
		&cli.StringFlag{Name: "file", Usage: "Filter results by file path pattern (glob supported)"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("requires a search query: marionette search <query>")
		}
		query := c.Args().First()
		dbPath := c.String("db")
		format := output.Format(c.String("format"))

		store, err := storage.NewStore(dbPath)
		if err != nil { return err }
		defer store.Close()

		fileFilter := c.String("file")

		// Use FTS5 search on tag_properties
		results, err := store.SearchTags(query, c.Int("limit"))
		if err != nil {
			// Fallback: simple substring search
			tags, err := store.QueryTags(models.Query{})
			if err != nil { return err }

			var matched []models.Tag
			q := strings.ToLower(query)
			for _, t := range tags {
				// Apply file filter
				if fileFilter != "" {
					matched, err := filepath.Match(fileFilter, filepath.Base(t.File))
					if err != nil || (!matched && !strings.Contains(t.File, fileFilter)) {
						continue
					}
				}
				for k, v := range t.Metadata {
					if strings.Contains(strings.ToLower(k), q) || strings.Contains(strings.ToLower(fmt.Sprintf("%v", v)), q) {
						matched = append(matched, t)
						break
					}
				}
			}
			// Honor limit
			if len(matched) > c.Int("limit") {
				matched = matched[:c.Int("limit")]
			}
			if format == "markdown" || format == "" {
				fmt.Printf("Search results for %q (%d):\n", query, len(matched))
				for _, t := range matched {
					fmt.Printf("  %s (%s) — %s\n", t.File, t.Language, formatBriefMeta(t.Metadata))
				}
			} else {
				resultStr, _ := output.NewFormatter(format).FormatTags(matched)
				fmt.Println(resultStr)
			}
			return nil
		}

		// Apply file filter to FTS5 results
		if fileFilter != "" {
			var filtered []storage.SearchResult
			for _, r := range results {
				matched, err := filepath.Match(fileFilter, filepath.Base(r.File))
				if err == nil && (matched || strings.Contains(r.File, fileFilter)) {
					filtered = append(filtered, r)
				}
			}
			results = filtered
		}

		if format == "markdown" || format == "" {
			fmt.Printf("Search results for %q (%d):\n", query, len(results))
			for _, r := range results {
				fmt.Printf("  [%.2f] %s — %s=%s\n", r.Score, r.File, r.Key, r.Value)
			}
		} else {
			b, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(b))
		}
		return nil
	},
}

// ── analytics command ─────────────────────────────────────────────────

var analyticsCommand = &cli.Command{
	Name:  "analytics",
	Usage: "Tag analytics over time",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "since", Usage: "Show data since date (YYYY-MM-DD)"},
		&cli.BoolFlag{Name: "snapshot", Usage: "Record a new analytics snapshot"},
	},
	Action: func(c *cli.Context) error {
		dbPath := c.String("db")
		store, err := storage.NewStore(dbPath)
		if err != nil { return err }
		defer store.Close()

		if c.Bool("snapshot") {
			meta, _ := store.GetStats()
			tagStats, _ := store.GetTagStats()
			byLang, _ := json.Marshal(tagStats.ByLanguage)
			byKind, _ := json.Marshal(tagStats.ByKind)
			byKey, _ := json.Marshal(tagStats.ByKey)
			store.RecordSnapshot(meta.TotalTags, meta.TotalFiles, string(byLang), string(byKind), string(byKey))
			fmt.Printf("Snapshot recorded: %d tags, %d files\n", meta.TotalTags, meta.TotalFiles)
			return nil
		}

		snapshots, err := store.GetSnapshots(30)
		if err != nil {
			return fmt.Errorf("no snapshots yet: use --snapshot to record one")
		}

		fmt.Println("Tag Analytics (last 30 snapshots):")
		fmt.Printf("  %-25s %8s %8s\n", "Timestamp", "Tags", "Files")
		fmt.Printf("  %-25s %8s %8s\n", "-------------------------", "--------", "--------")
		for _, s := range snapshots {
			fmt.Printf("  %-25s %8d %8d\n", s.Timestamp, s.TotalTags, s.TotalFiles)
		}

		if len(snapshots) >= 2 {
			first, last := snapshots[len(snapshots)-1], snapshots[0]
			tagDelta := last.TotalTags - first.TotalTags
			fileDelta := last.TotalFiles - first.TotalFiles
			fmt.Printf("\n  Trend: %+d tags, %+d files\n", tagDelta, fileDelta)
		}
		return nil
	},
}

// formatBriefMeta formats metadata as a short string for display.
func formatBriefMeta(m map[string]interface{}) string {
	parts := make([]string, 0, 3)
	// Show kind first
	if k, ok := m["kind"]; ok {
		parts = append(parts, fmt.Sprintf("kind=%v", k))
	}
	// Show route or security
	if r, ok := m["route"]; ok {
		parts = append(parts, fmt.Sprintf("route=%v", r))
	} else if s, ok := m["security"]; ok {
		parts = append(parts, fmt.Sprintf("security=%v", s))
	}
	return strings.Join(parts, ", ")
}

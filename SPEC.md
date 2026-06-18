# Marionette — Software Requirements Specification (SPEC.md)

Status: current as of 2026-10-15 (post-rebrand).
This document is the authoritative specification (SRS) for the project. It is
written to be consumed by both humans and AI agents working on the codebase.

---

## 1. Overview

**Marionette** is a structured code-annotation indexing and query system. It
extracts `@tag` annotations from source-code comments, builds an embedded
SQLite index, and exposes that index through a CLI, a Language Server Protocol
(LSP) server, and a Model Context Protocol (MCP) server.

- **Project / repository name:** `marionette`
- **Command-line tool / binary name:** `marionette`
- **Go module path:** `github.com/xyzzyapps/marionette`
- **License:** PolyForm Noncommercial License 1.0.0 (noncommercial use only)

### 1.1 Naming contract (IMPORTANT)

The project and the shipped executable share a single name: **Marionette**
(in prose, `Marionette`; the CLI is written `marionette`, lowercase/monospace).
The repository, Go module path, config file, and default database all use the
same name. There is no separate "project vs tool" naming split.

The following identifiers are **intentionally retained from the historical name
`tagdoc`** and MUST NOT be renamed without an explicit product decision (they are
public API surface / stable identifiers):

| Identifier | Kind | Value (unchanged) |
|------------|------|-------------------|
| `tagdoc_search` | MCP tool | search tags by kind/key/value |
| `tagdoc_pii_scan` | MCP tool | list PII-tagged code |
| `tagdoc_architecture` | MCP tool | architecture entry points |
| `tagdoc_compliance` | MCP tool | security/GDPR report |
| `tagdoc_feature_flags` | MCP tool | feature-flag status |
| `tagdoc_depends` | MCP tool | dependency relationships |
| `tagdoc-lsp` | LSP server `name` | advertised by `initialize` result |
| `tagdoc-mcp` | MCP server `name` | advertised by `initialize` result |
| `tagdoc-core` | `@tag domain:` value | source-annotation domain for all packages |

Also intentionally **not** renamed: the historical session archive under
`.todo/` and the comment headers in `queries/*.scm`.

### 1.2 Renamed identifiers (for reference)

| Before | After |
|--------|-------|
| Module `github.com/manic/tagdoc` | `github.com/xyzzyapps/marionette` |
| Binary `tagdoc(.exe)` | `marionette(.exe)` |
| CLI command `tagdoc <cmd>` | `marionette <cmd>` |
| `src/cmd/tagdoc/` | `src/cmd/marionette/` |
| Config file `.tagdocrc.json` | `.marionetterc.json` |
| Default DB `.tagdoc.db` | `.marionette.db` |
| Go const `tagDocVersion` | `marionetteVersion` |

---

## 2. Goals and Non-Goals

### 2.1 Goals
- Extract structured, typed metadata from `@tag` comments in 30+ languages.
- Provide an incremental, embedded (no external DB) index.
- Support querying, diffing, migration, linting, and health reporting.
- Integrate with IDEs (LSP) and AI coding agents (MCP).
- Enforce architectural/compliance constraints in CI.

### 2.2 Non-Goals
- Executing or type-checking the analyzed code.
- Being a full language server (no completion, no diagnostics beyond tags).
- Network services: all servers speak over stdio (JSON-RPC).

---

## 3. Glossary

| Term | Meaning |
|------|---------|
| Block tag | `@tag-block <kind>` annotation attached to a code entity (function/class/method). |
| Inline tag | `@tag-inline key: value` annotation attached to the next statement/expression. |
| Kind | The `<kind>` of a block tag (e.g., `api-endpoint`, `cron-job`); stored as metadata `kind`. |
| Property | A `key: value` pair parsed from a tag comment. |
| Hierarchical key | A property key containing `/` path separators (e.g., `area/billing/tax`). |
| Template | A named set of properties in config, expanded by `@tag-use <name>`. |
| Snapshot | A recorded analytics data point (tag/file counts over time). |

---

## 4. System Architecture

### 4.1 Component diagram

```
                         Source files (*.js, *.py, *.rs, *.go, ...)
                                       │
                                       ▼
                         ┌───────────────────────────┐
                         │  parser  (tree-sitter)    │  31-language registry
                         │  comment + entity mining  │  helpers: sibling walk
                         └─────────────┬─────────────┘
                                       │ raw comment blocks + entity spans
                                       ▼
                         ┌───────────────────────────┐
                         │  extractor (EBNF)         │  @tag tokenizer
                         │  types, hierarchy, tmpl   │  template expansion
                         └─────────────┬─────────────┘
                                       │ []models.Tag
                                       ▼
                         ┌───────────────────────────┐
                         │  indexer                  │  MD5 hash caching,
                         │  incremental orchestration│  call-graph extraction
                         └─────────────┬─────────────┘
                                       │
                                       ▼
                         ┌───────────────────────────┐
                         │  storage (SQLite)         │  tags, files,
                         │  schema v2.0.0            │  tag_properties, symbols,
                         │                           │  call_edges, snapshots
                         └───────┬───────────┬───────┘
                                 │           │
             ┌───────────────────┘           └────────────────────┐
             ▼                                                     ▼
   ┌──────────────────┐    ┌──────────────┐    ┌──────────────┐  ┌──────────────┐
   │  query engine    │───▶│  output      │    │  lsp (LSP)   │  │  lint /      │
   │  filters + stats │    │  json/md/ctags│   │  mcp (MCP)   │  │  constraint  │
   └────────┬─────────┘    └──────────────┘    └──────────────┘  └──────────────┘
            │
            ▼
   ┌──────────────────────────────────────────────┐
   │  cmd/marionette  — CLI entry (18 commands)    │
   └──────────────────────────────────────────────┘
```

### 4.2 Package responsibilities

| Package | Path | Responsibility |
|---------|------|----------------|
| `main` | `src/cmd/marionette` | CLI wiring, command definitions, orchestration, exit codes. |
| `models` | `src/pkg/models` | Core data types: `Tag`, `FileEntry`, `IndexMetadata`, `Query`, `Range`, `Position`. |
| `parser` | `src/pkg/parser` | Tree-sitter language registry; comment-block extraction; entity resolution via sibling walk. |
| `extractor` | `src/pkg/extractor` | `@tag` tokenizer (EBNF grammar), typed value parsing, template resolution. |
| `indexer` | `src/pkg/indexer` | Walks paths, MD5 cache checks, drives parser→extractor→storage, builds call graph. |
| `storage` | `src/pkg/storage` | SQLite persistence, schema DDL, diff between databases. |
| `query` | `src/pkg/query` | Structured filtering, aggregated statistics, dependency lookups. |
| `output` | `src/pkg/output` | Formatters: JSON, Markdown table, legacy ctags; lint result rendering. |
| `config` | `src/pkg/config` | Loads/saves `.marionetterc.json` (rules, templates, constraints, repositories). |
| `lint` | `src/pkg/lint` | Orphan-tag detection, type-safety validation against config rules. |
| `constraint` | `src/pkg/constraint` | Evaluates `tag` / `code` / `arch` constraint rules over the AST + index. |
| `callgraph` | `src/pkg/callgraph` | Symbol and caller/callee extraction feeding `symbols` and `call_edges`. |
| `lsp` | `src/pkg/lsp` | LSP server (`lsp.go`) and MCP server (`mcp.go`). |

---

## 5. Data Model

### 5.1 Go types (package `models`)

```
Tag
 ├─ ID           string            // "tag_<hex>"
 ├─ File         string
 ├─ Language     string
 ├─ TagType      "block" | "inline"
 ├─ EntityName   *string           // nil for inline-on-expression
 ├─ EntityKind   *string
 ├─ Range        {Start{Line,Column}, End{Line,Column}}  // 0-based
 ├─ Metadata     map[string]any    // strings, numbers, booleans, []string
 ├─ CreatedAt    time.Time
 └─ UpdatedAt    time.Time

FileEntry   { Path, Language, Size, MD5Hash, TagsCount, IndexedAt }
IndexMetadata { Version, CreatedAt, UpdatedAt, TotalFiles, TotalTags }
Query       { Kind?, Key?, Value?, TagType?, Language?, FilePattern?, EntityKind? }
IndexOutput { Metadata, []Tag, []FileEntry }
```

### 5.2 SQLite schema (v2.0.0)

```
index_metadata(key PK, value)
files(id PK, path UNIQUE, language, size, md5_hash, tags_count, indexed_at)
tags(id PK, file_id FK→files ON DELETE CASCADE, language, tag_type
     CHECK(IN ('block','inline')), entity_name, entity_kind,
     start_line, start_col, end_line, end_col, created_at, updated_at)
tag_properties(id PK, tag_id FK→tags ON DELETE CASCADE, key, value)
symbols(id PK, file_id FK→files, name, kind, line, language)
call_edges(id PK, caller_file FK→files, caller_symbol,
           callee_file FK→files, callee_symbol, call_line)
analytics_snapshots(id PK, timestamp, total_tags, total_files,
                    by_language, by_kind, by_key)
```

Indexes exist on `files(path|md5_hash|language)`, `tags(file_id|tag_type|entity_kind)`,
`tag_properties(tag_id|key|value|key,value)`, `symbols(name)`,
`call_edges(caller_symbol|callee_symbol)`.

Incremental indexing: a file is skipped when its stored `md5_hash` equals the
computed MD5 of its current contents. On change, the file's tags, properties,
symbols, and call edges are replaced.

---

## 6. `@tag` Annotation Syntax

### 6.1 Block tags
```
@tag-block <kind>
@tag key: value
...
```
The `<kind>` is stored as metadata `kind`. Applies to the next code entity.

### 6.2 Inline tags
```
@tag-inline key: value
```
Attaches to the next statement; no `kind`.

### 6.3 Value types
| Type | Syntax |
|------|--------|
| String | `"quoted"`, `` `backtick` ``, or unquoted fallback |
| Multiline | `"""..."""` or ```` ```...``` ```` |
| Number | `42`, `3.14` |
| Boolean | `true` / `false` |
| Array | `["a", "b"]` |

### 6.4 Hierarchical keys
Keys may contain `/` separators (`area/billing/tax-calculation`). Matching in the
CLI is exact; the Go API offers `FindByKeyHierarchy` for prefix matching.

### 6.5 Templates
`@tag-use <name>` expands the template's properties from
`.marionetterc.json` `templates[]` before tokenization.

### 6.6 Agent directives
`@agent-todo`, `@agent-skill`, `@agent-ignore begin/end` are captured as tags
and are searchable.

---

## 7. Configuration (`.marionetterc.json`)

Resolved by walking up parent directories from the working directory; falls back
to `DefaultConfig()` when not found.

```jsonc
{
  "version": "2.0",
  "index":       { "paths": [], "exclude": ["node_modules/**", ".git/**"], "follow_symlinks": false },
  "repositories":[ { "path": "../backend", "label": "Go API", "language": "go" } ],
  "rules":       [ { "key": "security", "required": true,
                     "allowed_values": ["public","rbac"], "type": "string" } ],
  "constraints": [ { "type": "tag|code|arch", "name": "...", "rule": "...",
                     "severity": "error|warning|info" } ],
  "templates":   [ { "name": "billing-endpoint", "tags": { "type": "backend" } } ],
  "output":      { "format": "json", "color": false }
}
```

Constraint rule grammars (`marionette lint --strict`):
- `tag`  — `require:<key>` or `if-then:<key>=<value>,<required-key>`
- `code` — `no <identifier>` (forbidden AST nodes / calls)
- `arch` — `<module> must not import <module>`

---

## 8. CLI Contract (`marionette`)

Global flags: `--db,-d` (default `.marionette.db`), `--format,-f`
(`json|markdown|ctags`), `--config` (path to `.marionetterc.json`).

| Command | Purpose |
|---------|---------|
| `init` | Create index DB + default `.marionetterc.json`. |
| `index [path...]` | Incrementally index files (MD5 cache); loads `repositories`. |
| `query [flags]` | Filter (`--kind --key --value --tag-type --language --entity-kind --file`), `--stats`, `--stat`, `--depends-on`. |
| `tag {add\|edit\|rm}` | Manual tag mutation. |
| `callers <symbol>` | Callers of a symbol (call graph). |
| `callees <symbol>` | Callees of a symbol. |
| `impact <symbol>` | Blast radius: tags on + references to a symbol. |
| `diff <db1> <db2>` | Compare tags across two index DBs (branch diff). |
| `migrate --rename/--delete-key` | Bulk rename/delete a metadata key. |
| `depends --on <value>` | Tags referencing a value. |
| `search <query> [--limit] [--file]` | Full-text search over tag metadata. |
| `analytics [--snapshot] [--since]` | Tag analytics over time. |
| `health [--verbose] [--threshold X]` | Coverage grade (A–F); CI gate on threshold. |
| `lint [path] [--strict]` | Validate tags; enforce constraints in strict mode. |
| `lsp` | LSP server over stdin/stdout. |
| `serve` | MCP server over stdin/stdout. |
| `watch [path]` | Poll-based auto-reindex (~2s). |
| `stats` | Show index metadata. |

Exit codes: `0` success; non-zero on error. `lint`/`health --threshold` return
non-zero to fail CI.

---

## 9. Server Interfaces

### 9.1 LSP (`marionette lsp`)
JSON-RPC over stdin/stdout. Server `name` = `tagdoc-lsp`.
Supported methods: `initialize`, `initialized`, `textDocument/definition`,
`textDocument/references`.

### 9.2 MCP (`marionette serve`)
JSON-RPC over stdin/stdout. Server `name` = `tagdoc-mcp`.
Tools (stable identifiers, see §1.1): `tagdoc_search`, `tagdoc_pii_scan`,
`tagdoc_architecture`, `tagdoc_compliance`, `tagdoc_feature_flags`,
`tagdoc_depends`.

---

## 10. Data Flow (index)

```
paths ─▶ walk ─▶ extension → language ─▶ tree-sitter parse
      ─▶ comment blocks + entity spans ─▶ @tag tokenizer (typed values, templates)
      ─▶ []Tag ─▶ MD5 compare ─▶ (skip | replace) ─▶ SQLite (tags, tag_properties)
      ─▶ call-graph pass ─▶ symbols, call_edges
```

---

## 11. Testing Strategy

- Unit tests target each package's public interface (models, extractor,
  storage, query, config, lint, constraint) with an in-memory/temp SQLite DB.
- The CLI is the primary end-to-end surface; the `Makefile` `test` target runs
  `init → index testdata → query --format markdown → stats` against
  `testdata/` and asserts a clean exit.
- **Current gap:** no `*_test.go` files exist yet; end-to-end behavior is
  validated via the Makefile target and fixture files under `testdata/` and
  `examples/`.

---

## 12. Build and Tooling

- Toolchain: Go 1.26 (module `github.com/xyzzyapps/marionette`).
- Build: `make build` → `go build -o marionette.exe ./src/cmd/marionette/`.
- Test: `make test`; Lint helper: `make lint`.
- SQLite driver is pure-Go (`modernc.org/sqlite`); no CGO required for it.
- Tree-sitter grammars are vendored through `github.com/smacker/go-tree-sitter`
  (CGO; `C:/msys64/ucrt64/bin` provides `gcc` on Windows).

### 12.1 Known formatting note
Several Go files are **not** `gofmt`-clean at the project's own style baseline
(manual struct-field alignment and single-line `if` bodies). This predates the
rebrand. Reformatting is intentionally out of scope unless requested, to keep
diffs focused.

### 12.2 Repository assets
- `assets/marionette.png` — the README banner: a 1280×640 (2:1) PNG with
  transparent padding, embedded at the top of `README.md` at full width. It was
  derived (via `ffmpeg`) from `Image.avif`, which is intentionally left untracked.

---

## 13. Security, Privacy, and Compliance

The tool itself performs no network I/O and stores nothing but source-derived
metadata in a local SQLite file. Compliance features (PII scanning, GDPR
constraint rules) are advisory and operate purely on annotations and AST.

---

## 14. Licensing

PolyForm Noncommercial License 1.0.0. Noncommercial use only. See `LICENSE`.
Required Notice: Copyright (c) 2026 Xyzzy Apps.

---

## 15. Future Work

Tracked in `TODO.md` (icebox): interactive `browse` TUI, PR-comment diff
format, end-to-end `lint --strict` verification against constraint fixtures,
`tag import-batch`, and an `export` command.

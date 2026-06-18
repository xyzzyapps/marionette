# Marionette — Structured Code Annotation Indexing & Query System

<p align="center">
  <img src="assets/marionette.png" alt="Marionette" width="100%">
</p>

**Marionette** extracts `@tag` annotations from code comments and builds a queryable,
structured index. It is a modern replacement for traditional ctags, designed for
AI-assisted development, cross-language architecture discovery, compliance auditing,
and semantic codebase navigation.

```
Source Code with @tag Comments
         │
         ▼
┌─────────────────────┐
│  Tree-sitter Parser  │  ← 31 languages supported
│  (AST comment mining)│
└─────────┬───────────┘
          │ comment + entity pairs
          ▼
┌─────────────────────┐
│  @tag Tokenizer      │  ← EBNF grammar with typed values
│  (hierarchical keys) │     multiline strings, templates
└─────────┬───────────┘
          │ structured tags
          ▼
┌─────────────────────┐
│  SQLite Index        │  ← Incremental via MD5 hashing
│  (files, tags, meta) │     tag mutations, diffs, stats
└─────────┬───────────┘
          │
          ├──→ marionette query   (CLI — filter, stat, depend)
          ├──→ marionette diff    (branch comparison)
          ├──→ marionette migrate (key rename/delete)
          ├──→ marionette lint    (validation)
          ├──→ marionette lsp     (LSP — IDE integration)
          ├──→ marionette serve   (MCP — AI agent integration)
          └──→ marionette watch   (auto-reindex)
```

## Features

- **31 languages**: JavaScript, TypeScript, Python, Rust, Go, Java, C, C++, C#,
  Ruby, PHP, Swift, Kotlin, Scala, Bash, SQL, HTML, CSS, YAML, TOML, Elixir,
  Elm, Lua, Groovy, HCL, Dockerfile, OCaml, Protobuf, Svelte, CUE, and more.
  Extensible via tree-sitter grammar registry.
- **Structured tags**: Block tags (`@tag-block <kind>`) and inline tags (`@tag-inline`)
- **Typed values**: Strings, numbers, booleans, arrays parsed automatically
- **Hierarchical keys**: `@tag area/billing/tax-calculation: "v2"` — path-based taxonomies
- **Multiline strings**: `@tag description: """long text with newlines"""`
- **Tag templates**: `@tag-use billing-endpoint` expands from `.marionetterc.json` templates
- **Incremental indexing**: MD5 hash caching skips unchanged files
- **SQLite-backed**: Fast, embedded, no external database
- **Multiple output formats**: JSON, Markdown tables, legacy ctags format
- **Tag mutations**: `marionette tag add`, `marionette tag edit`, `marionette tag rm`
- **Branch diff**: `marionette diff main HEAD --key security` — compare index states
- **Tag migration**: `marionette migrate --rename "old/key" "new/key"` — bulk rename/delete
- **Dependency queries**: `marionette depends --on billing` — find related tags
- **Stat queries**: `marionette query --stat` — aggregated counts by key, kind, language
- **LSP server**: Go to Definition, Find References for @tag annotations
- **MCP server**: `marionette serve` — expose tags to AI agents via Model Context Protocol
- **Linter**: Orphan tag detection, type safety validation via `.marionetterc.json`
- **Watch mode**: `marionette watch` — auto-reindex on file changes
- **Multi-repository**: Index multiple directories from one config
- **Configuration**: Project-level `.marionetterc.json` for rules, templates, repositories

## Quick Start

```bash
# Build from source
cd marionette
go build -o marionette.exe ./src/cmd/marionette/

# Initialize a new index
marionette --db demo.db init

# Index all supported files
marionette --db demo.db index .

# Query everything
marionette --db demo.db query --format markdown

# Query with filters
marionette --db demo.db query --kind api-endpoint
marionette --db demo.db query --key security --value rbac

# Aggregated statistics
marionette --db demo.db query --stat

# Show index health
marionette --db demo.db stats
```

## @tag Syntax

### Block Tags — attach metadata to code blocks

```javascript
/**
 * @tag-block api-endpoint
 * @tag route: "/v1/users/:id/promote"
 * @tag method: "POST"
 * @tag security: "rbac"
 * @tag roles: ["admin", "super-admin"]
 */
async function promoteUser(req, res) { ... }
```

```python
# @tag-block cron-job
# @tag schedule: "0 0 * * *"
# @tag data-domain: "analytics"
# @tag side-effects: ["purges-s3", "drops-cache"]
def nightly_purger(): ...
```

### Inline Tags — annotate specific statements

```javascript
// @tag-inline audit: "Mutates critical user role data"
await db.users.update(id, { role: 'admin' });
```

### Hierarchical Keys — taxonomy paths

```python
# @tag area/billing/tax-calculation: "v2"
# @tag area/billing/invoicing: "v1"
# @tag compliance/regions: ["US", "EU"]
# @tag compliance/gdpr: "relevant"
```

### Multiline Strings — triple-quoted values

```python
# @tag description: """
#   Calculates tax for a given invoice line item.
#   Supports VAT, GST, and sales tax regimes.
# """
```

### Tag Templates — reuse common tag sets

```python
# @tag-use billing-endpoint
# @tag route: "POST /v1/calculate-tax"
```

Templates defined in `.marionetterc.json`:
```json
{
  "templates": [
    {"name": "billing-endpoint", "tags": {"type": "backend", "security": "admin-only"}}
  ]
}
```

### Agent Directives — communicate with AI agents

```python
# @agent-todo "Add rate limiting to this endpoint"
# @agent-skill "This handler collects PII. Get GDPR consent before storage."
# @agent-ignore begin
# ... legacy compatibility shim — do not modify ...
# @agent-ignore end
```

## Supported Value Types

| Type | Syntax | Example |
|------|--------|---------|
| String | `"quoted"` or `` `backtick` `` | `route: "/v1/users"` |
| Multiline | `"""..."""` or `` ```...``` `` | `description: """..."""` |
| Number | No quotes | `rate-limit: 100` |
| Boolean | `true` / `false` | `enabled: true` |
| Array | `["v1", "v2"]` | `roles: ["admin", "user"]` |

## Commands (18 total)

### `marionette init`

Creates a new `.marionette.db` SQLite index and default `.marionetterc.json`.

```
marionette --db demo.db init
```

### `marionette index [path...]`

Indexes source files. Supports multiple paths. Uses MD5 hashing for incremental
indexing. Loads additional repositories from `.marionetterc.json`.

```
marionette --db demo.db index ./src
marionette --db demo.db index ./backend ./frontend ./shared
```

### `marionette query [flags]`

Queries the tag index with powerful filters.

| Flag | Description |
|------|-------------|
| `--kind` | Filter by @tag-block kind (e.g., `api-endpoint`) |
| `--key` | Filter by metadata key |
| `--value` | Filter by metadata value |
| `--tag-type` | Filter by `block` or `inline` |
| `--language` | Filter by language |
| `--entity-kind` | Filter by entity type |
| `--file` | Filter by file path pattern |
| `--format` | Output: `json`, `markdown`, `ctags` |
| `--stats` | Show index metadata |
| `--stat` | Show aggregated tag statistics |
| `--depends-on` | Find tags referencing a value |

### `marionette tag {add|edit|rm}`

Manually manage tags in the index.

```
marionette tag add --file src/api.js --key security --value rbac --kind api-endpoint
marionette tag edit --id tag_abc123 --key priority --value critical
marionette tag rm --id tag_abc123
```

### `marionette diff <db1> <db2>`

Compare tags between two index databases — ideal for branch comparisons.

```
marionette diff main.db feature.db --key security
marionette diff before.db after.db --kind api-endpoint
```

Shows: `+ ADDED`, `- REMOVED`, `~ CHANGED` tags.

### `marionette migrate`

Bulk rename or delete tag keys across the entire index.

```
marionette migrate --rename "area/billing/tax-calculation" "tax/v2/calculation"
marionette migrate --delete-key "deprecated-flag"
```

### `marionette depends`

Find tags that depend on or reference a value.

```
marionette depends --on billing
marionette depends --on pii
```

### `marionette callers <symbol>`

Shows which functions call a given symbol, using the call graph from `call_edges` table.

```
marionette callers register_user
marionette callers promoteUser --show-tags
```

### `marionette callees <symbol>`

Shows which functions a symbol calls, using the call graph.

```
marionette callees processPayment
marionette callees processPayment --show-tags
```

### `marionette impact <symbol>`

Shows impact analysis (blast radius) for a symbol: all @tag annotations on it,
all references to it across the index.

```
marionette impact register_user
```

### `marionette search <query>`

Full-text search over tag metadata using LIKE queries. Supports ranked results.

```
marionette search "pii"
marionette search "billing" --limit 10 --format json
marionette search "gdpr" --file "*.py"
```

### `marionette analytics`

Tag analytics over time with snapshot recording.

```
marionette analytics --snapshot           # Record current state
marionette analytics                      # Show trend (last 30 snapshots)
marionette analytics --since 2026-01-01   # Filter by date
```

### `marionette health`

Marionette health dashboard — coverage, compliance, gaps with letter grade (A-F).

```
marionette health
marionette health --verbose
marionette health --threshold B    # CI gating: exit 1 if grade < B
```

### `marionette lint [path]`

Validates @tag annotations: orphan tags, syntax errors, type safety.

```
marionette lint ./src
```

Use `--strict` to enforce architectural constraints from `.marionetterc.json`:
tag, code, and architecture rules.

```
marionette lint --strict --db demo.db ./src
```

### `marionette lsp`

Starts a Language Server Protocol server (stdin/stdout, JSON-RPC).
Supports `textDocument/definition` and `textDocument/references`.

### `marionette serve`

Starts a Model Context Protocol server for AI agent integration.
Exposes @tag annotations as MCP tools.

```
marionette serve
```

MCP tools: `tagdoc_search`, `tagdoc_pii_scan`, `tagdoc_architecture`,
`tagdoc_compliance`, `tagdoc_feature_flags`, `tagdoc_depends`.

### `marionette watch [path]`

Watches files and auto-reindexes on change (polling-based, 2s interval).

```
marionette watch ./src
```

### `marionette stats`

Displays index statistics: total files, total tags, timestamps.

```
marionette stats
marionette stats --format json
```

## Configuration (.marionetterc.json)

```json
{
  "version": "2.0",
  "index": {
    "exclude": ["node_modules/**", "__pycache__/**", ".git/**"]
  },
  "repositories": [
    {"path": "../backend", "label": "Go API"},
    {"path": "../frontend", "label": "Node Frontend"}
  ],
  "templates": [
    {"name": "billing-endpoint", "tags": {"type": "backend", "security": "admin-only"}}
  ],
  "rules": [
    {"key": "security", "required": true, "allowed_values": ["public", "rbac", "admin-only"]},
    {"key": "gdpr-impact", "required": true}
  ],
  "output": {
    "format": "json"
  }
}
```

### Constraints (marionette lint --strict)

Architectural constraint rules enforce code quality and compliance at the CI level.
Three rule types are supported:

```json
{
  "constraints": [
    {"type": "tag", "name": "pii-needs-gdpr", "rule": "if-then:security=pii-access,gdpr-impact", "severity": "error"},
    {"type": "code", "name": "no-malloc", "rule": "no malloc, calloc, realloc in C code", "severity": "error"},
    {"type": "code", "name": "no-unsafe", "rule": "no unsafe", "severity": "error"},
    {"type": "arch", "name": "billing-boundary", "rule": "billing must not import reporting", "severity": "warning"}
  ]
}
```

| Type | What it checks | Example rule |
|------|---------------|-------------|
| `tag` | Required metadata keys and conditional values | `"require:gdpr-impact"`, `"if-then:security=pii-access,gdpr-impact"` |
| `code` | Forbidden AST nodes (function calls, keywords) | `"no malloc"`, `"no unsafe"`, `"no goto"` |
| `arch` | Module import boundaries | `"billing must not import reporting"` |

Run with: `marionette lint --strict --db demo.db`

## Output Schema (JSON)

```json
{
  "id": "tag_00000001abcd",
  "file": "server/routes/users.js",
  "language": "javascript",
  "tag_type": "block",
  "entity_name": "promoteUser",
  "entity_kind": "function_declaration",
  "range": {
    "start": { "line": 10, "column": 0 },
    "end": { "line": 17, "column": 1 }
  },
  "metadata": {
    "kind": "api-endpoint",
    "route": "/v1/users/:id/promote",
    "method": "POST",
    "layer": "controller",
    "security": "rbac",
    "roles": ["admin", "super-admin"]
  },
  "created_at": "2026-06-18T13:36:10Z",
  "updated_at": "2026-06-18T13:36:10Z"
}
```

## Project Architecture

```
marionette/
├── SPEC.md                      # Software Requirements Specification (SRS)
├── README.md
├── TUTORIAL.md                  # Hands-on guide
├── SKILL.md                     # AI agent skill file
├── LICENSE                      # PolyForm Noncommercial License 1.0.0
├── Makefile
├── go.mod, go.sum
├── .marionetterc.json           # Default project config
├── assets/                      # README banner image
├── queries/                     # SCM query files for reference
├── testdata/                    # Sample files for testing
│   ├── sample.js, sample.py, sample.rs
│   └── hierarchical_multiline.js
├── examples/                    # Real-world example projects
│   ├── constraints/             # Constraint-rule violation demo (C)
│   ├── microservices/           # Architecture mapping demo (Python)
│   ├── tag-templates/           # Tag-template expansion demo (Python)
│   └── todoapp/                 # GTD-style task manager (Python)
└── src/
    ├── cmd/marionette/main.go       # CLI entry point (18 commands)
    └── pkg/
        ├── models/              # Core data types
        ├── parser/              # Tree-sitter 31-language parser
        │   ├── parser.go        # Dynamic language registry
        │   └── helpers.go       # Comment traversal, entity extraction
        ├── extractor/           # @tag tokenizer (EBNF grammar)
        ├── indexer/             # Incremental indexing
        ├── callgraph/           # Symbol + call-edge extraction
        ├── storage/             # SQLite persistence + diff
        ├── query/               # Query engine
        ├── output/              # Formatters (JSON, MD, ctags)
        ├── config/              # .marionetterc.json configuration
        ├── constraint/          # tag/code/arch constraint engine
        ├── lint/                # Validation and linting
        └── lsp/                 # LSP + MCP server
```

## Data Flow

```
Source Files (31 languages)
    │
    ▼
┌──────────────┐     ┌──────────────────────┐
│ tree-sitter  │────→│ Comment groupings     │
│ (recursive   │     │ + NextNamedSibling()  │
│  comment     │     │ entity resolution     │
│  collection) │     └───────────┬───────────┘
└──────────────┘                 │
                                 ▼
                       ┌──────────────────┐
                       │ @tag Tokenizer    │
                       │ (EBNF + templates)│
                       └────────┬─────────┘
                                │
                                ▼
                       ┌──────────────────┐     ┌──────────────┐
                       │ Tag Models       │────→│ SQLite Store │
                       │ + MD5 Hash       │     │ (incremental)│
                       └──────────────────┘     └──────┬───────┘
                                                        │
              ┌──────────────────────┬──────────────────┤
              │                      │                  │
              ▼                      ▼                  ▼
       ┌────────────┐        ┌──────────────┐    ┌──────────────┐
       │ CLI Query  │        │ LSP Server   │    │ MCP Server   │
       │ + Format   │        │ (IDE)        │    │ (AI agents)  │
       │ + Diff     │        └──────────────┘    └──────────────┘
       │ + Migrate  │
       │ + Depends  │
       └────────────┘
```

## Use Cases

- **Architecture Mapping**: `@tag-block architecture` with `layer`, `domain`, `entrypoint`
  metadata. Query all entry points: `marionette query --kind architecture`
- **Compliance Auditing**: `@tag security:pii-access` and `@tag gdpr-impact: "high"`.
  Scan all PII access points: `marionette query --key security --value pii-access`
- **Feature Flag Tracking**: `@tag-block feature-flag` with `rollout`, `owner`, `status`.
  Find stale flags: `marionette query --key rollout --value 100%`
- **Agent Directives**: `@agent-todo`, `@agent-skill`, `@agent-ignore` for AI agents.
- **Cross-Language Discovery**: `marionette query --key type --value backend` finds tagged
  code across JS, Python, Rust, Go, Java, and all 31 languages in one query.
- **CI/CD Guardrails**: `marionette lint` fails the build if tags violate `.marionetterc.json` rules.
- **Branch Reviews**: `marionette diff main HEAD --key security` catches PII changes in PRs.

## Inspired By

Marionette draws inspiration from several pioneering projects in the code
intelligence space:

- **[tokensave](https://github.com/aovestdipaperino/tokensave)** — Semantic knowledge
  graph for AI agents with 80+ MCP tools and 50+ language support. Inspired
  Marionette's MCP server and call graph traversal features.
- **[wat](https://github.com/heimann/wat)** — A modern ctags replacement built on
  tree-sitter with interactive TUI, fuzzy finding, and file-level dependency
  analysis. Inspired Marionette's symbol extraction and call tree visualization.
- **[tree-sitter-analyzer](https://github.com/aimasteracc/tree-sitter-analyzer)** —
  Cross-language-safe code-intelligence MCP with architectural constraint
  enforcement, project health grading, and miswire-audit. Inspired Marionette's
  constraint engine and health dashboard.
- **[mmcg](https://github.com/xcrft/mastermind)** — Multi-language code indexer
  with spec verification gates and centrality analysis. Inspired Marionette's
  verification and rule enforcement design.
- **[universal-ctags](https://github.com/universal-ctags/ctags)** — The classic
  tag generator. Marionette's `--format ctags` output provides retro-compatibility.
- **[tree-sitter](https://tree-sitter.github.io/)** — The parsing foundation that
  makes Marionette's multi-language support possible.

**Marionette is unique** in its focus on structured `@tag` syntax inside comments,
typed value parsing with EBNF grammar, hierarchical taxonomies (`area/billing/tax`),
conditional constraint rules (`if-then`), tag templates (`@tag-use`), agent
directives (`@agent-todo`, `@agent-skill`, `@agent-ignore`), and direct AI agent
integration via both LSP and MCP protocols.

## License

PolyForm Noncommercial License 1.0.0 — see [LICENSE](LICENSE).

This project is licensed for **noncommercial use only**. Any commercial use
requires a separate license from the licensor.

Required Notice: Copyright (c) 2026 Xyzzy Apps
